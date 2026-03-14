// Package web は Gin ベースの HTTP/WebSocket ハンドラを提供します。
// GET /ws で WebSocket アップグレードを行い、プロトコルのディスパッチを担当します。
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stackchan/server/internal/conversation"
	"github.com/stackchan/server/internal/logging"
	"github.com/stackchan/server/internal/protocol"
	"github.com/stackchan/server/internal/providers"
	"github.com/stackchan/server/internal/session"
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
}

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

// NewWSHandler は WSHandler を初期化して返します。
// readTimeoutSec / writeTimeoutSec に 0 を指定するとタイムアウトなしになります。
func NewWSHandler(manager *session.Manager, readTimeoutSec, writeTimeoutSec int, orchestrator *conversation.Orchestrator) *WSHandler {
	return &WSHandler{
		manager:         manager,
		readTimeoutSec:  readTimeoutSec,
		writeTimeoutSec: writeTimeoutSec,
		orchestrator:    orchestrator,
	}
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
	defer func() {
		h.manager.Delete(s.ID)
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

		// v0 は JSON テキストメッセージのみ対応（バイナリは将来拡張）
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
		if res.Response != nil {
			if err := conn.WriteMessage(websocket.TextMessage, res.Response); err != nil {
				log.Error().Err(err).Msg("failed to write session.hello response")
			}
		}
		if res.Fatal {
			return true
		}

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
		s.AddAudioChunk(providers.AudioChunk{
			StreamID:        payload.StreamID,
			ChunkIndex:      payload.ChunkIndex,
			Codec:           payload.Codec,
			SampleRateHz:    payload.SampleRateHz,
			FrameDurationMs: payload.FrameDurationMs,
			ChannelCount:    payload.ChannelCount,
			DataBase64:      payload.DataBase64,
		})

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
			chunks := s.ConsumeAudioStream(payload.StreamID)
			if _, err := h.orchestrator.ProcessAudioStream(ctx, s.ID, payload.StreamID, chunks); err != nil {
				code, message, retryable := providers.ToProtocolError(err)
				h.writeError(conn, s, code, message, retryable, &env.Type, &env.Sequence)
				return false
			}
		}

	default:
		log.Warn().Str("type", env.Type).Msg("unhandled event type")
	}

	return false
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
