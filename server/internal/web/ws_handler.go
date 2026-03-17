// Package web は Gin ベースの HTTP/WebSocket ハンドラを提供します。
// GET /ws で WebSocket アップグレードを行い、プロトコルのディスパッチを担当します。
package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/horitaku/stackchan/server/internal/audio"
	"github.com/horitaku/stackchan/server/internal/conversation"
	"github.com/horitaku/stackchan/server/internal/logging"
	"github.com/horitaku/stackchan/server/internal/protocol"
	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/session"
)

var upgrader = websocket.Upgrader{
	// TODO: 本番環境では Origin を適切に検証してください
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler は WebSocket 接続のライフサイクルを管理するハンドラです。
type WSHandler struct {
	manager         *session.Manager
	readTimeoutSec  int
	writeTimeoutSec int
	orchestrator    *conversation.Orchestrator
	runtimeState    *RuntimeState
	mu              sync.Mutex
	connections     map[string]*websocket.Conn
	activeSessionID string
}

const ttsChunkByteSize = 512

type audioChunkPayload struct {
	StreamID        string `json:"stream_id"`
	ChunkIndex      int    `json:"chunk_index"`
	Codec           string `json:"codec"`
	SampleRateHz    int    `json:"sample_rate_hz"`
	FrameDurationMs int    `json:"frame_duration_ms"`
	ChannelCount    int    `json:"channel_count"`
	DataBase64      string `json:"data_base64"`
}

type audioEndPayload struct {
	StreamID        string `json:"stream_id"`
	FinalChunkIndex int    `json:"final_chunk_index"`
	Reason          string `json:"reason,omitempty"`
}

// heartbeatPayload は firmware から定期送信される heartbeat の payload 定義です。
type heartbeatPayload struct {
	UptimeMs int64 `json:"uptime_ms"`
	RSSI     int   `json:"rssi,omitempty"`
}

// NewWSHandler は WSHandler を初期化して返します。
// readTimeoutSec / writeTimeoutSec に 0 を指定するとタイムアウトなしになります。
func NewWSHandler(manager *session.Manager, readTimeoutSec, writeTimeoutSec int, orchestrator *conversation.Orchestrator) *WSHandler {
	runtimeState := NewRuntimeState()
	return &WSHandler{
		manager:         manager,
		readTimeoutSec:  readTimeoutSec,
		writeTimeoutSec: writeTimeoutSec,
		orchestrator:    orchestrator,
		runtimeState:    runtimeState,
		connections:     make(map[string]*websocket.Conn),
	}
}

// RuntimeState は API ハンドラ連携用にランタイム状態ストアを返します。
func (h *WSHandler) RuntimeState() *RuntimeState {
	return h.runtimeState
}

// Handle は GET /ws の WebSocket アップグレードと受信ループを担当します。
func (h *WSHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logging.Logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// セッションを生成し、session_id 付きのロギングコンテキストを作成します
	s := h.manager.Create(context.Background())
	ctx := logging.WithSessionID(context.Background(), s.ID)
	log := logging.FromContext(ctx)

	log.Info().Msg("WebSocket connection established")
	h.runtimeState.OnConnected(s.ID)
	h.mu.Lock()
	h.connections[s.ID] = conn
	h.mu.Unlock()
	defer func() {
		h.manager.Delete(s.ID)
		h.mu.Lock()
		delete(h.connections, s.ID)
		if h.activeSessionID == s.ID {
			h.activeSessionID = ""
		}
		h.mu.Unlock()
		h.runtimeState.OnDisconnected()
		conn.Close()
		log.Info().Msg("WebSocket connection closed")
	}()

	h.readLoop(ctx, conn, s)
}

// readLoop はメッセージ受信ループです。接続が閉じられるまで継続します。
func (h *WSHandler) readLoop(ctx context.Context, conn *websocket.Conn, s *session.Session) {
	log := logging.FromContext(ctx)

	for {
		// 読み取りデッドライン設定（毎ループで更新することでアクティブ接続を維持します）
		if h.readTimeoutSec > 0 {
			conn.SetReadDeadline(time.Now().Add(time.Duration(h.readTimeoutSec) * time.Second))
		}

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn().Err(err).Msg("unexpected WebSocket close")
			}
			return
		}

		// バイナリメッセージは handleBinaryFrame で処理します
		if msgType == websocket.BinaryMessage {
			h.handleBinaryFrame(ctx, conn, s, data)
			continue
		}
		// v0 は JSON テキストメッセージのみ対応（バイナリは上記で分岐済み）
		if msgType != websocket.TextMessage {
			log.Warn().Int("msg_type", msgType).Msg("unsupported message type, skipping")
			continue
		}

		// dispatch が true を返した場合は接続を切断します
		if shouldClose := h.dispatch(ctx, conn, s, data); shouldClose {
			return
		}
	}
}

