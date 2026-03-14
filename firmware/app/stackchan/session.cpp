/**
 * @file session.cpp
 * @brief Stackchan セッション・オーケストレーション実装
 * 
 * P5-06: session.hello/welcome フロー
 * P5-07: heartbeat 送受信
 * P5-08: 最小音声送信フロー（audio.stream_open → binary → audio.end）
 * P5-09: stt.final / tts.end 受信とデバッグログ
 */
#include "session.h"
#include <ArduinoJson.h>

namespace App {

StackchanSession::StackchanSession()
    : _mic(FW_AUDIO_SAMPLE_RATE, FW_AUDIO_FRAME_MS) {}

// ──────────────────────────────────────────────────────────────────────
// Public: begin / loop
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::begin() {
  Serial.println("[Session] begin()");

  _mic.begin();

  // WebSocket クライアントの設定
  _ws.setUrl(String(FW_WS_URL));
  _ws.setReconnectPolicy({FW_RECONNECT_BASE_MS, FW_RECONNECT_MAX_MS});

  // FW_WS_TOKEN が設定されていれば Authorization ヘッダに付与します（値はログ非出力）
#ifdef FW_WS_TOKEN
  {
    String token = String(FW_WS_TOKEN);
    if (token.length() > 0 && token != "OPTIONAL_WS_TOKEN") {
      _ws.setToken(token);
    }
  }
#endif

  // コールバック登録
  _ws.onConnected([this]() { onWSConnected(); });
  _ws.onDisconnected([this]() { onWSDisconnected(); });
  _ws.onTextMessage([this](const String& msg) { onTextMessage(msg); });

  // Wi-Fi 接続を開始します
  setState(SessionState::ConnectingWiFi);
  if (Network::connect()) {
    setState(SessionState::ConnectingWS);
    _ws.begin();
  } else {
    // 接続失敗: loop() 内で指数バックオフ再試行します
    Serial.println("[Session] WiFi connect failed at startup, will retry in loop()");
  }
}

void StackchanSession::loop() {
  // Wi-Fi 未接続時: 指数バックオフで再接続を試みます
  if (_state == SessionState::Idle || _state == SessionState::ConnectingWiFi) {
    if (!Network::isConnected()) {
      if (millis() - _lastWiFiAttemptMs >= _wifiRetryDelayMs) {
        _lastWiFiAttemptMs = millis();
        if (Network::connect(5000)) {
          setState(SessionState::ConnectingWS);
          _ws.begin();
          // 次回待機時間をリセットします
          _wifiRetryDelayMs = FW_RECONNECT_BASE_MS;
        } else {
          // 指数バックオフ: 最大 FW_RECONNECT_MAX_MS までキャップします
          _wifiRetryDelayMs = min(_wifiRetryDelayMs * 2,
                                   static_cast<unsigned long>(FW_RECONNECT_MAX_MS));
          Serial.printf("[Session] WiFi retry delay → %lu ms\n", _wifiRetryDelayMs);
        }
      }
    }
    return;
  }

  // WebSocket のループ処理（自動再接続もここで実行されます）
  _ws.loop();

  // Active 状態でのみ heartbeat を定期送信します
  if (_state == SessionState::Active) {
    if (millis() - _lastHeartbeatMs >= _heartbeatIntervalMs) {
      sendHeartbeat();
    }
  }
}

// ──────────────────────────────────────────────────────────────────────
// Public: sendAudioStream
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::sendAudioStream(int frameCount) {
  // Active 状態でない場合は送信をスキップします
  if (_state != SessionState::Active) {
    Serial.printf("[AudioSend] Skipped: state=%d (not Active)\n", (int)_state);
    return;
  }

  // UUID v4 で stream_id を生成します
  String streamId = Protocol::generateUUIDv4();
  Serial.printf("[AudioSend] Start: stream_id=%s frames=%d\n",
    streamId.c_str(), frameCount);

  // ── Step 1: audio.stream_open を送信します ─────────────────────────
  JsonDocument openPayload;
  openPayload["stream_id"]         = streamId;
  openPayload["codec"]             = "pcm";  // Phase 5 は raw PCM（Phase 6 で opus に変更）
  openPayload["sample_rate_hz"]    = _mic.sampleRateHz();
  openPayload["frame_duration_ms"] = _mic.frameDurationMs();
  openPayload["channel_count"]     = 1;

  String openPayloadStr;
  serializeJson(openPayload, openPayloadStr);
  String openEnv = Protocol::buildEnvelope(
    Protocol::EventType::AUDIO_STREAM_OPEN, _sessionId, _seq.next(), openPayloadStr);

  if (!_ws.sendText(openEnv)) {
    Serial.println("[AudioSend] audio.stream_open send failed");
    return;
  }
  Serial.println("[AudioSend] audio.stream_open sent");

  // ── Step 2: バイナリフレームを送信します ──────────────────────────
  // フレームフォーマット: [stream_id(36 bytes ASCII)][PCM data(N bytes)]
  const size_t frameBytes = _mic.frameSizeBytes();
  const size_t totalBytes = 36 + frameBytes;

  // スタック上にバッファを確保します（最大 676 bytes = 36 + 16000*20/1000*2）
  uint8_t frameBuf[36 + 640];  // FW_AUDIO_SAMPLE_RATE=16000, FW_AUDIO_FRAME_MS=20 の最大値
  if (totalBytes > sizeof(frameBuf)) {
    Serial.printf("[AudioSend] Frame too large: %zu > %zu\n", totalBytes, sizeof(frameBuf));
    return;
  }

  // 先頭 36 バイトに stream_id ASCII 文字列をコピーします（NULL 終端なし）
  memcpy(frameBuf, streamId.c_str(), 36);

  for (int i = 0; i < frameCount; i++) {
    // PCM データを読み取ります（Phase 5: ゼロ PCM）
    _mic.readFrame(frameBuf + 36, frameBytes);

    if (!_ws.sendBinary(frameBuf, totalBytes)) {
      Serial.printf("[AudioSend] Binary frame %d send failed\n", i + 1);
      break;
    }

    // 最初・最後のフレームのみログ出力します（ログ量を抑制）
    if (i == 0 || i == frameCount - 1) {
      Serial.printf("[AudioSend] Frame %d/%d sent (%zu bytes)\n",
        i + 1, frameCount, totalBytes);
    }

    // FW_AUDIO_FRAME_MS の間隔でフレームを送信します
    // NOTE: 本 delay はブロッキングです。Phase 6 でタスクベースに置き換えてください。
    delay(FW_AUDIO_FRAME_MS);
  }

  // ── Step 3: audio.end を送信します ────────────────────────────────
  JsonDocument endPayload;
  endPayload["stream_id"]         = streamId;
  endPayload["final_chunk_index"] = frameCount - 1;
  endPayload["reason"]            = "normal";

  String endPayloadStr;
  serializeJson(endPayload, endPayloadStr);
  String endEnv = Protocol::buildEnvelope(
    Protocol::EventType::AUDIO_END, _sessionId, _seq.next(), endPayloadStr);

  if (_ws.sendText(endEnv)) {
    Serial.printf("[AudioSend] audio.end sent (stream_id=%s final_chunk_index=%d)\n",
      streamId.c_str(), frameCount - 1);
  }
}

// ──────────────────────────────────────────────────────────────────────
// Private: 状態遷移
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::setState(SessionState next) {
  if (_state == next) return;
  Serial.printf("[Session] State: %d → %d\n", (int)_state, (int)next);
  _state = next;
}

// ──────────────────────────────────────────────────────────────────────
// Private: WebSocket コールバック
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::onWSConnected() {
  setState(SessionState::Handshaking);
  // 再接続時はシーケンスをリセットして新しい hello を送信します
  _seq.reset();
  _sessionId = "";
  sendHello();
}

void StackchanSession::onWSDisconnected() {
  setState(SessionState::ConnectingWS);
  // _ws の指数バックオフ再接続が自動で動作します
}

// ──────────────────────────────────────────────────────────────────────
// Private: 送信ヘルパー
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::sendHello() {
  JsonDocument payload;
  payload["device_id"]   = FW_DEVICE_ID;
  payload["client_type"] = "firmware";
  // 音声チャンク / audio.end に対応していることを宣言します
  JsonObject caps = payload["protocol_capabilities"].to<JsonObject>();
  caps["audio_chunk"] = true;
  caps["audio_end"]   = true;

  String payloadStr;
  serializeJson(payload, payloadStr);

  // session.hello では session_id は空文字列を使います
  String env = Protocol::buildEnvelope(
    Protocol::EventType::SESSION_HELLO, "", _seq.next(), payloadStr);

  _ws.sendText(env);
  Serial.printf("[Session] session.hello sent (device_id=%s)\n", FW_DEVICE_ID);
}

void StackchanSession::sendHeartbeat() {
  JsonDocument payload;
  payload["uptime_ms"] = millis();
  payload["rssi"]      = Network::getRSSI();  // dBm

  String payloadStr;
  serializeJson(payload, payloadStr);

  String env = Protocol::buildEnvelope(
    Protocol::EventType::HEARTBEAT, _sessionId, _seq.next(), payloadStr);

  if (_ws.sendText(env)) {
    _lastHeartbeatMs = millis();
    Serial.printf("[Session] heartbeat sent (uptime=%lu ms, rssi=%d dBm)\n",
      millis(), Network::getRSSI());
  }
}

// ──────────────────────────────────────────────────────────────────────
// Private: 受信ディスパッチ
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::onTextMessage(const String& msg) {
  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, msg);
  if (err) {
    Serial.printf("[Session] JSON parse error: %s (len=%d)\n", err.c_str(), msg.length());
    return;
  }

