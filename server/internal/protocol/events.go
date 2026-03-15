// Package protocol - フェーズ 4 で追加するイベントのペイロード型定義です。
// events.md § 3.6 (stt.final)、§ 3.7 (tts.end)、§ 3.8 (audio.stream_open) に対応します。
package protocol

// STTFinalPayload は stt.final イベントのペイロードです（server -> firmware）。
// STT 処理が完了したことを通知し、認識テキストを格納します。
type STTFinalPayload struct {
	RequestID  string  `json:"request_id"`
	Transcript string  `json:"transcript"`
	Confidence float64 `json:"confidence,omitempty"`
}

// TTSChunkPayload は tts.chunk イベントのペイロードです（server -> firmware）。
// TTS 音声本体を小さなチャンクへ分割して配信します。
type TTSChunkPayload struct {
	RequestID   string `json:"request_id"`
	ChunkIndex  int    `json:"chunk_index"`
	TotalChunks int    `json:"total_chunks"`
	AudioBase64 string `json:"audio_base64"`
}

// TTSEndPayload は tts.end イベントのペイロードです（server -> firmware）。
// TTS 合成が完了した再生メタデータを格納します。
// audio_base64 は後方互換 fallback 用にのみ使用します。
type TTSEndPayload struct {
	RequestID    string `json:"request_id"`
	AudioBase64  string `json:"audio_base64,omitempty"`
	DurationMs   int    `json:"duration_ms"`
	SampleRateHz int    `json:"sample_rate_hz"`
	Codec        string `json:"codec"`
	TotalChunks  int    `json:"total_chunks,omitempty"`
}

// BinaryStreamOpenPayload は audio.stream_open イベントのペイロードです（firmware -> server）。
// バイナリフレーム送信を開始する前に、ストリームのコーデック・フォーマット情報を通知します。
type BinaryStreamOpenPayload struct {
	StreamID        string `json:"stream_id"`
	Codec           string `json:"codec"`
	SampleRateHz    int    `json:"sample_rate_hz"`
	FrameDurationMs int    `json:"frame_duration_ms"`
	ChannelCount    int    `json:"channel_count"`
}

// AvatarExpressionPayload は avatar.expression イベントのペイロードです（server -> firmware）。
type AvatarExpressionPayload struct {
	RequestID  string  `json:"request_id"`
	Expression string  `json:"expression"`
	Intensity  float64 `json:"intensity,omitempty"`
}

// MotionPlayPayload は motion.play イベントのペイロードです（server -> firmware）。
type MotionPlayPayload struct {
	RequestID string  `json:"request_id"`
	Motion    string  `json:"motion"`
	Speed     float64 `json:"speed,omitempty"`
}
