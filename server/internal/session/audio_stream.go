package session

import "github.com/stackchan/server/internal/providers"

// AddAudioChunk は stream_id ごとに受信チャンクを蓄積します。
func (s *Session) AddAudioChunk(chunk providers.AudioChunk) {
	if s.AudioStreams == nil {
		s.AudioStreams = make(map[string][]providers.AudioChunk)
	}
	s.AudioStreams[chunk.StreamID] = append(s.AudioStreams[chunk.StreamID], chunk)
}

// ConsumeAudioStream は stream_id のチャンクを取り出して内部バッファから削除します。
func (s *Session) ConsumeAudioStream(streamID string) []providers.AudioChunk {
	if s.AudioStreams == nil {
		return nil
	}
	chunks := s.AudioStreams[streamID]
	delete(s.AudioStreams, streamID)
	return chunks
}