  const char* type          = doc["type"]       | "";
  const char* envSessionId  = doc["session_id"] | "";

  // ペイロードを文字列に再シリアライズして各ハンドラに渡します
  String payloadStr;
  serializeJson(doc["payload"], payloadStr);

  if (strcmp(type, Protocol::EventType::SESSION_WELCOME) == 0) {
    handleWelcome(payloadStr, String(envSessionId));
  } else if (strcmp(type, Protocol::EventType::STT_FINAL) == 0) {
    handleSTTFinal(payloadStr);
  } else if (strcmp(type, Protocol::EventType::TTS_END) == 0) {
    handleTTSEnd(payloadStr);
  } else if (strcmp(type, Protocol::EventType::ERROR_EVENT) == 0) {
    handleError(payloadStr);
  } else {
    Serial.printf("[Session] Unhandled event type: %s\n", type);
  }
}

// ──────────────────────────────────────────────────────────────────────
// Private: 受信イベントハンドラ
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::handleWelcome(const String& payloadJson,
                                     const String& envelopeSessionId) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  bool accepted = payload["accepted"] | false;
  if (!accepted) {
    Serial.println("[Session] welcome: accepted=false → closing session");
    setState(SessionState::Error);
    return;
  }

  // エンベロープの session_id を保存します（以降のメッセージで使用します）
  if (envelopeSessionId.length() > 0) {
    _sessionId = envelopeSessionId;
  }

  // heartbeat_interval_ms を取得します（省略時はデフォルト値を使用）
  _heartbeatIntervalMs = payload["heartbeat_interval_ms"] | FW_HEARTBEAT_INTERVAL_MS;

  setState(SessionState::Active);
  _lastHeartbeatMs = millis();

  Serial.printf("[Session] welcome accepted → Active\n");
  Serial.printf("[Session] session_id=%s heartbeat_interval_ms=%lu\n",
    _sessionId.c_str(), _heartbeatIntervalMs);
}

