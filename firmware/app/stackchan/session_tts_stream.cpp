/**
 * @file session_tts_stream.cpp
 * @brief StackchanSession の TTS ストリーム処理実装
 */
#include "session.h"
#include <ArduinoJson.h>
#include <M5Unified.h>
#include <mbedtls/base64.h>
extern "C" {
#include <opus.h>
}

namespace App {

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
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(_tts.frameCount));
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
    static_cast<unsigned>(_tts.incomingBufferLen));
}

void StackchanSession::handleTTSEnd(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* codec = payload["codec"] | "";
  int durationMs = payload["duration_ms"] | 0;
  int sampleRateHz = payload["sample_rate_hz"] | 0;
  const char* audioBase64 = payload["audio_base64"] | "";
  int totalChunks = payload["total_chunks"] | 0;

  _currentRequestId = String(requestId);

  // P8-15: フレームキュー方式（v1.1）では tts.end をストリーム終端として扱います。
  if (_tts.streamRequestId == String(requestId)) {
    if (String(codec).length() > 0) {
      _tts.streamCodec = String(codec);
    }
    if (sampleRateHz > 0) {
      _tts.sampleRateHz = static_cast<uint32_t>(sampleRateHz);
    }
    _tts.streamEnded = true;
    Serial.printf("[TTS] request_id=%s stream playback pending codec=%s buffered_ms=%u frames=%u sample_rate_hz=%u\n",
      requestId,
      _tts.streamCodec.c_str(),
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(_tts.frameCount),
      static_cast<unsigned>(_tts.sampleRateHz));
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

  if (_tts.incomingRequestId == String(requestId) && _tts.incomingBuffer != nullptr) {
    if (totalChunks > 0 && _tts.incomingReceivedChunks != totalChunks) {
      Serial.printf("[TTS] request_id=%s chunk count mismatch received=%d expected=%d\n",
        requestId,
        _tts.incomingReceivedChunks,
        totalChunks);
      clearIncomingTTSBuffer();
      return;
    }
    playbackBytes = _tts.incomingBuffer;
    playbackLen = _tts.incomingBufferLen;
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

  if (playbackBytes == _tts.incomingBuffer) {
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
  if (_tts.incomingBuffer != nullptr) {
    free(_tts.incomingBuffer);
    _tts.incomingBuffer = nullptr;
  }
  _tts.incomingBufferLen = 0;
  _tts.incomingExpectedChunks = 0;
  _tts.incomingReceivedChunks = 0;
  _tts.incomingRequestId = "";
}

bool StackchanSession::appendIncomingTTSChunk(const String& requestId, int chunkIndex, int totalChunks, const String& audioBase64) {
  if (requestId.length() == 0 || chunkIndex < 0 || totalChunks <= 0 || audioBase64.length() == 0) {
    return false;
  }

  if (_tts.incomingRequestId != requestId) {
    clearIncomingTTSBuffer();
    _tts.incomingRequestId = requestId;
    _tts.incomingExpectedChunks = totalChunks;
  }

  if (chunkIndex != _tts.incomingReceivedChunks) {
    Serial.printf("[TTS] request_id=%s unexpected chunk index=%d expected=%d\n",
      requestId.c_str(), chunkIndex, _tts.incomingReceivedChunks);
    clearIncomingTTSBuffer();
    return false;
  }

  uint8_t* decoded = nullptr;
  size_t decodedLen = 0;
  if (!decodeBase64(audioBase64, &decoded, &decodedLen)) {
    return false;
  }

  uint8_t* next = static_cast<uint8_t*>(realloc(_tts.incomingBuffer, _tts.incomingBufferLen + decodedLen));
  if (next == nullptr) {
    free(decoded);
    clearIncomingTTSBuffer();
    return false;
  }

  _tts.incomingBuffer = next;
  memcpy(_tts.incomingBuffer + _tts.incomingBufferLen, decoded, decodedLen);
  _tts.incomingBufferLen += decodedLen;
  _tts.incomingReceivedChunks++;
  _tts.incomingExpectedChunks = totalChunks;

  free(decoded);
  return true;
}

void StackchanSession::clearTTSFrameQueue() {
  for (size_t i = 0; i < TTSStreamContext::kFrameQueueCapacity; i++) {
    if (_tts.frameQueue[i].bytes != nullptr) {
      free(_tts.frameQueue[i].bytes);
      _tts.frameQueue[i].bytes = nullptr;
    }
    _tts.frameQueue[i].byteLen = 0;
    _tts.frameQueue[i].frameDurationMs = 0;
    _tts.frameQueue[i].samplesPerChunk = 0;
    _tts.frameQueue[i].chunkIndex = 0;
  }

  _tts.frameHead = 0;
  _tts.frameTail = 0;
  _tts.frameCount = 0;
  _tts.bufferedMs = 0;
  _tts.playbackPrimed = false;
  _tts.streamEnded = false;
  _tts.expectedChunkIndex = 0;
  _tts.streamRequestId = "";
  _tts.streamId = "";
  _tts.streamCodec = "pcm";
  resetOpusDecoder();

  // P8-16: concealment 状態をリセットします。
  if (_tts.lastGoodFrameBytes != nullptr) {
    free(_tts.lastGoodFrameBytes);
    _tts.lastGoodFrameBytes = nullptr;
  }
  _tts.lastGoodFrameLen = 0;
  _tts.missingChunkCount = 0;
  _tts.concealmentFrameCount = 0;
}

// P8-16: concealment（欠落補完）フレームをキューに挿入します。
void StackchanSession::insertConcealmentFrames(int gapCount, int frameDurationMs, int samplesPerChunk) {
  const int insertCount = min(gapCount, TTSStreamContext::kMaxConcealmentFrames);
  const size_t frameByteLen = static_cast<size_t>(samplesPerChunk) * 2;

  for (int i = 0; i < insertCount; i++) {
    if (_tts.frameCount >= TTSStreamContext::kFrameQueueCapacity) {
      Serial.printf("[TTS][concealment] queue full, cannot insert frame %d/%d\n", i + 1, insertCount);
      break;
    }

    const uint32_t nextBufferedMs = _tts.bufferedMs + static_cast<uint32_t>(frameDurationMs);
    if (nextBufferedMs > TTSStreamContext::kHighWaterMs) {
      Serial.printf("[TTS][concealment] high-water reached at frame %d/%d, stopping insertion\n",
        i + 1, insertCount);
      break;
    }

    uint8_t* concealFrame = static_cast<uint8_t*>(malloc(frameByteLen));
    if (concealFrame == nullptr) {
      Serial.printf("[TTS][concealment] malloc failed for frame %d/%d\n", i + 1, insertCount);
      break;
    }

    if (_tts.lastGoodFrameBytes != nullptr && _tts.lastGoodFrameLen == frameByteLen) {
      const int16_t* src = reinterpret_cast<const int16_t*>(_tts.lastGoodFrameBytes);
      int16_t* dst = reinterpret_cast<int16_t*>(concealFrame);
      const size_t sampleCount = frameByteLen / 2;
      const int shiftBits = i + 1;
      for (size_t s = 0; s < sampleCount; s++) {
        dst[s] = static_cast<int16_t>(src[s] >> shiftBits);
      }
    } else {
      memset(concealFrame, 0, frameByteLen);
    }

    TTSStreamContext::FrameSlot& slot = _tts.frameQueue[_tts.frameTail];
    slot.bytes = concealFrame;
    slot.byteLen = frameByteLen;
    slot.frameDurationMs = static_cast<uint16_t>(frameDurationMs);
    slot.samplesPerChunk = static_cast<uint16_t>(samplesPerChunk);
    slot.chunkIndex = _tts.expectedChunkIndex + i;

    _tts.frameTail = (_tts.frameTail + 1) % TTSStreamContext::kFrameQueueCapacity;
    _tts.frameCount++;
    _tts.bufferedMs += static_cast<uint32_t>(frameDurationMs);
    _tts.concealmentFrameCount++;
  }

  _tts.expectedChunkIndex += gapCount;

  Serial.printf("[TTS][concealment] request_id=%s stream_id=%s gap=%d inserted=%d total_missing=%d total_conc=%d\n",
    _tts.streamRequestId.c_str(),
    _tts.streamId.c_str(),
    gapCount,
    insertCount,
    _tts.missingChunkCount,
    _tts.concealmentFrameCount);
}

bool StackchanSession::enqueueTTSFrame(const String& requestId,
                                       const String& streamId,
                                       int chunkIndex,
                                       int frameDurationMs,
                                       int samplesPerChunk,
                                       const String& audioBase64,
                                       const String& codec) {
  if (requestId.length() == 0 ||
      streamId.length() == 0 ||
      chunkIndex < 0 ||
      frameDurationMs <= 0 ||
      samplesPerChunk <= 0 ||
      audioBase64.length() == 0) {
    return false;
  }

  if (_tts.streamRequestId != requestId || _tts.streamId != streamId) {
    if (_ttsPlayer.state() == Audio::PlaybackState::Playing ||
        _ttsPlayer.state() == Audio::PlaybackState::Buffering) {
      _ttsPlayer.stop();
    }
    clearTTSFrameQueue();
    _tts.streamRequestId = requestId;
    _tts.streamId = streamId;
    _tts.streamCodec = codec.length() > 0 ? codec : "pcm";
    _tts.expectedChunkIndex = 0;
  }

  if (codec.length() > 0 && _tts.streamCodec != codec) {
    Serial.printf("[TTS] request_id=%s stream_id=%s codec changed %s -> %s (ignored)\n",
      requestId.c_str(),
      streamId.c_str(),
      _tts.streamCodec.c_str(),
      codec.c_str());
  }

  if (chunkIndex != _tts.expectedChunkIndex) {
    if (chunkIndex < _tts.expectedChunkIndex) {
      Serial.printf("[TTS] request_id=%s stream_id=%s duplicate frame idx=%d expected=%d (skipped)\n",
        requestId.c_str(), streamId.c_str(), chunkIndex, _tts.expectedChunkIndex);
      return true;
    }

    const int gapCount = chunkIndex - _tts.expectedChunkIndex;
    _tts.missingChunkCount += gapCount;

    Serial.printf("[TTS] request_id=%s stream_id=%s gap detected missing=%d idx=%d expected=%d\n",
      requestId.c_str(), streamId.c_str(), gapCount, chunkIndex, _tts.expectedChunkIndex);

    if (_tts.streamCodec == "pcm") {
      insertConcealmentFrames(gapCount, frameDurationMs, samplesPerChunk);
    } else {
      _tts.expectedChunkIndex += gapCount;
    }
  }

  uint8_t* decoded = nullptr;
  size_t decodedLen = 0;
  if (!decodeBase64(audioBase64, &decoded, &decodedLen)) {
    return false;
  }

  if (_tts.frameCount >= TTSStreamContext::kFrameQueueCapacity) {
    Serial.printf("[TTS] request_id=%s stream_id=%s frame queue overflow (capacity=%u)\n",
      requestId.c_str(),
      streamId.c_str(),
      static_cast<unsigned>(TTSStreamContext::kFrameQueueCapacity));
    free(decoded);
    _tts.expectedChunkIndex++;
    return true;
  }

  const uint32_t nextBufferedMs = _tts.bufferedMs + static_cast<uint32_t>(frameDurationMs);
  if (nextBufferedMs > TTSStreamContext::kHighWaterMs) {
    Serial.printf("[TTS] request_id=%s stream_id=%s high-water drop idx=%d buffered_ms=%u limit_ms=%u\n",
      requestId.c_str(),
      streamId.c_str(),
      chunkIndex,
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(TTSStreamContext::kHighWaterMs));
    sendTTSBufferWatermark("high_water", _tts.bufferedMs, TTSStreamContext::kHighWaterMs, static_cast<uint32_t>(_tts.frameCount));
    free(decoded);
    _tts.expectedChunkIndex++;
    return true;
  }

  if (chunkIndex == 0 && samplesPerChunk > 0 && frameDurationMs > 0) {
    const uint32_t inferredHz =
        static_cast<uint32_t>(samplesPerChunk) * 1000u /
        static_cast<uint32_t>(frameDurationMs);
    if (inferredHz > 0 && inferredHz != _tts.sampleRateHz) {
      Serial.printf("[TTS] sample_rate_hz inferred from first chunk: %u -> %u\n",
                    static_cast<unsigned>(_tts.sampleRateHz),
                    static_cast<unsigned>(inferredHz));
      _tts.sampleRateHz = inferredHz;
    }
  }

  TTSStreamContext::FrameSlot& slot = _tts.frameQueue[_tts.frameTail];
  slot.bytes = decoded;
  slot.byteLen = decodedLen;
  slot.frameDurationMs = static_cast<uint16_t>(frameDurationMs);
  slot.samplesPerChunk = static_cast<uint16_t>(samplesPerChunk);
  slot.chunkIndex = chunkIndex;

  _tts.frameTail = (_tts.frameTail + 1) % TTSStreamContext::kFrameQueueCapacity;
  _tts.frameCount++;
  _tts.bufferedMs += static_cast<uint32_t>(frameDurationMs);
  _tts.expectedChunkIndex++;

  if (_tts.streamCodec == "pcm") {
    if (_tts.lastGoodFrameBytes != nullptr) {
      free(_tts.lastGoodFrameBytes);
      _tts.lastGoodFrameBytes = nullptr;
    }
    _tts.lastGoodFrameBytes = static_cast<uint8_t*>(malloc(decodedLen));
    if (_tts.lastGoodFrameBytes != nullptr) {
      memcpy(_tts.lastGoodFrameBytes, decoded, decodedLen);
      _tts.lastGoodFrameLen = decodedLen;
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

  if (_tts.frameCount == 0) {
    return false;
  }

  const uint16_t durationLimit = targetDurationMs > 0 ? targetDurationMs : TTSStreamContext::kPlaybackBatchMs;

  size_t totalBytes = 0;
  uint16_t totalDuration = 0;
  size_t framesToPop = 0;
  size_t cursor = _tts.frameHead;
  size_t available = _tts.frameCount;

  while (available > 0) {
    const TTSStreamContext::FrameSlot& slot = _tts.frameQueue[cursor];
    totalBytes += slot.byteLen;
    totalDuration = static_cast<uint16_t>(totalDuration + slot.frameDurationMs);
    framesToPop++;

    cursor = (cursor + 1) % TTSStreamContext::kFrameQueueCapacity;
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
    TTSStreamContext::FrameSlot& slot = _tts.frameQueue[_tts.frameHead];
    memcpy(merged + offset, slot.bytes, slot.byteLen);
    offset += slot.byteLen;

    if (_tts.bufferedMs >= slot.frameDurationMs) {
      _tts.bufferedMs -= slot.frameDurationMs;
    } else {
      _tts.bufferedMs = 0;
    }

    if (slot.bytes != nullptr) {
      free(slot.bytes);
      slot.bytes = nullptr;
    }
    slot.byteLen = 0;
    slot.frameDurationMs = 0;
    slot.samplesPerChunk = 0;
    slot.chunkIndex = 0;

    _tts.frameHead = (_tts.frameHead + 1) % TTSStreamContext::kFrameQueueCapacity;
    _tts.frameCount--;
  }

  *outBytes = merged;
  *outByteLen = totalBytes;
  *outDurationMs = totalDuration;
  return true;
}

bool StackchanSession::dequeueTTSFrame(TTSStreamContext::FrameSlot* outFrame) {
  if (outFrame == nullptr || _tts.frameCount == 0) {
    return false;
  }

  TTSStreamContext::FrameSlot& slot = _tts.frameQueue[_tts.frameHead];
  *outFrame = slot;

  if (_tts.bufferedMs >= slot.frameDurationMs) {
    _tts.bufferedMs -= slot.frameDurationMs;
  } else {
    _tts.bufferedMs = 0;
  }

  slot.bytes = nullptr;
  slot.byteLen = 0;
  slot.frameDurationMs = 0;
  slot.samplesPerChunk = 0;
  slot.chunkIndex = 0;

  _tts.frameHead = (_tts.frameHead + 1) % TTSStreamContext::kFrameQueueCapacity;
  _tts.frameCount--;
  return true;
}

void StackchanSession::resetOpusDecoder() {
  if (_tts.opusDecoder != nullptr) {
    opus_decoder_destroy(static_cast<OpusDecoder*>(_tts.opusDecoder));
    _tts.opusDecoder = nullptr;
  }
  _tts.opusDecoderSampleRateHz = 0;
}

bool StackchanSession::ensureOpusDecoder(uint32_t sampleRateHz) {
  if (sampleRateHz != 8000 && sampleRateHz != 12000 && sampleRateHz != 16000 && sampleRateHz != 24000 && sampleRateHz != 48000) {
    Serial.printf("[TTS][opus] unsupported decoder sample_rate_hz=%u\n", static_cast<unsigned>(sampleRateHz));
    return false;
  }

  if (_tts.opusDecoder != nullptr && _tts.opusDecoderSampleRateHz == sampleRateHz) {
    return true;
  }

  resetOpusDecoder();

  int err = OPUS_OK;
  OpusDecoder* decoder = opus_decoder_create(static_cast<opus_int32>(sampleRateHz), 1, &err);
  if (decoder == nullptr || err != OPUS_OK) {
    Serial.printf("[TTS][opus] decoder create failed err=%d\n", err);
    return false;
  }

  _tts.opusDecoder = decoder;
  _tts.opusDecoderSampleRateHz = sampleRateHz;
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
    static_cast<OpusDecoder*>(_tts.opusDecoder),
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
  if (_tts.frameCount == 0) {
    if (_tts.streamEnded && _ttsPlayer.state() == Audio::PlaybackState::Idle) {
      if (_tts.missingChunkCount > 0 || _tts.concealmentFrameCount > 0) {
        Serial.printf("[TTS][metrics] stream_id=%s request_id=%s missing_chunks=%d concealment_frames=%d\n",
          _tts.streamId.c_str(),
          _tts.streamRequestId.c_str(),
          _tts.missingChunkCount,
          _tts.concealmentFrameCount);
      }
      clearTTSFrameQueue();
      if (_conversationState == ConversationState::Speaking) {
        setConversationState(ConversationState::Idle, "tts stream drained");
      }
    }
    return;
  }

  if (_ttsPlayer.state() != Audio::PlaybackState::Idle) {
    return;
  }

  if (!_tts.playbackPrimed) {
    if (_tts.bufferedMs < TTSStreamContext::kPrebufferStartMs && !_tts.streamEnded) {
      return;
    }
    _tts.playbackPrimed = true;
    Serial.printf("[TTS] prebuffer ready request_id=%s buffered_ms=%u threshold_ms=%u\n",
      _tts.streamRequestId.c_str(),
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(TTSStreamContext::kPrebufferStartMs));
  }

  if (_tts.bufferedMs < TTSStreamContext::kLowWaterMs && !_tts.streamEnded) {
    Serial.printf("[TTS][watermark] low-water request_id=%s buffered_ms=%u threshold_ms=%u frames_in_queue=%u\n",
      _tts.streamRequestId.c_str(),
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(TTSStreamContext::kLowWaterMs),
      static_cast<unsigned>(_tts.frameCount));
    sendTTSBufferWatermark("low_water", _tts.bufferedMs, TTSStreamContext::kLowWaterMs, static_cast<uint32_t>(_tts.frameCount));
  } else if (_tts.watermarkStatus != "normal") {
    sendTTSBufferWatermark("normal", _tts.bufferedMs, TTSStreamContext::kLowWaterMs, static_cast<uint32_t>(_tts.frameCount));
  }

  if (_tts.streamCodec == "opus") {
    TTSStreamContext::FrameSlot frame;
    if (!dequeueTTSFrame(&frame)) {
      return;
    }

    uint8_t* decodedPcm = nullptr;
    size_t decodedPcmLen = 0;
    if (!decodeOpusFrame(frame.bytes, frame.byteLen, _tts.sampleRateHz, &decodedPcm, &decodedPcmLen)) {
      Serial.printf("[TTS][opus] request_id=%s decode failed idx=%d\n", _tts.streamRequestId.c_str(), frame.chunkIndex);
      if (frame.bytes != nullptr) {
        free(frame.bytes);
      }
      clearTTSFrameQueue();
      return;
    }

    if (frame.bytes != nullptr) {
      free(frame.bytes);
    }

    const bool started = _ttsPlayer.playPCM16(decodedPcm, decodedPcmLen, _tts.sampleRateHz, true);
    free(decodedPcm);

    if (!started) {
      Serial.printf("[TTS][opus] request_id=%s playback start failed decoded_bytes=%u\n",
        _tts.streamRequestId.c_str(),
        static_cast<unsigned>(decodedPcmLen));
      clearTTSFrameQueue();
      return;
    }

    if (_conversationState != ConversationState::Speaking) {
      setConversationState(ConversationState::Speaking, "tts stream playback started");
    }

    Serial.printf("[TTS][playback] request_id=%s codec=opus frame_index=%d decoded_bytes=%u buffered_after_dequeue_ms=%u frames_remaining=%u\n",
      _tts.streamRequestId.c_str(),
      frame.chunkIndex,
      static_cast<unsigned>(decodedPcmLen),
      static_cast<unsigned>(_tts.bufferedMs),
      static_cast<unsigned>(_tts.frameCount));
    return;
  }

  uint8_t* mergedBytes = nullptr;
  size_t mergedLen = 0;
  uint16_t mergedDurationMs = 0;
  if (!dequeueTTSPlaybackBatch(TTSStreamContext::kPlaybackBatchMs, &mergedBytes, &mergedLen, &mergedDurationMs)) {
    return;
  }

  const bool started = _ttsPlayer.playPCM16(mergedBytes, mergedLen, _tts.sampleRateHz, true);
  free(mergedBytes);

  if (!started) {
    Serial.printf("[TTS] request_id=%s playback batch start failed bytes=%u\n",
      _tts.streamRequestId.c_str(),
      static_cast<unsigned>(mergedLen));
    clearTTSFrameQueue();
    return;
  }

  if (_conversationState != ConversationState::Speaking) {
    setConversationState(ConversationState::Speaking, "tts stream playback started");
  }

  Serial.printf("[TTS][playback] request_id=%s batch_duration_ms=%u batch_bytes=%u buffered_after_dequeue_ms=%u frames_remaining=%u\n",
    _tts.streamRequestId.c_str(),
    static_cast<unsigned>(mergedDurationMs),
    static_cast<unsigned>(mergedLen),
    static_cast<unsigned>(_tts.bufferedMs),
    static_cast<unsigned>(_tts.frameCount));
}

}  // namespace App
