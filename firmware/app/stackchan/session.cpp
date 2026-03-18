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
#include <M5Unified.h>
#include <mbedtls/base64.h>
extern "C" {
#include <opus.h>
}

namespace App {

StackchanSession::StackchanSession()
    : _mic(FW_AUDIO_SAMPLE_RATE, FW_AUDIO_FRAME_MS) {}

// ──────────────────────────────────────────────────────────────────────
// Public: begin / loop
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::begin() {
  Serial.println("[Session] begin()");
  setConversationState(ConversationState::Idle, "boot completed");

  _mic.begin();
  _ttsPlayer.begin();

  // P8-07: M5Stack-Avatar の顔描画を開始します。
  _avatar.init();
  _avatar.setExpression(m5avatar::Expression::Neutral);
  _avatar.setSpeechText("Connecting...");
  _avatarReady = true;

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

  // ┌─────────────────────────────────────────────────────────────────────────┐
  // │ P8-17: 受信・消費・表示の責務分離（Producer-Consumer pattern）          │
  // ├─────────────────────────────────────────────────────────────────────────┤
  // │ Producer（受信側）: _ws.loop()                                         │
  // │   → onTextMessage() → enqueueTTSFrame() で frame を enqueue            │
  // │   → Non-blocking: フレーム受信をキューに積み込むのみ                  │
  // │                                                                         │
  // │ Consumer（消費側）: processTTSPlaybackQueue()                          │
  // │   → キューから dequeue → 40ms 分を集約 → playPCM16() で再生          │
  // │   → キュー watermark 監視（prebuffer / low-water / high-water）       │
  // │   → observability: バッファ深さ、滞留時間をログ出力                   │
  // │                                                                         │
  // │ 効果: 受信遅延が再生に直接影響しない → より安定したストリーミング      │
  // └─────────────────────────────────────────────────────────────────────────┘

  // ── Producer: WebSocket ノンブロッキング受信 ──────────────────────────────
  // 受信フレーム到達時に自動的に onTextMessage() → enqueueTTSFrame() が実行されます
  _ws.loop();

  // ── TTS 再生状態の更新 ────────────────────────────────────────────
  _ttsPlayer.update();

  // ── Consumer: キューから消費・再生 ────────────────────────────────────
  // processTTSPlaybackQueue() はキュー監視・watermark チェック・dequeue を担当します
  // 受信フロー（producer）と分離されているため、互いに独立して進捗できます
  processTTSPlaybackQueue();

  // ── Display: 口パク同期・顔表情更新 ────────────────────────────────────
  updateAvatarFace();

  // ── 状態遷移: speaking -> idle の検知 ────────────────────────────────────
  const Audio::PlaybackState nowPlaybackState = _ttsPlayer.state();
  if (_lastPlaybackState == Audio::PlaybackState::Playing &&
      nowPlaybackState == Audio::PlaybackState::Idle &&
      _conversationState == ConversationState::Speaking) {
    setConversationState(ConversationState::Idle, "tts playback finished");
  }
  _lastPlaybackState = nowPlaybackState;

  // ── 定期送信: heartbeat（Active 状態のみ） ────────────────────────────
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
  setConversationState(ConversationState::Listening, "audio capture started");
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
    setConversationState(ConversationState::Thinking, "audio.end sent");
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

void StackchanSession::setConversationState(ConversationState next, const char* reason) {
  if (_conversationState == next) {
    return;
  }
  Serial.printf("[Conversation] State: %s -> %s reason=%s\n",
                conversationStateName(_conversationState),
                conversationStateName(next),
                reason == nullptr ? "(none)" : reason);
  _conversationState = next;
}

const char* StackchanSession::conversationStateName(ConversationState state) const {
  switch (state) {
    case ConversationState::Idle:
      return "idle";
    case ConversationState::Listening:
      return "listening";
    case ConversationState::Thinking:
      return "thinking";
    case ConversationState::Speaking:
      return "speaking";
    case ConversationState::Interrupted:
      return "interrupted";
    case ConversationState::Error:
      return "error";
    default:
      return "unknown";
  }
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
  _ttsPlayer.stop();
  clearTTSFrameQueue();
  clearIncomingTTSBuffer();
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
  } else if (strcmp(type, Protocol::EventType::TTS_CHUNK) == 0) {
    handleTTSChunk(payloadStr);
  } else if (strcmp(type, Protocol::EventType::TTS_END) == 0) {
    handleTTSEnd(payloadStr);
  } else if (strcmp(type, Protocol::EventType::AVATAR_EXPRESSION) == 0) {
    handleAvatarExpression(payloadStr);
  } else if (strcmp(type, Protocol::EventType::MOTION_PLAY) == 0) {
    handleMotionPlay(payloadStr);
  } else if (strcmp(type, Protocol::EventType::CONVERSATION_CANCEL) == 0) {
    handleConversationCancel(payloadStr);
  } else if (strcmp(type, Protocol::EventType::TTS_STOP) == 0) {
    handleTTSStop(payloadStr);
  } else if (strcmp(type, Protocol::EventType::AUDIO_STREAM_ABORT) == 0) {
    handleAudioStreamAbort(payloadStr);
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
  setConversationState(ConversationState::Idle, "session welcome accepted");
  _lastHeartbeatMs = millis();
  if (_avatarReady) {
    _avatar.setSpeechText("Ready");
  }

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
  setConversationState(ConversationState::Thinking, "stt.final received");
}

void StackchanSession::handleTTSChunk(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* streamId = payload["stream_id"] | "";
  const char* codec = payload["codec"] | "pcm";
  int chunkIndex = payload["chunk_index"] | -1;
  int frameDurationMs = payload["frame_duration_ms"] | 0;
  int samplesPerChunk = payload["samples_per_chunk"] | 0;
  int totalChunks = payload["total_chunks"] | 0;
  const char* audioBase64 = payload["audio_base64"] | "";

  // P8-15: v1.1 形式（stream_id + frame_duration_ms + samples_per_chunk）を優先処理します。
  if (String(streamId).length() > 0 && frameDurationMs > 0 && samplesPerChunk > 0) {
    if (!enqueueTTSFrame(String(requestId), String(streamId), chunkIndex, frameDurationMs, samplesPerChunk, String(audioBase64), String(codec))) {
      Serial.printf("[TTS] request_id=%s stream_id=%s frame enqueue failed idx=%d\n",
        requestId, streamId, chunkIndex);
      return;
    }

    Serial.printf("[TTS] request_id=%s stream_id=%s codec=%s frame queued idx=%d buffered_ms=%u frames=%u\n",
      requestId,
      streamId,
      codec,
      chunkIndex,
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(_ttsFrameCount));
    return;
  }

  if (!appendIncomingTTSChunk(String(requestId), chunkIndex, totalChunks, String(audioBase64))) {
    Serial.printf("[TTS] request_id=%s chunk append failed idx=%d total=%d\n",
      requestId, chunkIndex, totalChunks);
    return;
  }

  Serial.printf("[TTS] request_id=%s chunk received %d/%d total_bytes=%u\n",
    requestId,
    chunkIndex + 1,
    totalChunks,
    static_cast<unsigned>(_incomingTTSBufferLen));
}

void StackchanSession::handleTTSEnd(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId  = payload["request_id"] | "";
  const char* codec      = payload["codec"] | "";
  int durationMs         = payload["duration_ms"] | 0;
  int sampleRateHz       = payload["sample_rate_hz"] | 0;
  const char* audioBase64 = payload["audio_base64"] | "";
  int totalChunks        = payload["total_chunks"] | 0;

  _currentRequestId = String(requestId);

  // P8-15: フレームキュー方式（v1.1）では tts.end をストリーム終端として扱います。
  if (_ttsStreamRequestId == String(requestId)) {
    if (String(codec).length() > 0) {
      _ttsStreamCodec = String(codec);
    }
    if (sampleRateHz > 0) {
      _ttsSampleRateHz = static_cast<uint32_t>(sampleRateHz);
    }
    _ttsStreamEnded = true;
    Serial.printf("[TTS] request_id=%s stream playback pending codec=%s buffered_ms=%u frames=%u sample_rate_hz=%u\n",
      requestId,
      _ttsStreamCodec.c_str(),
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(_ttsFrameCount),
      static_cast<unsigned>(_ttsSampleRateHz));
    return;
  }

  // Phase 6 最小実装: PCM 音声を想定して再生します（opus は未対応）。
  if (strcmp(codec, "pcm") != 0) {
    Serial.printf("[TTS] request_id=%s codec=%s is not supported yet, fallback beep\n", requestId, codec);
    M5.Speaker.tone(1200, 80);
    return;
  }

  uint8_t* playbackBytes = nullptr;
  size_t playbackLen = 0;

  if (_incomingTTSRequestId == String(requestId) && _incomingTTSBuffer != nullptr) {
    if (totalChunks > 0 && _incomingTTSReceivedChunks != totalChunks) {
      Serial.printf("[TTS] request_id=%s chunk count mismatch received=%d expected=%d\n",
        requestId,
        _incomingTTSReceivedChunks,
        totalChunks);
      clearIncomingTTSBuffer();
      return;
    }
    playbackBytes = _incomingTTSBuffer;
    playbackLen = _incomingTTSBufferLen;
  } else {
    uint8_t* decoded = nullptr;
    size_t decodedLen = 0;
    if (!decodeBase64(String(audioBase64), &decoded, &decodedLen)) {
      Serial.printf("[TTS] request_id=%s base64 decode failed\n", requestId);
      return;
    }
    playbackBytes = decoded;
    playbackLen = decodedLen;
  }

  const bool started = _ttsPlayer.playPCM16(playbackBytes, playbackLen, static_cast<uint32_t>(sampleRateHz), true);

  if (playbackBytes == _incomingTTSBuffer) {
    clearIncomingTTSBuffer();
  } else if (playbackBytes != nullptr) {
    free(playbackBytes);
  }

  if (!started) {
    Serial.printf("[TTS] request_id=%s playback start failed\n", requestId);
    return;
  }

  setConversationState(ConversationState::Speaking, "tts.end playback started");

  Serial.printf("[TTS] request_id=%s playback started codec=%s duration_ms=%d sample_rate_hz=%d decoded_bytes=%u start_latency_ms=%u chunks=%d\n",
    requestId,
    codec,
    durationMs,
    sampleRateHz,
    static_cast<unsigned>(playbackLen),
    static_cast<unsigned>(_ttsPlayer.startLatencyMs()),
    totalChunks);
}

void StackchanSession::handleAvatarExpression(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* expression = payload["expression"] | "neutral";
  _expression = String(expression);
  if (_avatarReady) {
    _avatar.setExpression(toAvatarExpression(_expression));
  }
  Serial.printf("[Avatar] expression=%s\n", _expression.c_str());
}

void StackchanSession::handleMotionPlay(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* motion = payload["motion"] | "idle";
  _motion = String(motion);

  // フェーズ 6 では安全な最小モーション（通知 + 軽いビープ）のみ実装します。
  if (_motion == "nod") {
    M5.Speaker.tone(900, 40);
  } else if (_motion == "shake") {
    M5.Speaker.tone(700, 40);
  }

  if (_avatarReady) {
    if (_motion == "nod") {
      _avatar.setRotation(0.10f);
    } else if (_motion == "shake") {
      _avatar.setRotation(-0.10f);
    } else {
      _avatar.setRotation(0.0f);
    }
  }

  Serial.printf("[Avatar] motion=%s\n", _motion.c_str());
}

void StackchanSession::handleConversationCancel(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* reason = payload["reason"] | "cancelled";

  Serial.printf("[Interrupt] conversation.cancel request_id=%s reason=%s\n", requestId, reason);
  setConversationState(ConversationState::Interrupted, "conversation.cancel received");
  _ttsPlayer.stop();
  clearTTSFrameQueue();
  clearIncomingTTSBuffer();
  setConversationState(ConversationState::Idle, "conversation.cancel applied");
}

void StackchanSession::handleTTSStop(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* reason = payload["reason"] | "stopped";

  Serial.printf("[Interrupt] tts.stop request_id=%s reason=%s\n", requestId, reason);
  setConversationState(ConversationState::Interrupted, "tts.stop received");
  _ttsPlayer.stop();
  clearTTSFrameQueue();
  clearIncomingTTSBuffer();
  setConversationState(ConversationState::Idle, "tts.stop applied");
}

void StackchanSession::handleAudioStreamAbort(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* streamId = payload["stream_id"] | "";
  const char* reason = payload["reason"] | "aborted";

  Serial.printf("[Interrupt] audio.stream_abort stream_id=%s reason=%s\n", streamId, reason);
  setConversationState(ConversationState::Interrupted, "audio.stream_abort received");
  clearTTSFrameQueue();
  clearIncomingTTSBuffer();
  setConversationState(ConversationState::Idle, "audio.stream_abort applied");
}

void StackchanSession::handleError(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* code    = payload["code"]      | "";
  const char* message = payload["message"]   | "";
  bool        retry   = payload["retryable"] | false;

  Serial.printf("[Session] ERROR code=%s message=%s retryable=%d\n",
    code, message, (int)retry);

  if (!retry) {
    setConversationState(ConversationState::Error, "non-retryable error event");
  }
}

void StackchanSession::updateAvatarFace() {
  // 更新は 80ms 周期に制限します（描画処理の負荷抑制）。
  const unsigned long now = millis();
  if (now - _lastAvatarRenderMs < 80) {
    return;
  }
  _lastAvatarRenderMs = now;

  if (!_avatarReady) {
    return;
  }

  const float lip = _ttsPlayer.lipLevel();
  _avatar.setMouthOpenRatio(lip);

  // 再生中のみ口パクメタ情報を表示し、待機時は最小ラベル表示に戻します。
  if (_ttsPlayer.state() == Audio::PlaybackState::Playing) {
    String speech = String("Req:") + _currentRequestId;
    _avatar.setSpeechText(speech.c_str());
  } else {
    _avatar.setSpeechText(_expression.c_str());
  }

  // 回転は毎周期で減衰させ、モーション演出後に自然復帰させます。
  if (_motion == "nod" || _motion == "shake") {
    _avatar.setRotation(0.0f);
    _motion = "idle";
  }
}

m5avatar::Expression StackchanSession::toAvatarExpression(const String& expression) const {
  if (expression == "happy") {
    return m5avatar::Expression::Happy;
  }
  if (expression == "sad") {
    return m5avatar::Expression::Sad;
  }
  if (expression == "surprised") {
    return m5avatar::Expression::Doubt;
  }
  if (expression == "angry") {
    return m5avatar::Expression::Angry;
  }
  return m5avatar::Expression::Neutral;
}

bool StackchanSession::decodeBase64(const String& src, uint8_t** out, size_t* outLen) {
  if (out == nullptr || outLen == nullptr) {
    return false;
  }
  *out = nullptr;
  *outLen = 0;

  if (src.length() == 0) {
    return false;
  }

  // Base64 展開後サイズの上限見積り
  const size_t maxLen = (src.length() * 3) / 4 + 4;
  uint8_t* buffer = static_cast<uint8_t*>(malloc(maxLen));
  if (buffer == nullptr) {
    return false;
  }

  size_t written = 0;
  const int rc = mbedtls_base64_decode(
      buffer,
      maxLen,
      &written,
      reinterpret_cast<const unsigned char*>(src.c_str()),
      src.length());
  if (rc != 0 || written == 0) {
    free(buffer);
    return false;
  }

  *out = buffer;
  *outLen = written;
  return true;
}

void StackchanSession::clearIncomingTTSBuffer() {
  if (_incomingTTSBuffer != nullptr) {
    free(_incomingTTSBuffer);
    _incomingTTSBuffer = nullptr;
  }
  _incomingTTSBufferLen = 0;
  _incomingTTSExpectedChunks = 0;
  _incomingTTSReceivedChunks = 0;
  _incomingTTSRequestId = "";
}

bool StackchanSession::appendIncomingTTSChunk(const String& requestId, int chunkIndex, int totalChunks, const String& audioBase64) {
  if (requestId.length() == 0 || chunkIndex < 0 || totalChunks <= 0 || audioBase64.length() == 0) {
    return false;
  }

  if (_incomingTTSRequestId != requestId) {
    clearIncomingTTSBuffer();
    _incomingTTSRequestId = requestId;
    _incomingTTSExpectedChunks = totalChunks;
  }

  if (chunkIndex != _incomingTTSReceivedChunks) {
    Serial.printf("[TTS] request_id=%s unexpected chunk index=%d expected=%d\n",
      requestId.c_str(), chunkIndex, _incomingTTSReceivedChunks);
    clearIncomingTTSBuffer();
    return false;
  }

  uint8_t* decoded = nullptr;
  size_t decodedLen = 0;
  if (!decodeBase64(audioBase64, &decoded, &decodedLen)) {
    return false;
  }

  uint8_t* next = static_cast<uint8_t*>(realloc(_incomingTTSBuffer, _incomingTTSBufferLen + decodedLen));
  if (next == nullptr) {
    free(decoded);
    clearIncomingTTSBuffer();
    return false;
  }

  _incomingTTSBuffer = next;
  memcpy(_incomingTTSBuffer + _incomingTTSBufferLen, decoded, decodedLen);
  _incomingTTSBufferLen += decodedLen;
  _incomingTTSReceivedChunks++;
  _incomingTTSExpectedChunks = totalChunks;

  free(decoded);
  return true;
}

void StackchanSession::clearTTSFrameQueue() {
  for (size_t i = 0; i < kTTSFrameQueueCapacity; i++) {
    if (_ttsFrameQueue[i].bytes != nullptr) {
      free(_ttsFrameQueue[i].bytes);
      _ttsFrameQueue[i].bytes = nullptr;
    }
    _ttsFrameQueue[i].byteLen = 0;
    _ttsFrameQueue[i].frameDurationMs = 0;
    _ttsFrameQueue[i].samplesPerChunk = 0;
    _ttsFrameQueue[i].chunkIndex = 0;
  }

  _ttsFrameHead = 0;
  _ttsFrameTail = 0;
  _ttsFrameCount = 0;
  _ttsBufferedMs = 0;
  _ttsPlaybackPrimed = false;
  _ttsStreamEnded = false;
  _ttsExpectedChunkIndex = 0;
  _ttsStreamRequestId = "";
  _ttsStreamId = "";
  _ttsStreamCodec = "pcm";
  resetOpusDecoder();

  // P8-16: concealment 状態をリセットします。
  if (_ttsLastGoodFrameBytes != nullptr) {
    free(_ttsLastGoodFrameBytes);
    _ttsLastGoodFrameBytes = nullptr;
  }
  _ttsLastGoodFrameLen = 0;
  _ttsMissingChunkCount = 0;
  _ttsConcealmentFrameCount = 0;
}

// P8-16: concealment（欠落補完）フレームをキューに挿入します。
void StackchanSession::insertConcealmentFrames(int gapCount, int frameDurationMs, int samplesPerChunk) {
  // 挿入するフレーム数を上限でキャップします（過度な無音挿入を防ぎます）。
  const int insertCount = min(gapCount, kMaxConcealmentFrames);
  const size_t frameByteLen = static_cast<size_t>(samplesPerChunk) * 2; // PCM16 mono

  for (int i = 0; i < insertCount; i++) {
    if (_ttsFrameCount >= kTTSFrameQueueCapacity) {
      Serial.printf("[TTS][concealment] queue full, cannot insert frame %d/%d\n", i + 1, insertCount);
      break;
    }

    const uint32_t nextBufferedMs = _ttsBufferedMs + static_cast<uint32_t>(frameDurationMs);
    if (nextBufferedMs > kTTSHighWaterMs) {
      Serial.printf("[TTS][concealment] high-water reached at frame %d/%d, stopping insertion\n",
        i + 1, insertCount);
      break;
    }

    uint8_t* concealFrame = static_cast<uint8_t*>(malloc(frameByteLen));
    if (concealFrame == nullptr) {
      Serial.printf("[TTS][concealment] malloc failed for frame %d/%d\n", i + 1, insertCount);
      break;
    }

    if (_ttsLastGoodFrameBytes != nullptr && _ttsLastGoodFrameLen == frameByteLen) {
      // 減衰コピー: 直前のフレームを振幅 50% に減らしてコピーします。
      // 再生グリッチを目立ちにくくしながら音声継続感を保ちます。
      // i=0 → 0.5倍、i=1 → 0.25倍 と徐々にフェードアウトします。
      const int16_t* src = reinterpret_cast<const int16_t*>(_ttsLastGoodFrameBytes);
      int16_t* dst = reinterpret_cast<int16_t*>(concealFrame);
      const size_t sampleCount = frameByteLen / 2;
      const int shiftBits = i + 1; // 1ビット右シフトで 1/2^(i+1) 倍に減衰
      for (size_t s = 0; s < sampleCount; s++) {
        dst[s] = static_cast<int16_t>(src[s] >> shiftBits);
      }
    } else {
      // 無音補完: 量子化ノイズを避けるためゼロ埋めします。
      memset(concealFrame, 0, frameByteLen);
    }

    TTSFrameSlot& slot = _ttsFrameQueue[_ttsFrameTail];
    slot.bytes = concealFrame;
    slot.byteLen = frameByteLen;
    slot.frameDurationMs = static_cast<uint16_t>(frameDurationMs);
    slot.samplesPerChunk = static_cast<uint16_t>(samplesPerChunk);
    slot.chunkIndex = _ttsExpectedChunkIndex + i; // 仮インデックス

    _ttsFrameTail = (_ttsFrameTail + 1) % kTTSFrameQueueCapacity;
    _ttsFrameCount++;
    _ttsBufferedMs += static_cast<uint32_t>(frameDurationMs);
    _ttsConcealmentFrameCount++;
  }

  // gapCount 分の期待インデックスを進めます（concealment 挿入数に関わらず）。
  _ttsExpectedChunkIndex += gapCount;

  Serial.printf("[TTS][concealment] request_id=%s stream_id=%s gap=%d inserted=%d total_missing=%d total_conc=%d\n",
    _ttsStreamRequestId.c_str(),
    _ttsStreamId.c_str(),
    gapCount,
    insertCount,
    _ttsMissingChunkCount,
    _ttsConcealmentFrameCount);
}

bool StackchanSession::enqueueTTSFrame(const String& requestId,                                       const String& streamId,
                                       int chunkIndex,
                                       int frameDurationMs,
                                       int samplesPerChunk,
                                       const String& audioBase64,
                                       const String& codec) {
  // ─────────────────────────────────────────────────────────────────────────
  // [Producer: TTS フレーム受信ハンドラ]
  // P8-17: WebSocket 受信イベントからの呼び出し
  //  → このメソッドはノンブロッキングで実行され、キューに frame を enqueue するのみ
  //  → base64 デコードは Consumer フロー（processTTSPlaybackQueue）で後行
  // ─────────────────────────────────────────────────────────────────────────
  if (requestId.length() == 0 ||
      streamId.length() == 0 ||
      chunkIndex < 0 ||
      frameDurationMs <= 0 ||
      samplesPerChunk <= 0 ||
      audioBase64.length() == 0) {
    return false;
  }

  if (_ttsStreamRequestId != requestId || _ttsStreamId != streamId) {
    if (_ttsPlayer.state() == Audio::PlaybackState::Playing ||
        _ttsPlayer.state() == Audio::PlaybackState::Buffering) {
      _ttsPlayer.stop();
    }
    clearTTSFrameQueue();
    _ttsStreamRequestId = requestId;
    _ttsStreamId = streamId;
    _ttsStreamCodec = codec.length() > 0 ? codec : "pcm";
    _ttsExpectedChunkIndex = 0;
  }

  if (codec.length() > 0 && _ttsStreamCodec != codec) {
    Serial.printf("[TTS] request_id=%s stream_id=%s codec changed %s -> %s (ignored)\n",
      requestId.c_str(),
      streamId.c_str(),
      _ttsStreamCodec.c_str(),
      codec.c_str());
  }

  if (chunkIndex != _ttsExpectedChunkIndex) {
    if (chunkIndex < _ttsExpectedChunkIndex) {
      // 過去のインデックスは重複送信として無視します（再送など）。
      Serial.printf("[TTS] request_id=%s stream_id=%s duplicate frame idx=%d expected=%d (skipped)\n",
        requestId.c_str(), streamId.c_str(), chunkIndex, _ttsExpectedChunkIndex);
      return true;
    }

    // chunkIndex > _ttsExpectedChunkIndex: ギャップを検出しました。
    const int gapCount = chunkIndex - _ttsExpectedChunkIndex;
    _ttsMissingChunkCount += gapCount;

    Serial.printf("[TTS] request_id=%s stream_id=%s gap detected missing=%d idx=%d expected=%d\n",
      requestId.c_str(), streamId.c_str(), gapCount, chunkIndex, _ttsExpectedChunkIndex);

    if (_ttsStreamCodec == "pcm") {
      // PCM は concealment フレームを挿入してギャップを埋めます。
      insertConcealmentFrames(gapCount, frameDurationMs, samplesPerChunk);
    } else {
      // Opus はデコーダ状態を維持しつつ、欠落分はスキップして先へ進みます。
      _ttsExpectedChunkIndex += gapCount;
    }
  }

  uint8_t* decoded = nullptr;
  size_t decodedLen = 0;
  if (!decodeBase64(audioBase64, &decoded, &decodedLen)) {
    return false;
  }

  if (_ttsFrameCount >= kTTSFrameQueueCapacity) {
    Serial.printf("[TTS] request_id=%s stream_id=%s frame queue overflow (capacity=%u)\n",
      requestId.c_str(),
      streamId.c_str(),
      static_cast<unsigned>(kTTSFrameQueueCapacity));
    free(decoded);
    _ttsExpectedChunkIndex++;
    return true;
  }

  // high-water 超過を避けるため、末尾フレームを受け入れる前に抑制します。
  const uint32_t nextBufferedMs = _ttsBufferedMs + static_cast<uint32_t>(frameDurationMs);
  if (nextBufferedMs > kTTSHighWaterMs) {
    Serial.printf("[TTS] request_id=%s stream_id=%s high-water drop idx=%d buffered_ms=%u limit_ms=%u\n",
      requestId.c_str(),
      streamId.c_str(),
      chunkIndex,
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(kTTSHighWaterMs));
    free(decoded);
    _ttsExpectedChunkIndex++;
    return true;
  }

  // P8-18 補足: ストリーム先頭frame（chunkIndex == 0）到達時にサンプルレートを即時推算します。
  // P8-15 の prebuffer 設計では tts.end 到着前に再生が始まるため、_ttsSampleRateHz が
  // 初期値（FW_AUDIO_SAMPLE_RATE）のままだと初回のみ音が低くこもる現象が発生します。
  // samplesPerChunk / frameDurationMs の比からレートを推算することで、tts.end を待たずに
  // 正しいサンプルレートで再生できるようにします。
  if (chunkIndex == 0 && samplesPerChunk > 0 && frameDurationMs > 0) {
    const uint32_t inferredHz =
        static_cast<uint32_t>(samplesPerChunk) * 1000u /
        static_cast<uint32_t>(frameDurationMs);
    if (inferredHz > 0 && inferredHz != _ttsSampleRateHz) {
      Serial.printf("[TTS] sample_rate_hz inferred from first chunk: %u -> %u\n",
                    static_cast<unsigned>(_ttsSampleRateHz),
                    static_cast<unsigned>(inferredHz));
      _ttsSampleRateHz = inferredHz;
    }
  }

  TTSFrameSlot& slot = _ttsFrameQueue[_ttsFrameTail];
  slot.bytes = decoded;
  slot.byteLen = decodedLen;
  slot.frameDurationMs = static_cast<uint16_t>(frameDurationMs);
  slot.samplesPerChunk = static_cast<uint16_t>(samplesPerChunk);
  slot.chunkIndex = chunkIndex;

  _ttsFrameTail = (_ttsFrameTail + 1) % kTTSFrameQueueCapacity;
  _ttsFrameCount++;
  _ttsBufferedMs += static_cast<uint32_t>(frameDurationMs);
  _ttsExpectedChunkIndex++;

  // P8-16: PCM のみ concealment の減衰コピー用に正常フレームを保持します。
  if (_ttsStreamCodec == "pcm") {
    if (_ttsLastGoodFrameBytes != nullptr) {
      free(_ttsLastGoodFrameBytes);
      _ttsLastGoodFrameBytes = nullptr;
    }
    _ttsLastGoodFrameBytes = static_cast<uint8_t*>(malloc(decodedLen));
    if (_ttsLastGoodFrameBytes != nullptr) {
      memcpy(_ttsLastGoodFrameBytes, decoded, decodedLen);
      _ttsLastGoodFrameLen = decodedLen;
    }
  }

  return true;
}

bool StackchanSession::dequeueTTSPlaybackBatch(uint16_t targetDurationMs,
                                               uint8_t** outBytes,
                                               size_t* outByteLen,
                                               uint16_t* outDurationMs) {
  if (outBytes == nullptr || outByteLen == nullptr || outDurationMs == nullptr) {
    return false;
  }
  *outBytes = nullptr;
  *outByteLen = 0;
  *outDurationMs = 0;

  if (_ttsFrameCount == 0) {
    return false;
  }

  const uint16_t durationLimit = targetDurationMs > 0 ? targetDurationMs : kTTSPlaybackBatchMs;

  size_t totalBytes = 0;
  uint16_t totalDuration = 0;
  size_t framesToPop = 0;
  size_t cursor = _ttsFrameHead;
  size_t available = _ttsFrameCount;

  while (available > 0) {
    const TTSFrameSlot& slot = _ttsFrameQueue[cursor];
    totalBytes += slot.byteLen;
    totalDuration = static_cast<uint16_t>(totalDuration + slot.frameDurationMs);
    framesToPop++;

    cursor = (cursor + 1) % kTTSFrameQueueCapacity;
    available--;

    if (totalDuration >= durationLimit) {
      break;
    }
  }

  if (framesToPop == 0 || totalBytes == 0) {
    return false;
  }

  uint8_t* merged = static_cast<uint8_t*>(malloc(totalBytes));
  if (merged == nullptr) {
    return false;
  }

  size_t offset = 0;
  for (size_t i = 0; i < framesToPop; i++) {
    TTSFrameSlot& slot = _ttsFrameQueue[_ttsFrameHead];
    memcpy(merged + offset, slot.bytes, slot.byteLen);
    offset += slot.byteLen;

    if (_ttsBufferedMs >= slot.frameDurationMs) {
      _ttsBufferedMs -= slot.frameDurationMs;
    } else {
      _ttsBufferedMs = 0;
    }

    if (slot.bytes != nullptr) {
      free(slot.bytes);
      slot.bytes = nullptr;
    }
    slot.byteLen = 0;
    slot.frameDurationMs = 0;
    slot.samplesPerChunk = 0;
    slot.chunkIndex = 0;

    _ttsFrameHead = (_ttsFrameHead + 1) % kTTSFrameQueueCapacity;
    _ttsFrameCount--;
  }

  *outBytes = merged;
  *outByteLen = totalBytes;
  *outDurationMs = totalDuration;
  return true;
}

bool StackchanSession::dequeueTTSFrame(TTSFrameSlot* outFrame) {
  if (outFrame == nullptr || _ttsFrameCount == 0) {
    return false;
  }

  TTSFrameSlot& slot = _ttsFrameQueue[_ttsFrameHead];
  *outFrame = slot;

  if (_ttsBufferedMs >= slot.frameDurationMs) {
    _ttsBufferedMs -= slot.frameDurationMs;
  } else {
    _ttsBufferedMs = 0;
  }

  slot.bytes = nullptr;
  slot.byteLen = 0;
  slot.frameDurationMs = 0;
  slot.samplesPerChunk = 0;
  slot.chunkIndex = 0;

  _ttsFrameHead = (_ttsFrameHead + 1) % kTTSFrameQueueCapacity;
  _ttsFrameCount--;
  return true;
}

void StackchanSession::resetOpusDecoder() {
  if (_ttsOpusDecoder != nullptr) {
    opus_decoder_destroy(static_cast<OpusDecoder*>(_ttsOpusDecoder));
    _ttsOpusDecoder = nullptr;
  }
  _ttsOpusDecoderSampleRateHz = 0;
}

bool StackchanSession::ensureOpusDecoder(uint32_t sampleRateHz) {
  if (sampleRateHz != 8000 && sampleRateHz != 12000 && sampleRateHz != 16000 && sampleRateHz != 24000 && sampleRateHz != 48000) {
    Serial.printf("[TTS][opus] unsupported decoder sample_rate_hz=%u\n", static_cast<unsigned>(sampleRateHz));
    return false;
  }

  if (_ttsOpusDecoder != nullptr && _ttsOpusDecoderSampleRateHz == sampleRateHz) {
    return true;
  }

  resetOpusDecoder();

  int err = OPUS_OK;
  OpusDecoder* decoder = opus_decoder_create(static_cast<opus_int32>(sampleRateHz), 1, &err);
  if (decoder == nullptr || err != OPUS_OK) {
    Serial.printf("[TTS][opus] decoder create failed err=%d\n", err);
    return false;
  }

  _ttsOpusDecoder = decoder;
  _ttsOpusDecoderSampleRateHz = sampleRateHz;
  return true;
}

bool StackchanSession::decodeOpusFrame(const uint8_t* opusBytes, size_t opusLen, uint32_t sampleRateHz, uint8_t** outPcmBytes, size_t* outPcmLen) {
  if (outPcmBytes == nullptr || outPcmLen == nullptr || opusBytes == nullptr || opusLen == 0) {
    return false;
  }
  *outPcmBytes = nullptr;
  *outPcmLen = 0;

  if (!ensureOpusDecoder(sampleRateHz)) {
    return false;
  }

  const int maxSamplesPerChannel = static_cast<int>((sampleRateHz * 60U) / 1000U);
  if (maxSamplesPerChannel <= 0) {
    return false;
  }

  int16_t* pcmBuffer = static_cast<int16_t*>(malloc(static_cast<size_t>(maxSamplesPerChannel) * sizeof(int16_t)));
  if (pcmBuffer == nullptr) {
    Serial.println("[TTS][opus] pcm decode buffer allocation failed");
    return false;
  }

  const int decodedSamples = opus_decode(
    static_cast<OpusDecoder*>(_ttsOpusDecoder),
    reinterpret_cast<const unsigned char*>(opusBytes),
    static_cast<opus_int32>(opusLen),
    pcmBuffer,
    maxSamplesPerChannel,
    0);
  if (decodedSamples < 0) {
    Serial.printf("[TTS][opus] decode failed code=%d\n", decodedSamples);
    free(pcmBuffer);
    return false;
  }

  *outPcmBytes = reinterpret_cast<uint8_t*>(pcmBuffer);
  *outPcmLen = static_cast<size_t>(decodedSamples) * sizeof(int16_t);
  return true;
}

void StackchanSession::processTTSPlaybackQueue() {
  // ─────────────────────────────────────────────────────────────────────────
  // [キュー終端処理]
  // ストリームが終了し、すべてのフレームが消費された場合をクリーンアップします。
  // ─────────────────────────────────────────────────────────────────────────
  if (_ttsFrameCount == 0) {
    if (_ttsStreamEnded && _ttsPlayer.state() == Audio::PlaybackState::Idle) {
      // P8-16: ストリーム終端で concealment メトリクスをログ出力します。
      if (_ttsMissingChunkCount > 0 || _ttsConcealmentFrameCount > 0) {
        Serial.printf("[TTS][metrics] stream_id=%s request_id=%s missing_chunks=%d concealment_frames=%d\n",
          _ttsStreamId.c_str(),
          _ttsStreamRequestId.c_str(),
          _ttsMissingChunkCount,
          _ttsConcealmentFrameCount);
      }
      clearTTSFrameQueue();
      if (_conversationState == ConversationState::Speaking) {
        setConversationState(ConversationState::Idle, "tts stream drained");
      }
    }
    return;
  }

  // 再生中の場合はスキップ（前フレームの再生完了を待ちます）
  if (_ttsPlayer.state() != Audio::PlaybackState::Idle) {
    return;
  }

  // ─────────────────────────────────────────────────────────────────────────
  // [事前バッファ制御]
  // ストリーム開始前に最小限のバッファレベル(prebuffer_start = 80ms)に達するまで
  // 再生開始を遅延させ、受信ジッターに対する耐性を強化します（P8-15）。
  // ─────────────────────────────────────────────────────────────────────────
  if (!_ttsPlaybackPrimed) {
    if (_ttsBufferedMs < kTTSPrebufferStartMs && !_ttsStreamEnded) {
      return;
    }
    _ttsPlaybackPrimed = true;
    Serial.printf("[TTS] prebuffer ready request_id=%s buffered_ms=%u threshold_ms=%u\n",
      _ttsStreamRequestId.c_str(),
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(kTTSPrebufferStartMs));
  }

  // ─────────────────────────────────────────────────────────────────────────
  // [Watermark 監視]
  // P8-17: キューの深さが watermark 閾値を超える状況をログ出力し、
  // ネットワーク揺らぎやデコード遅延が再生に與える影響を可視化します。
  // ─────────────────────────────────────────────────────────────────────────
  if (_ttsBufferedMs < kTTSLowWaterMs && !_ttsStreamEnded) {
    // low-water 警告: バッファが枯渇に近い状態
    // 原因: 受信フレーム到着遅延、デコード重い処理、ネットワーク遅延など
    Serial.printf("[TTS][watermark] low-water request_id=%s buffered_ms=%u threshold_ms=%u frames_in_queue=%u\n",
      _ttsStreamRequestId.c_str(),
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(kTTSLowWaterMs),
      static_cast<unsigned>(_ttsFrameCount));
  }

  if (_ttsStreamCodec == "opus") {
    // Opus は 1 フレームずつデコードし、PCM16 に戻して再生します。
    TTSFrameSlot frame;
    if (!dequeueTTSFrame(&frame)) {
      return;
    }

    uint8_t* decodedPcm = nullptr;
    size_t decodedPcmLen = 0;
    if (!decodeOpusFrame(frame.bytes, frame.byteLen, _ttsSampleRateHz, &decodedPcm, &decodedPcmLen)) {
      Serial.printf("[TTS][opus] request_id=%s decode failed idx=%d\n", _ttsStreamRequestId.c_str(), frame.chunkIndex);
      if (frame.bytes != nullptr) {
        free(frame.bytes);
      }
      clearTTSFrameQueue();
      return;
    }

    if (frame.bytes != nullptr) {
      free(frame.bytes);
    }

    const bool started = _ttsPlayer.playPCM16(decodedPcm, decodedPcmLen, _ttsSampleRateHz, true);
    free(decodedPcm);

    if (!started) {
      Serial.printf("[TTS][opus] request_id=%s playback start failed decoded_bytes=%u\n",
        _ttsStreamRequestId.c_str(),
        static_cast<unsigned>(decodedPcmLen));
      clearTTSFrameQueue();
      return;
    }

    if (_conversationState != ConversationState::Speaking) {
      setConversationState(ConversationState::Speaking, "tts stream playback started");
    }

    Serial.printf("[TTS][playback] request_id=%s codec=opus frame_index=%d decoded_bytes=%u buffered_after_dequeue_ms=%u frames_remaining=%u\n",
      _ttsStreamRequestId.c_str(),
      frame.chunkIndex,
      static_cast<unsigned>(decodedPcmLen),
      static_cast<unsigned>(_ttsBufferedMs),
      static_cast<unsigned>(_ttsFrameCount));
    return;
  }

  // ─────────────────────────────────────────────────────────────────────────
  // [消費フロー: dequeue → 再生]
  // dequeueTTSPlaybackBatch() でキューから 40ms ぶんのフレームを集約し、
  // 1 回の playPCM16() で再生開始します。
  // ─────────────────────────────────────────────────────────────────────────
  uint8_t* mergedBytes = nullptr;
  size_t mergedLen = 0;
  uint16_t mergedDurationMs = 0;
  if (!dequeueTTSPlaybackBatch(kTTSPlaybackBatchMs, &mergedBytes, &mergedLen, &mergedDurationMs)) {
    return;
  }

  const bool started = _ttsPlayer.playPCM16(mergedBytes, mergedLen, _ttsSampleRateHz, true);
  free(mergedBytes);

  if (!started) {
    Serial.printf("[TTS] request_id=%s playback batch start failed bytes=%u\n",
      _ttsStreamRequestId.c_str(),
      static_cast<unsigned>(mergedLen));
    clearTTSFrameQueue();
    return;
  }

  if (_conversationState != ConversationState::Speaking) {
    setConversationState(ConversationState::Speaking, "tts stream playback started");
  }

  // P8-17: キュー状態のスナップショットを出力（observability 強化）
  // これにより、受信速度と消費速度の関係を外部から監視可能にします。
  Serial.printf("[TTS][playback] request_id=%s batch_duration_ms=%u batch_bytes=%u buffered_after_dequeue_ms=%u frames_remaining=%u\n",
    _ttsStreamRequestId.c_str(),
    static_cast<unsigned>(mergedDurationMs),
    static_cast<unsigned>(mergedLen),
    static_cast<unsigned>(_ttsBufferedMs),
    static_cast<unsigned>(_ttsFrameCount));
}

}  // namespace App
