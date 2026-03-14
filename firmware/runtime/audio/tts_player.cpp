/**
 * @file tts_player.cpp
 * @brief TTS 再生制御モジュール実装
 */
#include "tts_player.h"

#include <M5Unified.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

namespace Audio {

void TTSPlayer::begin() {
  if (M5.Speaker.isEnabled()) {
    M5.Speaker.setVolume(180);
    Serial.println("[TTSPlayer] speaker initialized");
  } else {
    Serial.println("[TTSPlayer] speaker is not enabled");
  }
}

bool TTSPlayer::playPCM16(const uint8_t* pcmBytes, size_t byteLen, uint32_t sampleRateHz, bool stopCurrent) {
  if (pcmBytes == nullptr || byteLen < 2 || (byteLen % 2) != 0 || sampleRateHz == 0) {
    _state = PlaybackState::Error;
    Serial.printf("[TTSPlayer] invalid PCM input (len=%u, sample_rate=%u)\n",
                  static_cast<unsigned>(byteLen),
                  static_cast<unsigned>(sampleRateHz));
    return false;
  }

  if (stopCurrent && M5.Speaker.isPlaying()) {
    _state = PlaybackState::Stopping;
    M5.Speaker.stop();
  }

  clearBuffer();

  _state = PlaybackState::Buffering;
  _requestedAtMs = millis();
  _sampleRateHz = sampleRateHz;
  _pcmByteLen = byteLen;

  _pcmBuffer = static_cast<uint8_t*>(malloc(byteLen));
  if (_pcmBuffer == nullptr) {
    _state = PlaybackState::Error;
    Serial.printf("[TTSPlayer] failed to allocate pcm buffer (len=%u)\n", static_cast<unsigned>(byteLen));
    return false;
  }
  memcpy(_pcmBuffer, pcmBytes, byteLen);

  const size_t sampleCount = byteLen / sizeof(int16_t);
  _estimatedDurationMs = static_cast<uint32_t>((sampleCount * 1000UL) / sampleRateHz);
  if (_estimatedDurationMs == 0) {
    _estimatedDurationMs = 1;
  }

  _baseLipLevel = estimateBaseLipLevel(reinterpret_cast<const int16_t*>(_pcmBuffer), sampleCount);

  const bool started = M5.Speaker.playRaw(
      reinterpret_cast<const int16_t*>(_pcmBuffer),
      sampleCount,
      sampleRateHz,
      false,
      1,
      0,
      true);

  if (!started) {
    _state = PlaybackState::Error;
    clearBuffer();
    Serial.println("[TTSPlayer] playRaw failed");
    return false;
  }

  _startedAtMs = millis();
  _startLatencyMs = _startedAtMs - _requestedAtMs;
  _state = PlaybackState::Playing;
  _lipLevel = _baseLipLevel;

  Serial.printf("[TTSPlayer] playback started sample_rate=%u duration_est_ms=%u start_latency_ms=%u\n",
                static_cast<unsigned>(_sampleRateHz),
                static_cast<unsigned>(_estimatedDurationMs),
                static_cast<unsigned>(_startLatencyMs));
  return true;
}

void TTSPlayer::update() {
  if (_state != PlaybackState::Playing) {
    return;
  }

  const uint32_t elapsedMs = millis() - _startedAtMs;

  // 振幅ベース値に周期変調を加え、口開閉に見えるようにします。
  const float phase = static_cast<float>(elapsedMs % 120) / 120.0f;
  const float modulation = 0.55f + 0.45f * sinf(phase * 6.2831853f);
  _lipLevel = _baseLipLevel * modulation;
  if (_lipLevel > 1.0f) {
    _lipLevel = 1.0f;
  }

  if (!M5.Speaker.isPlaying()) {
    _lastPlaybackDurationMs = elapsedMs;
    _state = PlaybackState::Idle;
    _lipLevel = 0.0f;
    clearBuffer();
    Serial.printf("[TTSPlayer] playback finished duration_ms=%u\n",
                  static_cast<unsigned>(_lastPlaybackDurationMs));
  }
}

void TTSPlayer::stop() {
  if (_state == PlaybackState::Playing || _state == PlaybackState::Buffering) {
    _state = PlaybackState::Stopping;
    M5.Speaker.stop();
  }
  _state = PlaybackState::Idle;
  _lipLevel = 0.0f;
  clearBuffer();
}

void TTSPlayer::clearBuffer() {
  if (_pcmBuffer != nullptr) {
    free(_pcmBuffer);
    _pcmBuffer = nullptr;
  }
  _pcmByteLen = 0;
}

float TTSPlayer::estimateBaseLipLevel(const int16_t* samples, size_t sampleCount) const {
  if (samples == nullptr || sampleCount == 0) {
    return 0.0f;
  }

  const size_t scanCount = sampleCount > 1024 ? 1024 : sampleCount;
  uint64_t acc = 0;
  for (size_t i = 0; i < scanCount; i++) {
    acc += static_cast<uint64_t>(abs(samples[i]));
  }

  const float avg = static_cast<float>(acc) / static_cast<float>(scanCount);
  // 16-bit 振幅（0〜32767）を 0.2〜1.0 の範囲へ正規化します。
  float normalized = avg / 32767.0f;
  if (normalized < 0.2f) {
    normalized = 0.2f;
  }
  if (normalized > 1.0f) {
    normalized = 1.0f;
  }
  return normalized;
}

}  // namespace Audio
