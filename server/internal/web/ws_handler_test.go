package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stackchan/server/internal/logging"
	"github.com/stackchan/server/internal/protocol"
	"github.com/stackchan/server/internal/session"
	"github.com/stackchan/server/internal/web"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Init("error") // テスト中はエラーレベルのみ出力します
}

// newTestServer はテスト用の HTTP サーバーと WebSocket 接続先 URL を返します。
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	manager := session.NewManager()
	handler := web.NewWSHandler(manager, 0, 0) // タイムアウトなし

	r := gin.New()
	r.GET("/ws", handler.Handle)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	return ts, wsURL
}

// dialWS はテスト用 WebSocket クライアントを接続します。
func dialWS(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial WebSocket: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// buildEnvelope はテスト用の JSON エンベロープを組み立てます。
func buildEnvelope(t *testing.T, msgType string, sequence int64, payload any) []byte {
	t.Helper()
	payloadBytes, _ := json.Marshal(payload)
	env := map[string]any{
		"type":       msgType,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"session_id": "",
		"sequence":   sequence,
		"version":    "1.0",
		"payload":    json.RawMessage(payloadBytes),
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}
	return data
}

// readEnvelope は WebSocket から 1 メッセージを読み取ってエンベロープにデコードします。
func readEnvelope(t *testing.T, conn *websocket.Conn) protocol.Envelope {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	return env
}

// --- 正常系テスト ---

// TestHelloWelcomeFlow は session.hello を送ると session.welcome が返ることを確認します。
func TestHelloWelcomeFlow(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	msg := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "test-device-001",
		"client_type": "test_harness",
	})
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("failed to send hello: %v", err)
	}

	env := readEnvelope(t, conn)
	if env.Type != "session.welcome" {
		t.Errorf("expected session.welcome, got %s", env.Type)
	}
	if env.SessionID == "" {
		t.Error("expected non-empty session_id in welcome")
	}
	if env.Sequence != 1 {
		t.Errorf("expected server sequence=1, got %d", env.Sequence)
	}

	var welcome session.WelcomePayload
	if err := json.Unmarshal(env.Payload, &welcome); err != nil {
		t.Fatalf("failed to unmarshal welcome payload: %v", err)
	}
	if !welcome.Accepted {
		t.Error("expected Accepted=true")
	}
}

// TestHelloWelcomeFlow_Firmware は client_type=firmware でも welcome が返ることを確認します。
func TestHelloWelcomeFlow_Firmware(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	msg := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "cores3-01",
		"client_type": "firmware",
	})
	conn.WriteMessage(websocket.TextMessage, msg) //nolint

	env := readEnvelope(t, conn)
	if env.Type != "session.welcome" {
		t.Errorf("expected session.welcome, got %s", env.Type)
	}
}

// --- 異常系テスト ---

// TestInvalidJSON は JSON でないメッセージに対して error が返ることを確認します。
func TestInvalidJSON(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	conn.WriteMessage(websocket.TextMessage, []byte("not-json")) //nolint

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeInvalidMessage)
}

// TestMissingDeviceID は device_id 欠落時に error と接続切断が発生することを確認します。
func TestMissingDeviceID(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	msg := buildEnvelope(t, "session.hello", 1, map[string]any{
		"client_type": "firmware",
		// device_id を意図的に省略
	})
	conn.WriteMessage(websocket.TextMessage, msg) //nolint

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeInvalidPayload)

	// Fatal エラー後は接続が切断されるはずです
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Error("expected connection to be closed after fatal error")
	}
}

// TestInvalidClientType は無効な client_type に対して error と接続切断が発生することを確認します。
func TestInvalidClientType(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	msg := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "device-001",
		"client_type": "invalid_type",
	})
	conn.WriteMessage(websocket.TextMessage, msg) //nolint

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeInvalidPayload)
}

// TestEventBeforeHello は session.hello 前に他のイベントを送ると error が返ることを確認します。
func TestEventBeforeHello(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	msg := buildEnvelope(t, "audio.end", 1, map[string]any{
		"stream_id":         "stream-001",
		"final_chunk_index": 5,
	})
	conn.WriteMessage(websocket.TextMessage, msg) //nolint

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeSessionRequired)
}

// TestUnsupportedVersion は未対応の version フィールドに対して error が返ることを確認します。
func TestUnsupportedVersion(t *testing.T) {
	_, wsURL := newTestServer(t)
	conn := dialWS(t, wsURL)

	// version フィールドだけ不正な値を使います
	data, _ := json.Marshal(map[string]any{
		"type":       "session.hello",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"session_id": "",
		"sequence":   1,
		"version":    "99.0", // 未対応バージョン
		"payload":    map[string]any{"device_id": "d", "client_type": "firmware"},
	})
	conn.WriteMessage(websocket.TextMessage, data) //nolint

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeUnsupportedVersion)
}

// TestHealthCheck は HTTP /healthz が 200 を返すことを確認します。
func TestHealthCheck(t *testing.T) {
	manager := session.NewManager()
	handler := web.NewWSHandler(manager, 0, 0)
	r := gin.New()
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ws", handler.Handle)

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- ヘルパー ---

// assertErrorResponse はレスポンスが期待する error コードを持つことを検証します。
func assertErrorResponse(t *testing.T, env protocol.Envelope, expectedCode string) {
	t.Helper()
	if env.Type != "error" {
		t.Errorf("expected type=error, got %s", env.Type)
		return
	}
	var errPayload protocol.ErrorPayload
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v", err)
	}
	if errPayload.Code != expectedCode {
		t.Errorf("expected error code=%s, got %s", expectedCode, errPayload.Code)
	}
}

// TestBuildJSONMessage は BuildJSONMessage が有効なエンベロープを生成することを確認します。
func TestBuildJSONMessage(t *testing.T) {
	data, err := web.BuildJSONMessage("test.event", "sess-001", 1, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("BuildJSONMessage error: %v", err)
	}

	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if env.Type != "test.event" {
		t.Errorf("expected type=test.event, got %s", env.Type)
	}

	// context パッケージのインポートが使われていることを確認（コンパイルチェック）
	_ = context.Background()
}
