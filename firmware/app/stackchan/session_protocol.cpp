/**
 * @file session_protocol.cpp
 * @brief StackchanSession の protocol 送受信実装
 */
#include "session.h"
#include <ArduinoJson.h>
#include <esp_heap_caps.h>

namespace App {

// ──────────────────────────────────────────────────────────────────────
// Private: 送信ヘルパー
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::sendHello() {
  JsonDocument payload;
  payload["device_id"]   = FW_DEVICE_ID;
  payload["client_type"] = "firmware";
  // 音声チャンク / audio.end に対応していることを宣言します。
  JsonObject caps = payload["protocol_capabilities"].to<JsonObject>();
  caps["audio_chunk"] = true;
  caps["audio_end"]   = true;

  String payloadStr;
  serializeJson(payload, payloadStr);

  // session.hello では session_id は空文字列を使います。
  String env = Protocol::buildEnvelope(
    Protocol::EventType::SESSION_HELLO, "", _seq.next(), payloadStr);

  _ws.sendText(env);
  Serial.printf("[Session] session.hello sent (device_id=%s)\n", FW_DEVICE_ID);
}

void StackchanSession::sendHeartbeat() {
  JsonDocument payload;
  payload["uptime_ms"] = millis();
  payload["rssi"]      = Network::getRSSI();

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

void StackchanSession::sendDeviceStateReport(const String& requestId, const String& source) {
  JsonDocument payload;
  payload["request_id"] = requestId;
  payload["source"] = source;
  payload["uptime_ms"] = millis();
  payload["rssi"] = Network::getRSSI();
  payload["free_heap_bytes"] = heap_caps_get_free_size(MALLOC_CAP_8BIT);

  payload["current_angle_x_deg"] = _servo.currentAngleXDeg();
  payload["current_angle_y_deg"] = _servo.currentAngleYDeg();

  const auto& cx = _servo.calibrationX();
  const auto& cy = _servo.calibrationY();

  JsonObject calib = payload["calibration"].to<JsonObject>();
  JsonObject x = calib["x"].to<JsonObject>();
  x["center_offset_deg"] = cx.center_offset_deg;
  x["min_deg"] = cx.min_deg;
  x["max_deg"] = cx.max_deg;
  x["invert"] = cx.invert;
  x["speed_limit_deg_per_sec"] = cx.speed_limit_deg_per_sec;
  x["soft_start"] = cx.soft_start;
  x["home_deg"] = cx.home_deg;

  JsonObject y = calib["y"].to<JsonObject>();
  y["center_offset_deg"] = cy.center_offset_deg;
  y["min_deg"] = cy.min_deg;
  y["max_deg"] = cy.max_deg;
  y["invert"] = cy.invert;
  y["speed_limit_deg_per_sec"] = cy.speed_limit_deg_per_sec;
  y["soft_start"] = cy.soft_start;
  y["home_deg"] = cy.home_deg;

  // Phase 11 時点では mic level の実計測配線が未導入のため 0.0 を返します。
  payload["mic_level"] = 0.0f;
  payload["speaker_busy"] = (_ttsPlayer.state() != Audio::PlaybackState::Idle);
  payload["camera_available"] = _camera.available();

#ifdef FW_DEVICE_ID
  payload["firmware_version"] = FW_DEVICE_ID;
#else
  payload["firmware_version"] = "stackchan-firmware";
#endif

  String payloadStr;
  serializeJson(payload, payloadStr);

  String env = Protocol::buildEnvelope(
    Protocol::EventType::DEVICE_STATE_REPORT, _sessionId, _seq.next(), payloadStr);

  if (_ws.sendText(env)) {
    Serial.printf("[StateReport] sent request_id=%s source=%s heap=%u rssi=%d\n",
      requestId.c_str(), source.c_str(),
      static_cast<unsigned>(heap_caps_get_free_size(MALLOC_CAP_8BIT)), Network::getRSSI());
  } else {
    Serial.printf("[StateReport] send failed request_id=%s source=%s\n",
      requestId.c_str(), source.c_str());
  }
}

void StackchanSession::handleDeviceStateReport(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* source = payload["source"] | "server.request";

  if (strlen(requestId) == 0) {
    sendDeviceStateReport(String("state-") + String(millis()), String(source));
    return;
  }

  sendDeviceStateReport(String(requestId), String(source));
}

// P8-19: TTS バッファ watermark 状態を server へ送信します。
// 状態変化時のみ送信し、TTSStreamContext::kWatermarkCooldownMs 内の連続送信は抑制します。
void StackchanSession::sendTTSBufferWatermark(
    const String& status, uint32_t bufferedMs, uint32_t thresholdMs, uint32_t framesInQueue) {
  const unsigned long now = millis();
  if (status == _tts.watermarkStatus && (now - _tts.watermarkLastSentMs) < TTSStreamContext::kWatermarkCooldownMs) {
    return;
  }
  _tts.watermarkStatus = status;
  _tts.watermarkLastSentMs = now;

  JsonDocument payload;
  payload["request_id"]      = _tts.streamRequestId;
  payload["stream_id"]       = _tts.streamId;
  payload["status"]          = status;
  payload["buffered_ms"]     = bufferedMs;
  payload["threshold_ms"]    = thresholdMs;
  payload["frames_in_queue"] = framesInQueue;

  String payloadStr;
  serializeJson(payload, payloadStr);

  String env = Protocol::buildEnvelope(
    Protocol::EventType::TTS_BUFFER_WATERMARK, _sessionId, _seq.next(), payloadStr);

  if (!_ws.sendText(env)) {
    Serial.printf("[TTS][watermark] failed to send %s event\n", status.c_str());
  } else {
    Serial.printf("[TTS][watermark] sent status=%s buffered_ms=%u threshold_ms=%u frames=%u\n",
      status.c_str(), static_cast<unsigned>(bufferedMs),
      static_cast<unsigned>(thresholdMs), static_cast<unsigned>(framesInQueue));
  }
}

// ──────────────────────────────────────────────────────────────────────
// Private: 受信ディスパッチ
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::onTextMessage(const String& msg) {
  using PayloadHandler = void (StackchanSession::*)(const String&);
  using EnvelopeHandler = void (StackchanSession::*)(const String&, const String&);

  struct PayloadRoute {
    const char* type;
    PayloadHandler handler;
  };

  struct EnvelopeRoute {
    const char* type;
    EnvelopeHandler handler;
  };

  static const EnvelopeRoute envelopeRoutes[] = {
    {Protocol::EventType::SESSION_WELCOME, &StackchanSession::handleWelcome},
  };

  static const PayloadRoute payloadRoutes[] = {
    {Protocol::EventType::STT_FINAL, &StackchanSession::handleSTTFinal},
    {Protocol::EventType::TTS_CHUNK, &StackchanSession::handleTTSChunk},
    {Protocol::EventType::TTS_END, &StackchanSession::handleTTSEnd},
    {Protocol::EventType::AVATAR_EXPRESSION, &StackchanSession::handleAvatarExpression},
    {Protocol::EventType::MOTION_PLAY, &StackchanSession::handleMotionPlay},
    {Protocol::EventType::CONVERSATION_CANCEL, &StackchanSession::handleConversationCancel},
    {Protocol::EventType::TTS_STOP, &StackchanSession::handleTTSStop},
    {Protocol::EventType::AUDIO_STREAM_ABORT, &StackchanSession::handleAudioStreamAbort},
    {Protocol::EventType::ERROR_EVENT, &StackchanSession::handleError},
    // P11-08: device.servo.* イベントを ServoController に委譲します
    {Protocol::EventType::DEVICE_SERVO_MOVE,            &StackchanSession::handleDeviceServoMove},
    {Protocol::EventType::DEVICE_SERVO_CALIBRATION_GET, &StackchanSession::handleDeviceServoCalibrationGet},
    {Protocol::EventType::DEVICE_SERVO_CALIBRATION_SET, &StackchanSession::handleDeviceServoCalibrationSet},
    // P11-03: device.led.* / device.ears.* を Lighting コントローラーへ委譲します
    {Protocol::EventType::DEVICE_LED_SET,               &StackchanSession::handleDeviceLedSet},
    {Protocol::EventType::DEVICE_EARS_SET,              &StackchanSession::handleDeviceEarsSet},
    {Protocol::EventType::DEVICE_CAMERA_CAPTURE,        &StackchanSession::handleDeviceCameraCapture},
    // P11-10: server からの state.report 要求を処理し、診断状態を返送します
    {Protocol::EventType::DEVICE_STATE_REPORT,          &StackchanSession::handleDeviceStateReport},
  };

  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, msg);
  if (err) {
    Serial.printf("[Session] JSON parse error: %s (len=%d)\n", err.c_str(), msg.length());
    return;
  }

  const char* type         = doc["type"] | "";
  const char* envSessionId = doc["session_id"] | "";

  String payloadStr;
  serializeJson(doc["payload"], payloadStr);

  for (const EnvelopeRoute& route : envelopeRoutes) {
    if (strcmp(type, route.type) == 0) {
      (this->*route.handler)(payloadStr, String(envSessionId));
      return;
    }
  }

  for (const PayloadRoute& route : payloadRoutes) {
    if (strcmp(type, route.type) == 0) {
      (this->*route.handler)(payloadStr);
      return;
    }
  }

  Serial.printf("[Session] Unhandled event type: %s\n", type);
}

// ──────────────────────────────────────────────────────────────────────
// Private: 受信イベントハンドラ
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::handleWelcome(const String& payloadJson,
                                     const String& envelopeSessionId) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const bool accepted = payload["accepted"] | false;
  if (!accepted) {
    Serial.println("[Session] welcome: accepted=false -> closing session");
    setState(SessionState::Error);
    return;
  }

