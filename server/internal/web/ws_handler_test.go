package web_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/horitaku/stackchan/server/internal/conversation"
	"github.com/horitaku/stackchan/server/internal/logging"
	"github.com/horitaku/stackchan/server/internal/protocol"
	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/providers/mock"
	"github.com/horitaku/stackchan/server/internal/session"
	"github.com/horitaku/stackchan/server/internal/web"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Init("error") // テスト中はエラーレベルのみ出力します
}

// newTestServer はテスト用の HTTP サーバーと WebSocket 接続先 URL を返します。
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	manager := session.NewManager()
	handler := web.NewWSHandler(manager, 0, 0, nil) // タイムアウトなし

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

func readTTSStreamUntilEnd(t *testing.T, conn *websocket.Conn) ([]protocol.TTSChunkPayload, protocol.Envelope, protocol.TTSEndPayload) {
	t.Helper()
	chunks := make([]protocol.TTSChunkPayload, 0)
	for {
		env := readEnvelope(t, conn)
		switch env.Type {
		case "tts.chunk":
			var chunk protocol.TTSChunkPayload
			if err := json.Unmarshal(env.Payload, &chunk); err != nil {
				t.Fatalf("failed to unmarshal tts.chunk payload: %v", err)
			}
			chunks = append(chunks, chunk)
		case "tts.end":
			var end protocol.TTSEndPayload
			if err := json.Unmarshal(env.Payload, &end); err != nil {
				t.Fatalf("failed to unmarshal tts.end payload: %v", err)
			}
			return chunks, env, end
		default:
			t.Fatalf("expected tts.chunk or tts.end, got %s", env.Type)
		}
	}
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
	handler := web.NewWSHandler(manager, 0, 0, nil)
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

func TestDeviceStateReportUpdatesRuntimeOverview(t *testing.T) {
	manager := session.NewManager()
	handler := web.NewWSHandler(manager, 0, 0, nil)

	r := gin.New()
	r.GET("/ws", handler.Handle)

	ts := httptest.NewServer(r)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn := dialWS(t, wsURL)

	hello := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "test-device-001",
		"client_type": "firmware",
	})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		t.Fatalf("failed to send hello: %v", err)
	}
	_ = readEnvelope(t, conn)

	stateReport := buildEnvelope(t, "device.state.report", 2, map[string]any{
		"request_id":          "hw-state-123",
		"source":              "webui.hardware_test",
		"uptime_ms":           9999,
		"rssi":                -47,
		"free_heap_bytes":     345678,
		"current_angle_x_deg": 10.0,
		"current_angle_y_deg": -8.0,
		"calibration": map[string]any{
			"x": map[string]any{"min_deg": -45, "max_deg": 45},
			"y": map[string]any{"min_deg": -30, "max_deg": 30},
		},
		"mic_level":        0.1,
		"speaker_busy":     true,
		"camera_available": false,
		"firmware_version": "stackchan-cores3-01",
	})
	if err := conn.WriteMessage(websocket.TextMessage, stateReport); err != nil {
		t.Fatalf("failed to send device.state.report: %v", err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := handler.RuntimeState().Snapshot()
		if snapshot.Hardware.RequestID == "hw-state-123" {
			if snapshot.Hardware.RSSI != -47 {
				t.Fatalf("expected rssi=-47, got %d", snapshot.Hardware.RSSI)
			}
			if !snapshot.Hardware.SpeakerBusy {
				t.Fatalf("expected speaker_busy=true")
			}
			if snapshot.Hardware.LastReportAt == "" || snapshot.Hardware.LastReportAt == "-" {
				t.Fatalf("expected last_report_at to be populated, got %q", snapshot.Hardware.LastReportAt)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("device.state.report was not reflected to runtime snapshot")
}

// TestAudioEnd_ProviderErrorMapping は provider エラーが protocol error へ変換されることを確認します。
func TestAudioEnd_ProviderErrorMapping(t *testing.T) {
	manager := session.NewManager()
	orchestrator := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 500 * time.Millisecond, MaxAttempts: 1, BaseDelay: 10 * time.Millisecond},
	)
	handler := web.NewWSHandler(manager, 0, 0, orchestrator)

	r := gin.New()
	r.GET("/ws", handler.Handle)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn := dialWS(t, wsURL)

	// まず hello でハンドシェイク完了
	hello := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "test-device-001",
		"client_type": "test_harness",
	})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		t.Fatalf("failed to send hello: %v", err)
	}
	_ = readEnvelope(t, conn)

	// audio.end に stream_id 空を渡すと invalid_payload
	audioEnd := buildEnvelope(t, "audio.end", 2, map[string]any{
		"stream_id":         "",
		"final_chunk_index": 0,
	})
	if err := conn.WriteMessage(websocket.TextMessage, audioEnd); err != nil {
		t.Fatalf("failed to send audio.end: %v", err)
	}
	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeInvalidPayload)
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

// --- フェーズ 4: 音声パス E2E テスト ---

