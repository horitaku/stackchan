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

// TTSEndPayload は tts.end イベントのペイロードです（server -> firmware）。
// TTS 合成が完了した音声データと再生メタデータを格納します。
type TTSEndPayload struct {
	RequestID    string `json:"request_id"`
	AudioBase64  string `json:"audio_base64"`
	DurationMs   int    `json:"duration_ms"`
	SampleRateHz int    `json:"sample_rate_hz"`
	Codec        string `json:"codec"`
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
