/**
 * @file base_led_controller.h
 * @brief M5GO Bottom3 RGB LED 制御サービス（P11-03）
 *
 * ## 責務
 * - M5GO Bottom3 に搭載された WS2812B 互換 RGB LED を制御する
 * - mode／color／brightness に基づくアニメーション（off / solid / blink / breathe）
 * - device.led.set イベントを受け取った StackchanSession が setMode() を呼ぶ
 *
 * ## 呼び出し方
 * 1. begin()    – setup() 内で 1 度だけ呼ぶ
 * 2. update()   – loop() 内で毎フレーム呼ぶ（アニメーション更新）
 * 3. setMode()  – device.led.set 受信時に呼ぶ
 * 4. off()      – 強制消灯（会話終了・エラー時など）
 *
 * ## LED ピンの設定
 * platformio.ini の [led] セクションでピン番号を設定してください。
 * デフォルトは M5GO Bottom3 + CoreS3 構成（GPIO 25）を想定しています。
 *
 * @note FW_LED_PIN は FastLED テンプレートの引数として使われるため、
 *       ビルドフラグ（-DFW_LED_PIN=25）で指定するコンパイル時定数です。
 */
#pragma once

#include <Arduino.h>

// LED ピン番号のデフォルト定義（platformio.ini の [led] セクションで上書き可）
// ⚠️ GPIO 25 は ESP32-S3 (CoreS3) には存在しません。
//    platformio.ini.local の [led] セクションで実際のピン番号を指定してください。
#ifndef FW_LED_PIN
  #define FW_LED_PIN 4    // デフォルト値（要変更：M5GO Bottom3 の正しいピンを確認）
#endif
#ifndef FW_LED_NUM
  #define FW_LED_NUM 10   // M5GO Bottom3 搭載 LED 数
#endif

namespace Lighting {

/**
 * @brief device.led.set で指定する点灯モードを表します。
 */
enum class LedMode : uint8_t {
  Off,     ///< 消灯
  Solid,   ///< 単色点灯
  Blink,   ///< 点滅
  Breathe, ///< 呼吸（明暗サイン波）
};

/**
 * @brief M5GO Bottom3 のRGB LED を制御するサービスクラス。
 *
 * FastLED（WS2812B）を使って LED アニメーションを管理します。
 * StackchanSession はこのクラスを所有し、device.led.set イベント受信時に
 * setMode() を呼び出します。
 */
class BaseLedController {
 public:
  BaseLedController() = default;

  /**
   * @brief FastLED を初期化し、LED を消灯した状態にします。
   *
   * setup() 内で 1 度だけ呼び出してください。
   * @param numLeds LED 数（デフォルト: FW_LED_NUM）
   */
  void begin(int numLeds = FW_LED_NUM);

  /**
   * @brief アニメーション状態を更新します。
   *
   * loop() 内で毎フレーム呼び出してください。
   * blink / breathe モードの場合のみ LED を更新します。
   */
  void update();

  /**
   * @brief device.led.set ペイロードに基づいて LED モードを設定します。
   *
   * @param mode             点灯パターン
   * @param rgb              RGB カラー（0xRRGGBB 形式）。mode=off の場合は無視
   * @param brightness       輝度（0〜255）
   * @param blinkIntervalMs  blink モード時の点滅間隔（ms）
   * @param breathePeriodMs  breathe モード時の明暗 1 周期（ms）
   */
  void setMode(LedMode mode,
               uint32_t rgb = 0,
               uint8_t brightness = 128,
               uint16_t blinkIntervalMs = 500,
               uint16_t breathePeriodMs = 2000);

  /**
   * @brief 全 LED を即時消灯します。setMode(LedMode::Off) と同義です。
   */
  void off();

  /** @brief ハードウェア初期化済みかどうかを返します。 */
  bool available() const { return _initialized; }

 private:
  bool _initialized{false};
  int  _numLeds{0};

  // 現在のアニメーション設定
  LedMode  _mode{LedMode::Off};
  uint32_t _colorRGB{0};         ///< ターゲット色（0xRRGGBB）
  uint8_t  _brightness{128};     ///< 最大輝度（0〜255）
  uint16_t _blinkIntervalMs{500};
  uint16_t _breathePeriodMs{2000};

  // アニメーション状態
  unsigned long _animStartMs{0}; ///< アニメーション開始時刻
  bool          _blinkOn{false}; ///< 現在の blink 状態
  unsigned long _lastUpdateMs{0};///< 最終 show() 更新時刻（breathe レート制限用）

  // アニメーション更新間隔（ms）。頻繁すぎる show() を避けるため。
  static constexpr uint16_t kUpdateIntervalMs = 20;
};

}  // namespace Lighting