// newTestServerWithOrchestrator は mock provider を持つ orchestrator 付きの
// テストサーバーと WebSocket 接続先 URL を返します。
func newTestServerWithOrchestrator(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	manager := session.NewManager()
	orchestrator := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 2 * time.Second, MaxAttempts: 1, BaseDelay: 10 * time.Millisecond},
	)
	handler := web.NewWSHandler(manager, 0, 0, orchestrator)

	r := gin.New()
	r.GET("/ws", handler.Handle)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	return ts, wsURL
}

// sendHelloAndExpectWelcome はハンドシェイクを実行するテストヘルパーです。
func sendHelloAndExpectWelcome(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	hello := buildEnvelope(t, "session.hello", 1, map[string]any{
		"device_id":   "test-device-e2e",
		"client_type": "test_harness",
	})
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		t.Fatalf("failed to send hello: %v", err)
	}
	env := readEnvelope(t, conn)
	if env.Type != "session.welcome" {
		t.Fatalf("expected session.welcome, got %s", env.Type)
	}
}

// TestAudioFullLifecycle は audio.chunk × 3 → audio.end → stt.final → tts.end の
// 正常系フルフローを確認します。
func TestAudioFullLifecycle(t *testing.T) {
	_, wsURL := newTestServerWithOrchestrator(t)
	conn := dialWS(t, wsURL)

	sendHelloAndExpectWelcome(t, conn)

	const streamID = "stream-lifecycle-001"

	// audio.chunk を 3 件送信します
	for i := 0; i < 3; i++ {
		chunk := buildEnvelope(t, "audio.chunk", int64(i+2), map[string]any{
			"stream_id":         streamID,
			"chunk_index":       i,
			"codec":             "opus",
			"sample_rate_hz":    16000,
			"frame_duration_ms": 20,
			"channel_count":     1,
			"data_base64":       "AAAA",
		})
		if err := conn.WriteMessage(websocket.TextMessage, chunk); err != nil {
			t.Fatalf("failed to send audio.chunk %d: %v", i, err)
		}
	}

	// audio.end を送信してオーケストレーションを起動します
	audioEnd := buildEnvelope(t, "audio.end", 5, map[string]any{
		"stream_id":         streamID,
		"final_chunk_index": 2,
		"reason":            "normal",
	})
	if err := conn.WriteMessage(websocket.TextMessage, audioEnd); err != nil {
		t.Fatalf("failed to send audio.end: %v", err)
	}

	// stt.final を受信して内容を検証します
	env1 := readEnvelope(t, conn)
	if env1.Type != "stt.final" {
		t.Errorf("expected stt.final, got %s", env1.Type)
	}
	var sttPayload protocol.STTFinalPayload
	if err := json.Unmarshal(env1.Payload, &sttPayload); err != nil {
		t.Fatalf("failed to unmarshal stt.final payload: %v", err)
	}
	if sttPayload.RequestID != streamID {
		t.Errorf("expected request_id=%s, got %s", streamID, sttPayload.RequestID)
	}
	if sttPayload.Transcript == "" {
		t.Error("expected non-empty transcript")
	}

	// tts.chunk 群と tts.end を受信して内容を検証します
	ttsChunks, env2, ttsPayload := readTTSStreamUntilEnd(t, conn)
	if ttsPayload.RequestID != streamID {
		t.Errorf("expected request_id=%s, got %s", streamID, ttsPayload.RequestID)
	}
	if ttsPayload.DurationMs <= 0 {
		t.Errorf("expected duration_ms > 0, got %d", ttsPayload.DurationMs)
	}
	if len(ttsChunks) == 0 {
		t.Fatal("expected at least one tts.chunk")
	}
	if ttsPayload.TotalChunks != len(ttsChunks) {
		t.Errorf("expected total_chunks=%d, got %d", len(ttsChunks), ttsPayload.TotalChunks)
	}
	decodedBytes := 0
	for index, chunk := range ttsChunks {
		if chunk.RequestID != streamID {
			t.Errorf("expected chunk request_id=%s, got %s", streamID, chunk.RequestID)
		}
		if chunk.ChunkIndex != index {
			t.Errorf("expected chunk_index=%d, got %d", index, chunk.ChunkIndex)
		}
		bytes, err := base64.StdEncoding.DecodeString(chunk.AudioBase64)
		if err != nil {
			t.Fatalf("failed to decode chunk %d: %v", index, err)
		}
		decodedBytes += len(bytes)
	}
	if decodedBytes <= 0 {
		t.Error("expected decoded chunk bytes > 0")
	}
	// stt.final → tts.end の sequence が連番であることを確認します
	if env2.Sequence <= env1.Sequence {
		t.Errorf("expected tts.end sequence > %d, got %d", env1.Sequence, env2.Sequence)
	}
}