void StackchanSession::handleSTTFinal(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId  = payload["request_id"]  | "";
  const char* transcript = payload["transcript"]   | "";
  float       confidence = payload["confidence"]   | -1.0f;

  if (confidence >= 0.0f) {
    Serial.printf("[STT] request_id=%s transcript=\"%s\" confidence=%.2f\n",
      requestId, transcript, confidence);
  } else {
    Serial.printf("[STT] request_id=%s transcript=\"%s\"\n",
      requestId, transcript);
  }
}

void StackchanSession::handleTTSEnd(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId    = payload["request_id"]    | "";
  const char* codec        = payload["codec"]         | "";
  int         durationMs   = payload["duration_ms"]   | 0;
  int         sampleRateHz = payload["sample_rate_hz"] | 0;

  // audio_base64 はフェーズ 6 でデコード予定。シリアルバッファ溢れを防ぐため値は出力しません。
  const char* audioBase64 = payload["audio_base64"] | "";
  size_t      b64Len      = strlen(audioBase64);

  Serial.printf("[TTS] request_id=%s codec=%s duration_ms=%d sample_rate_hz=%d base64_len=%zu\n",
    requestId, codec, durationMs, sampleRateHz, b64Len);
  Serial.println("[TTS] NOTE: audio_base64 decoding deferred to Phase 6");
}

void StackchanSession::handleError(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* code    = payload["code"]      | "";
  const char* message = payload["message"]   | "";
  bool        retry   = payload["retryable"] | false;

  Serial.printf("[Session] ERROR code=%s message=%s retryable=%d\n",
    code, message, (int)retry);
}

}  // namespace App