  if (envelopeSessionId.length() > 0) {
    _sessionId = envelopeSessionId;
  }

  _heartbeatIntervalMs = payload["heartbeat_interval_ms"] | FW_HEARTBEAT_INTERVAL_MS;

  setState(SessionState::Active);
  setConversationState(ConversationState::Idle, "session welcome accepted");
  _lastHeartbeatMs = millis();
  if (_avatarReady) {
    _avatar.setSpeechText("Ready");
  }

  Serial.printf("[Session] welcome accepted -> Active\n");
  Serial.printf("[Session] session_id=%s heartbeat_interval_ms=%lu\n",
    _sessionId.c_str(), _heartbeatIntervalMs);
}

void StackchanSession::handleSTTFinal(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId  = payload["request_id"] | "";
  const char* transcript = payload["transcript"] | "";
  const float confidence = payload["confidence"] | -1.0f;

  if (confidence >= 0.0f) {
    Serial.printf("[STT] request_id=%s transcript=\"%s\" confidence=%.2f\n",
      requestId, transcript, confidence);
  } else {
    Serial.printf("[STT] request_id=%s transcript=\"%s\"\n",
      requestId, transcript);
  }
  setConversationState(ConversationState::Thinking, "stt.final received");
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

  const char* code = payload["code"] | "";
  const char* message = payload["message"] | "";
  const bool retry = payload["retryable"] | false;

  Serial.printf("[Session] ERROR code=%s message=%s retryable=%d\n",
    code, message, static_cast<int>(retry));

  if (!retry) {
    setConversationState(ConversationState::Error, "non-retryable error event");
  }
}

}  // namespace App