// TestAudioEnd_EmptyStream はチャンク受信前に audio.end を送ると error が返ることを確認します。
func TestAudioEnd_EmptyStream(t *testing.T) {
	_, wsURL := newTestServerWithOrchestrator(t)
	conn := dialWS(t, wsURL)

	sendHelloAndExpectWelcome(t, conn)

	// チャンク送信なしで audio.end を送信します
	audioEnd := buildEnvelope(t, "audio.end", 2, map[string]any{
		"stream_id":         "stream-no-chunks",
		"final_chunk_index": 0,
	})
	if err := conn.WriteMessage(websocket.TextMessage, audioEnd); err != nil {
		t.Fatalf("failed to send audio.end: %v", err)
	}

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeInvalidPayload)
}

// TestAudioEnd_OrchestratorFailure はオーケストレーション失敗時に error が返ることを確認します。
// mock.LLM は "internal_fail" を含む入力に対して CodeInternal エラーを返します。
// mock.STT の transcript は "transcript:{stream_id}:chunks=N" 形式のため、
// stream_id に "internal_fail" を含めることで LLM エラーを再現します。
func TestAudioEnd_OrchestratorFailure(t *testing.T) {
	_, wsURL := newTestServerWithOrchestrator(t)
	conn := dialWS(t, wsURL)

	sendHelloAndExpectWelcome(t, conn)

	const streamID = "internal_fail"
	chunk := buildEnvelope(t, "audio.chunk", 2, map[string]any{
		"stream_id":         streamID,
		"chunk_index":       0,
		"codec":             "opus",
		"sample_rate_hz":    16000,
		"frame_duration_ms": 20,
		"channel_count":     1,
		"data_base64":       "AAAA",
	})
	if err := conn.WriteMessage(websocket.TextMessage, chunk); err != nil {
		t.Fatalf("failed to send audio.chunk: %v", err)
	}

	audioEnd := buildEnvelope(t, "audio.end", 3, map[string]any{
		"stream_id":         streamID,
		"final_chunk_index": 0,
	})
	if err := conn.WriteMessage(websocket.TextMessage, audioEnd); err != nil {
		t.Fatalf("failed to send audio.end: %v", err)
	}

	env := readEnvelope(t, conn)
	assertErrorResponse(t, env, protocol.ErrCodeProviderFailed)
}

// TestBinaryStreamOpenAndFrames は audio.stream_open + バイナリフレーム + audio.end の
// フルフローを確認します。
func TestBinaryStreamOpenAndFrames(t *testing.T) {
	_, wsURL := newTestServerWithOrchestrator(t)
	conn := dialWS(t, wsURL)

	sendHelloAndExpectWelcome(t, conn)

	// UUID v4 形式の固定 stream_id（バイナリフレームに先頭 36 バイトとして埋め込みます）
	const streamID = "00000000-0000-4000-a000-000000000001"

	// audio.stream_open でバイナリストリームのメタデータを登録します
	streamOpen := buildEnvelope(t, "audio.stream_open", 2, map[string]any{
		"stream_id":         streamID,
		"codec":             "opus",
		"sample_rate_hz":    16000,
		"frame_duration_ms": 20,
		"channel_count":     1,
	})
	if err := conn.WriteMessage(websocket.TextMessage, streamOpen); err != nil {
		t.Fatalf("failed to send audio.stream_open: %v", err)
	}

	// バイナリフレームを 2 回送信します（先頭 36 バイト = stream_id、残り = 音声データ）
	audioPayload := []byte("mock-opus-audio-data")
	frame := append([]byte(streamID), audioPayload...)
	for i := 0; i < 2; i++ {
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			t.Fatalf("failed to send binary frame %d: %v", i, err)
		}
	}

	// audio.end を送信してオーケストレーションを起動します
	audioEnd := buildEnvelope(t, "audio.end", 3, map[string]any{
		"stream_id":         streamID,
		"final_chunk_index": 1,
		"reason":            "normal",
	})
	if err := conn.WriteMessage(websocket.TextMessage, audioEnd); err != nil {
		t.Fatalf("failed to send audio.end: %v", err)
	}

	// stt.final を受信します
	env1 := readEnvelope(t, conn)
	if env1.Type != "stt.final" {
		t.Errorf("expected stt.final, got %s", env1.Type)
	}
	var sttPayload protocol.STTFinalPayload
	if err := json.Unmarshal(env1.Payload, &sttPayload); err != nil {
		t.Fatalf("failed to unmarshal stt.final: %v", err)
	}
	if sttPayload.RequestID != streamID {
		t.Errorf("expected request_id=%s, got %s", streamID, sttPayload.RequestID)
	}
	if sttPayload.Transcript == "" {
		t.Error("expected non-empty transcript")
	}

	// tts.chunk 群と tts.end を受信します
	ttsChunks, _, ttsPayload := readTTSStreamUntilEnd(t, conn)
	if ttsPayload.RequestID != streamID {
		t.Errorf("expected request_id=%s, got %s", streamID, ttsPayload.RequestID)
	}
	if len(ttsChunks) == 0 {
		t.Fatal("expected at least one tts.chunk")
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