// dispatch は受信メッセージをパース・検証し、イベントハンドラへ振り分けます。
// 接続を切断すべき場合は true を返します。
func (h *WSHandler) dispatch(ctx context.Context, conn *websocket.Conn, s *session.Session, data []byte) bool {
	log := logging.FromContext(ctx)

	// エンベロープのパース
	env, err := protocol.ParseEnvelope(data)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse envelope")
		h.writeError(conn, s, protocol.ErrCodeInvalidMessage, err.Error(), false, nil, nil)
		return false
	}

	// エンベロープの検証
	if err := env.Validate(); err != nil {
		log.Error().Err(err).Str("type", env.Type).Msg("envelope validation failed")
		code := protocol.ErrCodeInvalidMessage
		if env.Version != "" && env.Version != protocol.SupportedVersion {
			code = protocol.ErrCodeUnsupportedVersion
		}
		h.writeError(conn, s, code, err.Error(), false, &env.Type, &env.Sequence)
		return false
	}

	// sequence 管理（重複・逆転の検知）
	result, _ := s.Sequence.CheckInbound(env.Sequence)
	switch result {
	case protocol.SequenceDuplicate:
		log.Warn().Int64("sequence", env.Sequence).Str("type", env.Type).Msg("duplicate sequence, skipping")
		return false
	case protocol.SequenceReversed:
		log.Warn().Int64("sequence", env.Sequence).Str("type", env.Type).Msg("sequence reversed, processing anyway")
	}

	// session.hello を受信する前に他のイベントが来た場合はエラーを返します
	if s.State == session.StateConnected && env.Type != "session.hello" {
		log.Warn().Str("type", env.Type).Msg("received event before session.hello")
		h.writeError(conn, s, protocol.ErrCodeSessionRequired, "session.hello must be sent first", false, &env.Type, &env.Sequence)
		return false
	}

	// イベントの振り分け
	switch env.Type {
	case "session.hello":
		res := session.HandleHello(ctx, s, env)
		if s.State == session.StateHandshaked {
			h.mu.Lock()
			h.activeSessionID = s.ID
			h.mu.Unlock()
		}
		if res.Response != nil {
			if err := conn.WriteMessage(websocket.TextMessage, res.Response); err != nil {
				log.Error().Err(err).Msg("failed to write session.hello response")
			}
		}
		if res.Fatal {
			return true
		}

	case "audio.stream_open":
		// バイナリフレームストリームのメタデータを登録します
		var payload protocol.BinaryStreamOpenPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "failed to parse audio.stream_open payload", false, &env.Type, &env.Sequence)
			return false
		}
		if payload.StreamID == "" || payload.Codec == "" || payload.SampleRateHz <= 0 || payload.FrameDurationMs <= 0 || payload.ChannelCount <= 0 {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "audio.stream_open payload is invalid", false, &env.Type, &env.Sequence)
			return false
		}
		if err := audio.ValidateCodec(payload.Codec); err != nil {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, err.Error(), false, &env.Type, &env.Sequence)
			return false
		}
		s.RegisterBinaryStream(payload.StreamID, session.BinaryStreamMeta{
			Codec:           payload.Codec,
			SampleRateHz:    payload.SampleRateHz,
			FrameDurationMs: payload.FrameDurationMs,
			ChannelCount:    payload.ChannelCount,
		})
		log.Info().Str("stream_id", payload.StreamID).Str("codec", payload.Codec).Int("sample_rate_hz", payload.SampleRateHz).Msg("binary stream registered")

	case "audio.chunk":
		var payload audioChunkPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "failed to parse audio.chunk payload", false, &env.Type, &env.Sequence)
			return false
		}
		if payload.StreamID == "" || payload.Codec == "" || payload.DataBase64 == "" || payload.SampleRateHz <= 0 || payload.FrameDurationMs <= 0 || payload.ChannelCount <= 0 {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "audio.chunk payload is invalid", false, &env.Type, &env.Sequence)
			return false
		}
		if err := s.AddAudioChunk(providers.AudioChunk{
			StreamID:        payload.StreamID,
			ChunkIndex:      payload.ChunkIndex,
			Codec:           payload.Codec,
			SampleRateHz:    payload.SampleRateHz,
			FrameDurationMs: payload.FrameDurationMs,
			ChannelCount:    payload.ChannelCount,
			DataBase64:      payload.DataBase64,
		}); err != nil {
			log.Error().Err(err).Str("stream_id", payload.StreamID).Msg("audio chunk buffer overflow")
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, err.Error(), false, &env.Type, &env.Sequence)
			return false
		}

	case "audio.end":
		var payload audioEndPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "failed to parse audio.end payload", false, &env.Type, &env.Sequence)
			return false
		}
		if payload.StreamID == "" {
			h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "audio.end stream_id is required", false, &env.Type, &env.Sequence)
			return false
		}
		if h.orchestrator != nil {
			streamMeta, _ := s.GetBinaryStreamMeta(payload.StreamID)
			chunks, firstChunkAt := s.ConsumeAudioStream(payload.StreamID)
			// 空ストリームチェック: チャンク受信前に audio.end が届いた場合はエラーです
			if len(chunks) == 0 {
				h.writeError(conn, s, protocol.ErrCodeInvalidPayload, "audio stream is empty: no chunks received before audio.end", false, &env.Type, &env.Sequence)
				return false
			}
			queueWaitMs := int64(0)
			if !firstChunkAt.IsZero() {
				queueWaitMs = time.Since(firstChunkAt).Milliseconds()
			}
			log.Info().
				Str("stream_id", payload.StreamID).
				Int("chunk_count", len(chunks)).
				Int64("queue_wait_ms", queueWaitMs).
				Msg("audio stream consumed, starting orchestration")

			// requestID = stream_id（フェーズ 4 では stream_id を request_id として扱います）
			requestID := payload.StreamID
			result, err := h.orchestrator.ProcessAudioStream(ctx, s.ID, requestID, payload.StreamID, chunks)
			if err != nil {
				code, message, retryable := providers.ToProtocolError(err)
				h.runtimeState.OnOutputError()
				h.writeError(conn, s, code, message, retryable, &env.Type, &env.Sequence)
				return false
			}

			firstFrameLatencyMs := int64(0)
			cadenceJitterMs := int64(0)
			e2eLatencyMs := int64(0)
			if streamMeta != nil {
				if !streamMeta.OpenedAt.IsZero() {
					e2eLatencyMs = time.Since(streamMeta.OpenedAt).Milliseconds()
				}
				if !streamMeta.FirstFrameAt.IsZero() && !streamMeta.OpenedAt.IsZero() {
					firstFrameLatencyMs = streamMeta.FirstFrameAt.Sub(streamMeta.OpenedAt).Milliseconds()
				}
				cadenceJitterMs = calculateCadenceJitterMs(chunks, streamMeta.FrameDurationMs)
				h.runtimeState.OnOpusMetrics(result.RequestID, payload.StreamID, firstFrameLatencyMs, cadenceJitterMs, e2eLatencyMs)
				log.Info().
					Str("stream_id", payload.StreamID).
					Str("request_id", result.RequestID).
					Int64("first_frame_latency_ms", firstFrameLatencyMs).
					Int64("cadence_jitter_ms", cadenceJitterMs).
					Int64("e2e_latency_ms", e2eLatencyMs).
					Msg("opus runtime metrics recorded")
			}
			h.runtimeState.OnPipeline(result.RequestID, payload.StreamID, queueWaitMs, result.STTLatencyMs, result.LLMLatencyMs, result.TTSLatencyMs, result.TotalLatencyMs)
			h.runtimeState.OnPlaybackQueued(result.RequestID, result.TotalLatencyMs, result.TTSDuration)

			// stt.final を送信します
			sttPayload := protocol.STTFinalPayload{
				RequestID:  result.RequestID,
				Transcript: result.Transcript,
			}
			if err := h.sendEvent(conn, s, "stt.final", sttPayload); err != nil {
				log.Error().Err(err).Msg("failed to send stt.final")
				h.runtimeState.OnOutputError()
				return true // 送信失敗は致命的（接続クローズ）
			}

			if err := h.sendTTSAudio(conn, s, result.RequestID, result.TTSAudioBase64, result.TTSDuration, result.TTSSampleHz, "pcm"); err != nil {
				log.Error().Err(err).Msg("failed to send tts.end")
				h.runtimeState.OnOutputError()
				return true
			}
			h.runtimeState.OnPlaybackSent()
			h.runtimeState.OnPlaybackCompleted()
		}

	case "heartbeat":
		// firmware キープアライブを受信します（接続監視 + ログ記録）
		var payload heartbeatPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			log.Warn().Err(err).Msg("failed to parse heartbeat payload")
			return false
		}
		log.Info().
			Int64("uptime_ms", payload.UptimeMs).
			Int("rssi", payload.RSSI).
			Msg("heartbeat received")
		h.runtimeState.OnHeartbeat()

	case "avatar.expression":
		var payload struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil && payload.Expression != "" {
			h.runtimeState.OnAvatarExpression(payload.Expression)
		}

	case "motion.play":
		var payload struct {
			Motion string `json:"motion"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil && payload.Motion != "" {
			h.runtimeState.OnAvatarMotion(payload.Motion)
		}

	default:
		log.Warn().Str("type", env.Type).Msg("unhandled event type")
	}

	return false
}

func (h *WSHandler) sendTTSAudio(conn *websocket.Conn, s *session.Session, requestID, audioBase64 string, durationMs, sampleRateHz int, codec string) error {
	audioBytes, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return fmt.Errorf("failed to decode tts audio base64 for chunking: %w", err)
	}

	totalChunks := 0
	if len(audioBytes) > 0 {
		totalChunks = (len(audioBytes) + ttsChunkByteSize - 1) / ttsChunkByteSize
	}

	for index := 0; index < totalChunks; index++ {
		start := index * ttsChunkByteSize
		end := start + ttsChunkByteSize
		if end > len(audioBytes) {
			end = len(audioBytes)
		}
		payload := protocol.TTSChunkPayload{
			RequestID:   requestID,
			ChunkIndex:  index,
			TotalChunks: totalChunks,
			AudioBase64: base64.StdEncoding.EncodeToString(audioBytes[start:end]),
		}
		if err := h.sendEvent(conn, s, "tts.chunk", payload); err != nil {
			return err
		}
	}

	ttsPayload := protocol.TTSEndPayload{
		RequestID:    requestID,
		DurationMs:   durationMs,
		SampleRateHz: sampleRateHz,
		Codec:        codec,
		TotalChunks:  totalChunks,
	}
	if err := h.sendEvent(conn, s, "tts.end", ttsPayload); err != nil {
		return err
	}
	return nil
}

// writeError は error メッセージを生成して WebSocket に送信します。
func (h *WSHandler) writeError(
	conn *websocket.Conn,
	s *session.Session,
	code, message string,
	retryable bool,
	reqType *string,
	reqSeq *int64,
) {
	payload := protocol.ErrorPayload{
		Code:        code,
		Message:     message,
		Retryable:   retryable,
		RequestType: reqType,
	}
	if reqSeq != nil {
		payload.RequestSequence = reqSeq
	}
	seq := s.Sequence.NextOutbound()
	data, err := protocol.NewErrorEnvelope(s.ID, seq, payload)
	if err != nil {
		logging.Logger.Error().Err(err).Msg("failed to build error envelope")
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logging.Logger.Error().Err(err).Msg("failed to write error envelope")
		h.runtimeState.OnOutputError()
	}
}

// BuildJSONMessage は任意の型を JSON エンベロープにシリアライズして返します。
// テストや他のハンドラから利用します。
func BuildJSONMessage(msgType, sessionID string, sequence int64, payload any) ([]byte, error) {
	env, err := protocol.NewEnvelope(msgType, sessionID, sequence, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(env)
}

// sendEvent は指定のイベントをオートインクリメントされた sequence 付きで WebSocket へ送信します。
func (h *WSHandler) sendEvent(conn *websocket.Conn, s *session.Session, msgType string, payload any) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	seq := s.Sequence.NextOutbound()
	data, err := BuildJSONMessage(msgType, s.ID, seq, payload)
	if err != nil {
		return err
	}
	if h.writeTimeoutSec > 0 {
		conn.SetWriteDeadline(time.Now().Add(time.Duration(h.writeTimeoutSec) * time.Second))
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// SendTTSTestToActive は現在アクティブな handshaked セッションへ tts.end を送信します。
// 連携テストで視認性を高めるため avatar.expression と motion.play も続けて送信します。
func (h *WSHandler) SendTTSTestToActive(requestID, audioBase64 string, durationMs, sampleRateHz int, codec, expression, motion string) (string, error) {
	h.mu.Lock()
	sessionID := h.activeSessionID
	conn := h.connections[sessionID]
	h.mu.Unlock()

	if sessionID == "" || conn == nil {
		return "", fmt.Errorf("no active Stackchan session is connected")
	}

	s := h.manager.Get(sessionID)
	if s == nil {
		return "", fmt.Errorf("active Stackchan session is no longer available")
	}

	if sampleRateHz <= 0 {
		sampleRateHz = 24000
	}
	if strings.TrimSpace(codec) == "" {
		codec = "pcm"
	}

	if err := h.sendTTSAudio(conn, s, requestID, audioBase64, durationMs, sampleRateHz, codec); err != nil {
		h.mu.Lock()
		if h.activeSessionID == sessionID {
			h.activeSessionID = ""
		}
		delete(h.connections, sessionID)
		h.mu.Unlock()
		return "", err
	}

	if expression = strings.TrimSpace(expression); expression != "" {
		exprPayload := protocol.AvatarExpressionPayload{
			RequestID:  requestID,
			Expression: expression,
			Intensity:  0.8,
		}
		if err := h.sendEvent(conn, s, "avatar.expression", exprPayload); err != nil {
			h.mu.Lock()
			if h.activeSessionID == sessionID {
				h.activeSessionID = ""
			}
			delete(h.connections, sessionID)
			h.mu.Unlock()
			return "", err
		}
	}

	if motion = strings.TrimSpace(motion); motion != "" {
		motionPayload := protocol.MotionPlayPayload{
			RequestID: requestID,
			Motion:    motion,
			Speed:     1.0,
		}
		if err := h.sendEvent(conn, s, "motion.play", motionPayload); err != nil {
			h.mu.Lock()
			if h.activeSessionID == sessionID {
				h.activeSessionID = ""
			}
			delete(h.connections, sessionID)
			h.mu.Unlock()
			return "", err
		}
	}

	return sessionID, nil
}

// handleBinaryFrame はバイナリ WebSocket フレームを Session.AddBinaryAudioFrame 経由で処理します。
//
// フレームフォーマット: 先頭 36 バイト = stream_id、残り = 音声データ。
//
// P8-05: 以下の検証ログを記録します:
//   - codec（stream メタから取得）
//   - payload_bytes（音声データ部分のバイト数）
//   - expected_bytes（PCM の場合のみ、サンプルレート・フレーム長から計算）
//   - frame_index（ストリーム内の通し番号）
//   - first_frame_latency_ms（audio.stream_open からの経過時間。P8-10 計測用）
func (h *WSHandler) handleBinaryFrame(ctx context.Context, conn *websocket.Conn, s *session.Session, data []byte) {
	log := logging.FromContext(ctx)

	const uuidLen = 36

	if len(data) < uuidLen {
		log.Warn().
			Int("frame_size", len(data)).
			Int("minimum", uuidLen).
			Msg("binary frame too short to contain stream_id")
		h.writeError(conn, s, protocol.ErrCodeInvalidPayload,
			fmt.Sprintf("binary frame too short: %d bytes", len(data)), false, nil, nil)
		return
	}

	streamID := strings.TrimSpace(string(data[:uuidLen]))
	payloadBytes := len(data) - uuidLen

	if meta, ok := s.GetBinaryStreamMeta(streamID); ok {
		expectedBytes := audio.ExpectedPCMFrameBytes(meta.Codec, meta.SampleRateHz, meta.FrameDurationMs, meta.ChannelCount)
		if ok, warning := audio.ValidateFramePayload(meta.Codec, payloadBytes, expectedBytes); !ok {
			log.Warn().
				Str("stream_id", streamID).
				Str("codec", meta.Codec).
				Int("payload_bytes", payloadBytes).
				Int("expected_bytes", expectedBytes).
				Str("warning", warning).
				Msg("binary frame: unexpected frame size")
		}

		if meta.NextChunkIndex == 0 && !meta.OpenedAt.IsZero() {
			firstFrameLatencyMs := time.Since(meta.OpenedAt).Milliseconds()
			log.Info().
				Str("stream_id", streamID).
				Str("codec", meta.Codec).
				Int("payload_bytes", payloadBytes).
				Int("expected_bytes", expectedBytes).
				Int("frame_index", meta.NextChunkIndex).
				Int64("first_frame_latency_ms", firstFrameLatencyMs).
				Msg("binary frame: first frame received")
		} else {
			log.Debug().
				Str("stream_id", streamID).
				Str("codec", meta.Codec).
				Int("payload_bytes", payloadBytes).
				Int("frame_index", meta.NextChunkIndex).
				Msg("binary frame received")
		}
	} else {
		log.Warn().
			Str("stream_id", streamID).
			Int("payload_bytes", payloadBytes).
			Msg("binary frame received before audio.stream_open")
	}

	if err := s.AddBinaryAudioFrame(data); err != nil {
		log.Warn().
			Err(err).
			Str("stream_id", streamID).
			Int("frame_size", len(data)).
			Int("payload_bytes", payloadBytes).
			Msg("failed to process binary frame")
		h.writeError(conn, s, protocol.ErrCodeInvalidPayload, err.Error(), false, nil, nil)
	}
}

func calculateCadenceJitterMs(chunks []providers.AudioChunk, expectedFrameDurationMs int) int64 {
	if expectedFrameDurationMs <= 0 || len(chunks) < 2 {
		return 0
	}

	expected := float64(expectedFrameDurationMs)
	var sumAbsDelta float64
	count := 0

	for i := 1; i < len(chunks); i++ {
		prevAt := chunks[i-1].ReceivedAt
		currAt := chunks[i].ReceivedAt
		if prevAt.IsZero() || currAt.IsZero() {
			continue
		}
		intervalMs := currAt.Sub(prevAt).Seconds() * 1000.0
		sumAbsDelta += math.Abs(intervalMs - expected)
		count++
	}

	if count == 0 {
		return 0
	}

	return int64(math.Round(sumAbsDelta / float64(count)))
}
