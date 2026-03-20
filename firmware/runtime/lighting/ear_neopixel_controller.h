/**
 * @file ear_neopixel_controller.h
 * @brief NECO MIMI（NeoPixel 耳）制御サービス（P11-03）
 *
 * ## 責務
 * - NECO MIMI のWS2812B 互換 NeoPixel を制御する
 * - mode／color／brightness に基づくアニメーション（off / solid / blink / breathe / rainbow）
 * - device.ears.set イベントを受け取った StackchanSession が setMode() を呼ぶ
 *
 * ## オプションハードウェア対応
 * NECO MIMI は接続されていない場合がある。未接続時は以下の動作をする：
 * - available() が false を返す
 * - setMode() / update() は安全に呼び出せる（no-op）
 * - StackchanSession は error イベントを送信せず warning ログのみ出力する
 *
 * FW_NECO_MIMI_ENABLED の設定方法:
 * - platformio.ini.local の [neco_mimi] セクションで enabled = 1 に設定する
 * - NECO MIMI が接続されていない場合は enabled = 0 のまま（デフォルト）にする
 *
 * @note FW_NECO_MIMI_PIN は FastLED テンプレートの引数として使われるため、
 *       ビルドフラグ（-DFW_NECO_MIMI_PIN=2）で指定するコンパイル時定数です。
 */
#pragma once

#include <Arduino.h>

// NECO MIMI ピン・LED 数のデフォルト定義（platformio.ini の [neco_mimi] セクションで上書き可）
#ifndef FW_NECO_MIMI_PIN
  #define FW_NECO_MIMI_PIN 2    // CoreS3 Port A (SDA)
#endif
#ifndef FW_NECO_MIMI_NUM_LEDS
  #define FW_NECO_MIMI_NUM_LEDS 2  // NECO MIMI の LED 数（左右 1 個ずつ）
#endif
#ifndef FW_NECO_MIMI_ENABLED
  #define FW_NECO_MIMI_ENABLED 0   // 未接続のデフォルト（platformio.ini.local で 1 に上書き）
#endif

namespace Lighting {

/**
 * @brief device.ears.set で指定する点灯モードを表します。
 */
enum class EarMode : uint8_t {
  Off,     ///< 消灯
  Solid,   ///< 単色点灯
  Blink,   ///< 点滅
  Breathe, ///< 呼吸（明暗サイン波）
  Rainbow, ///< レインボーサイクル（色相を循環）
};

/**
 * @brief NECO MIMI（NeoPixel 耳）を制御するサービスクラス。
 *
 * FastLED（WS2812B）を使って NeoPixel アニメーションを管理します。
 * FW_NECO_MIMI_ENABLED=0 の場合、begin() は何もせず available() は false を返します。
 * StackchanSession は available() を確認せずに setMode() / update() を呼んでも安全です。
 */
class EarNeoPixelController {
 public:
  EarNeoPixelController() = default;

  /**
   * @brief FastLED を初期化し、LED を消灯した状態にします。
   *
   * FW_NECO_MIMI_ENABLED=0 の場合は何もしません。
   * setup() 内で 1 度だけ呼び出してください。
   * @param numLeds LED 数（デフォルト: FW_NECO_MIMI_NUM_LEDS）
   */
  void begin(int numLeds = FW_NECO_MIMI_NUM_LEDS);

  /**
   * @brief アニメーション状態を更新します。
   *
   * loop() 内で毎フレーム呼び出してください。
   * available() が false の場合は何もしません。
   */
  void update();

  /**
   * @brief device.ears.set ペイロードに基づいて LED モードを設定します。
   *
   * available() が false の場合は何もしません（呼び出しは安全）。
   *
   * @param mode              点灯パターン
   * @param rgb               RGB カラー（0xRRGGBB 形式）
   * @param brightness        輝度（0〜255）
   * @param blinkIntervalMs   blink モード時の点滅間隔（ms）
   * @param breathePeriodMs   breathe モード時の明暗 1 周期（ms）
   * @param rainbowPeriodMs   rainbow モード時の色相 1 周期（ms）
   */
  void setMode(EarMode mode,
               uint32_t rgb = 0,
               uint8_t brightness = 128,
               uint16_t blinkIntervalMs = 500,
               uint16_t breathePeriodMs = 2000,
               uint16_t rainbowPeriodMs = 3000);

  /**
   * @brief 全 LED を即時消灯します。setMode(EarMode::Off) と同義です。
   */
  void off();

  /** @brief NECO MIMI が有効かつ初期化済みかどうかを返します。 */
  bool available() const { return _initialized; }

 private:
  bool _initialized{false};
  int  _numLeds{0};

  // 現在のアニメーション設定
  EarMode  _mode{EarMode::Off};
  uint32_t _colorRGB{0};          ///< ターゲット色（0xRRGGBB）
  uint8_t  _brightness{128};      ///< 最大輝度（0〜255）
  uint16_t _blinkIntervalMs{500};
  uint16_t _breathePeriodMs{2000};
  uint16_t _rainbowPeriodMs{3000};

  // アニメーション状態
  unsigned long _animStartMs{0};  ///< アニメーション開始時刻
  bool          _blinkOn{false};
  unsigned long _lastUpdateMs{0}; ///< 最終 show() 更新時刻（レート制限用）

  // アニメーション更新間隔（ms）
  static constexpr uint16_t kUpdateIntervalMs = 20;
};

}  // namespace Lighting
