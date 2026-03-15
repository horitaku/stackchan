package session

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/horitaku/stackchan/server/internal/providers"
)

// MaxAudioChunksPerStream はストリームあたりの最大バッファチャンク数です。
// 超過した場合はエラーを返し、バッファをクリアします。
const MaxAudioChunksPerStream = 500

// BinaryStreamMeta はバイナリフレームストリームのメタデータです。
// audio.stream_open イベントで登録され、バイナリフレーム受信時に参照します。
type BinaryStreamMeta struct {
	Codec           string
	SampleRateHz    int
	FrameDurationMs int
	ChannelCount    int
	NextChunkIndex  int // バイナリフレームの自動採番カウンター
}

// AddAudioChunk は stream_id ごとに受信チャンクを蓄積します。
// 最初のチャンク受信時刻を記録し（レイテンシ計測に使用）、バッファ上限超過時はエラーを返しバッファをクリアします。
func (s *Session) AddAudioChunk(chunk providers.AudioChunk) error {
	if s.AudioStreams == nil {
		s.AudioStreams = make(map[string][]providers.AudioChunk)
	}
	if s.AudioStreamFirstChunkAt == nil {
		s.AudioStreamFirstChunkAt = make(map[string]time.Time)
	}

	// 最初のチャンク受信時刻を記録します
	if _, exists := s.AudioStreamFirstChunkAt[chunk.StreamID]; !exists {
		s.AudioStreamFirstChunkAt[chunk.StreamID] = time.Now()
	}

	// バッファ上限チェック
	if len(s.AudioStreams[chunk.StreamID]) >= MaxAudioChunksPerStream {
		delete(s.AudioStreams, chunk.StreamID)
		delete(s.AudioStreamFirstChunkAt, chunk.StreamID)
		return fmt.Errorf("audio stream buffer overflow for stream_id=%s (limit=%d)", chunk.StreamID, MaxAudioChunksPerStream)
	}

	s.AudioStreams[chunk.StreamID] = append(s.AudioStreams[chunk.StreamID], chunk)
	return nil
}

// ConsumeAudioStream は stream_id のチャンクを取り出して内部バッファから削除します。
// 最初のチャンク受信時刻も返します（キュー待機時間の計算に使用）。
func (s *Session) ConsumeAudioStream(streamID string) ([]providers.AudioChunk, time.Time) {
	if s.AudioStreams == nil {
		return nil, time.Time{}
	}
	chunks := s.AudioStreams[streamID]
	firstAt := s.AudioStreamFirstChunkAt[streamID]
	delete(s.AudioStreams, streamID)
	delete(s.AudioStreamFirstChunkAt, streamID)
	if s.BinaryStreams != nil {
		delete(s.BinaryStreams, streamID)
	}
	return chunks, firstAt
}

// RegisterBinaryStream は audio.stream_open で通知されたバイナリストリームのメタ情報を登録します。
func (s *Session) RegisterBinaryStream(streamID string, meta BinaryStreamMeta) {
	if s.BinaryStreams == nil {
		s.BinaryStreams = make(map[string]*BinaryStreamMeta)
	}
	m := meta
	s.BinaryStreams[streamID] = &m
}

// GetBinaryStreamMeta は登録済みバイナリストリームのメタ情報を返します。
func (s *Session) GetBinaryStreamMeta(streamID string) (*BinaryStreamMeta, bool) {
	if s.BinaryStreams == nil {
		return nil, false
	}
	m, ok := s.BinaryStreams[streamID]
	return m, ok
}

// AddBinaryAudioFrame はバイナリ WebSocket フレームを AudioChunk に変換してバッファに追加します。
// フレームフォーマット: 先頭 36 バイト = stream_id（UUID 文字列）、残りバイト = 音声データ。
// audio.stream_open でストリームメタを事前登録しておくことが必要です。
func (s *Session) AddBinaryAudioFrame(frame []byte) error {
	const uuidLen = 36
	if len(frame) < uuidLen {
		return fmt.Errorf("binary frame too short: %d bytes (minimum %d for stream_id)", len(frame), uuidLen)
	}
	streamID := strings.TrimSpace(string(frame[:uuidLen]))
	audioData := frame[uuidLen:]

	meta, ok := s.GetBinaryStreamMeta(streamID)
	if !ok {
		return fmt.Errorf("no metadata for binary stream_id=%q: send audio.stream_open first", streamID)
	}

	chunkIndex := meta.NextChunkIndex
	meta.NextChunkIndex++

	return s.AddAudioChunk(providers.AudioChunk{
		StreamID:        streamID,
		ChunkIndex:      chunkIndex,
		Codec:           meta.Codec,
		SampleRateHz:    meta.SampleRateHz,
		FrameDurationMs: meta.FrameDurationMs,
		ChannelCount:    meta.ChannelCount,
		DataBase64:      base64.StdEncoding.EncodeToString(audioData),
	})
}
