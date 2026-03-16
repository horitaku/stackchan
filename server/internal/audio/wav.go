// wav.go は 16-bit PCM フレーム列から WAV（RIFF）ファイルを構築します。
//
// 用途: OpenAI Whisper STT API など WAV ファイルを受け取るサービスへ
// バイナリストリームから受信した raw PCM フレームを渡す際に使用します。
//
// 対応フォーマット: PCM 16-bit signed little-endian (WAV fmt chunk type 1)
package audio

import (
	"bytes"
	"encoding/binary"
)

// BuildWAV は 16-bit PCM サンプル列から WAV（RIFF）バイト列を構築します。
//
// 引数:
//   - pcmData: 連結された 16-bit PCM サンプルのバイト列（little-endian）
//   - sampleRateHz: サンプルレート（例: 16000）
//   - channelCount: チャネル数（1=モノ, 2=ステレオ）
//
// 戻り値: WAV 形式のバイト列（.wav ファイルとして扱えます）
func BuildWAV(pcmData []byte, sampleRateHz, channelCount int) []byte {
	const bitsPerSample = 16
	byteRate := sampleRateHz * channelCount * bitsPerSample / 8
	blockAlign := channelCount * bitsPerSample / 8
	dataLen := len(pcmData)

	buf := &bytes.Buffer{}

	// ── RIFF チャンクヘッダ ───────────────────────────────────────────────
	buf.WriteString("RIFF")
	// ファイルサイズ - 8 bytes（RIFF ヘッダ自身を除く）
	// 内訳: 4(WAVE) + 8(fmt ヘッダ) + 16(fmt データ) + 8(data ヘッダ) + len(pcmData)
	binary.Write(buf, binary.LittleEndian, uint32(36+dataLen)) //nolint:errcheck
	buf.WriteString("WAVE")

	// ── fmt サブチャンク ─────────────────────────────────────────────────
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))            // チャンクサイズ（PCM は 16 bytes）//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(1))             // AudioFormat: 1 = PCM//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(channelCount))  // チャネル数//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint32(sampleRateHz))  // サンプルレート//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))      // バイトレート = SampleRate × NumChannels × BitsPerSample/8//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))    // ブロックアライン = NumChannels × BitsPerSample/8//nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample)) // ビット深度//nolint:errcheck

	// ── data サブチャンク ────────────────────────────────────────────────
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataLen)) //nolint:errcheck
	buf.Write(pcmData)

	return buf.Bytes()
}
