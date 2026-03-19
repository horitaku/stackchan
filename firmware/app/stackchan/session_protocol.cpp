/**
 * @file session_protocol.cpp
 * @brief StackchanSession の protocol 送受信実装
 */
#include "session.h"
#include <ArduinoJson.h>

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

// P8-19: TTS バッファ watermark 状態を server へ送信します。
// 状態変化時のみ送信し、kWatermarkCooldownMs 内の連続送信は抑制します。
void StackchanSession::sendTTSBufferWatermark(
    const String& status, uint32_t bufferedMs, uint32_t thresholdMs, uint32_t framesInQueue) {
  const unsigned long now = millis();
  if (status == _ttsWatermarkStatus && (now - _ttsWatermarkLastSentMs) < kWatermarkCooldownMs) {
    return;
  }
  _ttsWatermarkStatus = status;
  _ttsWatermarkLastSentMs = now;

  JsonDocument payload;
  payload["request_id"]      = _ttsStreamRequestId;
  payload["stream_id"]       = _ttsStreamId;
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
