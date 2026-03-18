// Package protocol - フェーズ 4 で追加するイベントのペイロード型定義です。
// events.md § 3.6 (stt.final)、§ 3.7 (tts.end)、§ 3.8 (audio.stream_open) に対応します。
package protocol

// STTFinalPayload は stt.final イベントのペイロードです（server -> firmware）。
// STT 処理が完了したことを通知し、認識テキストを格納します。
type STTFinalPayload struct {
	RequestID    string               `json:"request_id"`
	Transcript   string               `json:"transcript"`
	Confidence   float64              `json:"confidence,omitempty"`
	Alternatives []STTAlternativeText `json:"alternatives,omitempty"`
	ContextHint  string               `json:"context_hint,omitempty"`
}

// STTAlternativeText は stt.final の代替候補です。
type STTAlternativeText struct {
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

// TTSChunkPayloadV11 は tts.chunk v1.1 のペイロードです（server -> firmware）。
// フレーム単位の再生に必要なメタデータを含みます。
type TTSChunkPayloadV11 struct {
	RequestID       string `json:"request_id"`
	StreamID        string `json:"stream_id"`
	ChunkIndex      int    `json:"chunk_index"`
	FrameDurationMs int    `json:"frame_duration_ms"`
	SamplesPerChunk int    `json:"samples_per_chunk"`
	Codec           string `json:"codec,omitempty"`
	SentAt          string `json:"sent_at,omitempty"`
	PlayoutTS       string `json:"playout_ts,omitempty"`
	AudioBase64     string `json:"audio_base64"`
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

// TTSBufferWatermarkPayload は tts.buffer.watermark イベントのペイロードです（firmware -> server）。
// P8-19: TTS 再生バッファの watermark 状態変化を server へ通知します。
// status が変化した時点のみ送信し、同一状態での再送は 500ms 以上間隔を空けます。
type TTSBufferWatermarkPayload struct {
	RequestID     string `json:"request_id"`
	StreamID      string `json:"stream_id"`
	Status        string `json:"status"`          // "normal" | "low_water" | "high_water"
	BufferedMs    int    `json:"buffered_ms"`     // 現在のバッファ深さ（ms）
	ThresholdMs   int    `json:"threshold_ms"`    // 発火した watermark 閾値（ms）
	FramesInQueue int    `json:"frames_in_queue"` // キュー内フレーム数
}
