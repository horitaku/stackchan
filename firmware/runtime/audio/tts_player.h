/**
 * @file tts_player.h
 * @brief TTS 再生制御モジュール
 *
 * Base64 デコード済み PCM16 データを CoreS3 スピーカーへ再生します。
 * フェーズ 6 では最小状態管理（Idle/Buffering/Playing/Stopping/Error）と
 * 振幅ベースの簡易リップシンク値を提供します。
 */
#pragma once

#include <Arduino.h>

namespace Audio {

enum class PlaybackState {
  Idle,
  Buffering,
  Playing,
  Stopping,
  Error,
};

class TTSPlayer {
 public:
  TTSPlayer() = default;

  /**
   * @brief スピーカー出力を初期化します。
   */
  void begin();

  /**
   * @brief PCM16 モノラルデータを再生します。
   * @param pcmBytes      16-bit signed little-endian PCM バイト列
   * @param byteLen       データ長（バイト）
   * @param sampleRateHz  サンプルレート（Hz）
   * @param stopCurrent   既存再生を停止して差し替える場合 true
   * @return 再生開始できた場合 true
   */
  bool playPCM16(const uint8_t* pcmBytes, size_t byteLen, uint32_t sampleRateHz, bool stopCurrent);

  /**
   * @brief 状態更新処理。loop() で定期呼び出しします。
   */
  void update();

  /**
   * @brief 再生を停止します。
   */
  void stop();

  bool isPlaying() const { return _state == PlaybackState::Playing; }
  PlaybackState state() const { return _state; }

  /**
   * @brief 0.0〜1.0 の口開閉係数（簡易）を返します。
   */
  float lipLevel() const { return _lipLevel; }

  uint32_t startLatencyMs() const { return _startLatencyMs; }
  uint32_t lastPlaybackDurationMs() const { return _lastPlaybackDurationMs; }

 private:
  PlaybackState _state{PlaybackState::Idle};

  // 再生中バッファ（再生完了まで保持）
  uint8_t* _pcmBuffer{nullptr};
  size_t _pcmByteLen{0};
  uint32_t _sampleRateHz{0};

  // 計測
  uint32_t _requestedAtMs{0};
  uint32_t _startedAtMs{0};
  uint32_t _estimatedDurationMs{0};
  uint32_t _startLatencyMs{0};
  uint32_t _lastPlaybackDurationMs{0};

  // リップシンク
  float _baseLipLevel{0.0f};
  float _lipLevel{0.0f};

  void clearBuffer();
  float estimateBaseLipLevel(const int16_t* samples, size_t sampleCount) const;
};

}  // namespace Audio
