/**
 * @file session_audio_upload.cpp
 * @brief StackchanSession の audio uplink 実装
 */
#include "session.h"
#include <ArduinoJson.h>

namespace App {

void StackchanSession::sendAudioStream(int frameCount) {
  // Active 状態でない場合は送信をスキップします。
  if (_state != SessionState::Active) {
    Serial.printf("[AudioSend] Skipped: state=%d (not Active)\n", (int)_state);
    return;
  }

  // UUID v4 で stream_id を生成します。
  String streamId = Protocol::generateUUIDv4();
  setConversationState(ConversationState::Listening, "audio capture started");
  Serial.printf("[AudioSend] Start: stream_id=%s frames=%d\n",
    streamId.c_str(), frameCount);

  // Step 1: audio.stream_open を送信します。
  JsonDocument openPayload;
  openPayload["stream_id"]         = streamId;
  openPayload["codec"]             = "pcm";
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

  // Step 2: バイナリフレームを送信します。
  // フレームフォーマット: [stream_id(36 bytes ASCII)][PCM data(N bytes)]
  const size_t frameBytes = _mic.frameSizeBytes();
  const size_t totalBytes = 36 + frameBytes;

  // スタック上にバッファを確保します（最大 676 bytes = 36 + 16000*20/1000*2）。
  uint8_t frameBuf[36 + 640];
  if (totalBytes > sizeof(frameBuf)) {
    Serial.printf("[AudioSend] Frame too large: %zu > %zu\n", totalBytes, sizeof(frameBuf));
    return;
  }

  // 先頭 36 バイトに stream_id ASCII 文字列をコピーします（NULL 終端なし）。
  memcpy(frameBuf, streamId.c_str(), 36);

  for (int i = 0; i < frameCount; i++) {
    _mic.readFrame(frameBuf + 36, frameBytes);

    if (!_ws.sendBinary(frameBuf, totalBytes)) {
      Serial.printf("[AudioSend] Binary frame %d send failed\n", i + 1);
      break;
    }

    if (i == 0 || i == frameCount - 1) {
      Serial.printf("[AudioSend] Frame %d/%d sent (%zu bytes)\n",
        i + 1, frameCount, totalBytes);
    }

    // NOTE: 本 delay はブロッキングです。Phase 6 でタスクベースに置き換えます。
    delay(FW_AUDIO_FRAME_MS);
  }

  // Step 3: audio.end を送信します。
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

}  // namespace App
