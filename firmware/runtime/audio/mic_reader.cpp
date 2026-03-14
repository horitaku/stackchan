/**
 * @file mic_reader.cpp
 * @brief マイク読み取りモジュール実装（Phase 5: ダミーモード）
 */
#include "mic_reader.h"

namespace Audio {

MicReader::MicReader(int sampleRateHz, int frameDurationMs)
    : _sampleRateHz(sampleRateHz), _frameDurationMs(frameDurationMs) {}

void MicReader::begin() {
  // Phase 5: ダミーモードのため実際の初期化は行いません
  // Phase 6 で以下を有効化します:
  //   auto mic_cfg = M5.Mic.config();
  //   mic_cfg.sample_rate = _sampleRateHz;
  //   M5.Mic.config(mic_cfg);
  //   M5.Mic.begin();
  Serial.printf("[Mic] MicReader initialized (DUMMY mode, %d Hz, %d ms/frame, %zu bytes/frame)\n",
    _sampleRateHz, _frameDurationMs, frameSizeBytes());
}

size_t MicReader::frameSizeBytes() const {
  // 16bit（2 バイト）モノラル PCM のフレームサイズを計算します
  return static_cast<size_t>(_sampleRateHz * _frameDurationMs / 1000 * 2);
}

size_t MicReader::readFrame(uint8_t* buf, size_t len) {
  size_t toWrite = min(len, frameSizeBytes());
  // Phase 5: サイレンス（ゼロ PCM）を出力します
  // Phase 6: M5.Mic.record() を使って実マイクから録音します
  memset(buf, 0, toWrite);
  return toWrite;
}

}  // namespace Audio
