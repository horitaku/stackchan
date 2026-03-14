/**
 * @file mic_reader.h
 * @brief マイク読み取りインターフェース
 * 
 * Phase 5: テスト用サイレンス PCM データを生成します（ダミーモード）。
 * Phase 6 で M5Stack CoreS3 の内蔵マイクと接続します。
 * 
 * サンプルフォーマット: 16bit signed PCM, モノラル, 16kHz（デフォルト）
 */
#pragma once

#include <Arduino.h>

namespace Audio {

/**
 * @brief マイク読み取りクラス（Phase 5: ダミーモード）。
 * 
 * フレームサイズ（バイト）= sampleRateHz * frameDurationMs / 1000 * 2（16bit モノラル）
 */
class MicReader {
 public:
  /**
   * @param sampleRateHz   サンプルレート（Hz）、デフォルト: FW_AUDIO_SAMPLE_RATE
   * @param frameDurationMs フレーム長（ms）、デフォルト: FW_AUDIO_FRAME_MS
   */
  explicit MicReader(int sampleRateHz = FW_AUDIO_SAMPLE_RATE,
                     int frameDurationMs = FW_AUDIO_FRAME_MS);

  /**
   * @brief マイクを初期化します。
   * Phase 5 はダミーモードのため実際の初期化は行いません。
   * Phase 6 で M5.Mic.begin() を呼び出します。
   */
  void begin();

  /**
   * @brief 1 フレーム分の PCM データを buf に書き込みます。
   * Phase 5: ゼロ埋め（サイレンス）PCM を返します。
   * @param buf 出力バッファ（frameSizeBytes() 以上のサイズを確保してください）
   * @param len バッファサイズ（バイト）
   * @return 実際に書き込んだバイト数
   */
  size_t readFrame(uint8_t* buf, size_t len);

  /**
   * @brief 1 フレームのバイト数を返します。
   */
  size_t frameSizeBytes() const;

  int sampleRateHz()    const { return _sampleRateHz; }
  int frameDurationMs() const { return _frameDurationMs; }

 private:
  int _sampleRateHz;
  int _frameDurationMs;
};

}  // namespace Audio
