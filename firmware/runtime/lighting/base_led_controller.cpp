/**
 * @file base_led_controller.cpp
 * @brief M5GO Bottom3 RGB LED 制御サービス実装（P11-03）
 *
 * ### アニメーション実装の方針
 *
 * - off:     FastLED.clear(true) で即時消灯
 * - solid:   輝度スケール済みの色を全 LED に設定して即時表示
 * - blink:   kUpdateIntervalMs ごとにトグル状態を確認し、変化時のみ show()
 * - breathe: kUpdateIntervalMs ごとにサイン波で輝度を計算して show()
 *
 * FastLED.show() は RMT 経由で WS2812B タイミングを生成します。
 * BaseLedController と EarNeoPixelController は同一の FastLED インスタンスを
 * 共有し、一方が show() を呼ぶともう一方の LED も同時に更新されます。
 * これは設計上の意図した動作です（FastLED のマルチストリップ仕様）。
 *
 * ### 不揮発保存なし
 * LED 設定は再起動時にリセットされます。サーボとは異なり、
 * LED の「校正値」は存在しないためです。
 */
#include "base_led_controller.h"

#include <FastLED.h>
#include <math.h>

// LED データ配列。FastLED の要件により静的グローバルに置きます。
// FW_LED_NUM は build flag で定義されたコンパイル時定数です。
static CRGB s_baseLeds[FW_LED_NUM];

namespace Lighting {

// ──────────────────────────────────────────────────────────────────────
// 内部ヘルパー
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief 0xRRGGBB 形式の色と輝度スケールから CRGB を生成します。
 *
 * FastLED の setBrightness() はグローバルスケーラーのため、
 * ここでは色成分ごとに輝度を乗算して個別制御します。
 */
static CRGB scaledColor(uint32_t rgb, uint8_t brightness) {
  uint8_t r = (uint8_t)(((rgb >> 16) & 0xFF) * brightness / 255);
  uint8_t g = (uint8_t)(((rgb >>  8) & 0xFF) * brightness / 255);
  uint8_t b = (uint8_t)((rgb         & 0xFF) * brightness / 255);
  return CRGB(r, g, b);
}

// ──────────────────────────────────────────────────────────────────────
// begin / update
// ──────────────────────────────────────────────────────────────────────

void BaseLedController::begin(int numLeds) {
  _numLeds = min(numLeds, FW_LED_NUM);

  // WS2812B LED を FW_LED_PIN（コンパイル時定数）に登録します。
  // FastLED は静的グローバル配列へのポインタを保持します。
  FastLED.addLeds<WS2812B, FW_LED_PIN, GRB>(s_baseLeds, _numLeds);
  FastLED.clear(true);  // 全 LED 消灯して初期状態を確立します。

  _initialized = true;
  Serial.printf("[LED] BaseLedController ready: pin=%d num=%d\n",
    FW_LED_PIN, _numLeds);
}

void BaseLedController::update() {
  if (!_initialized) return;

  const unsigned long now = millis();

  switch (_mode) {
    case LedMode::Off:
    case LedMode::Solid:
      // これらのモードは setMode() 時点で即時反映済みです。
      // update() では何もしません。
      break;

    case LedMode::Blink: {
      // blink 間隔ごとにオン/オフをトグルし、変化があった時だけ show() します。
      const bool shouldBeOn = (((now - _animStartMs) / _blinkIntervalMs) % 2) == 0;
      if (shouldBeOn != _blinkOn) {
        _blinkOn = shouldBeOn;
        const CRGB color = _blinkOn ? scaledColor(_colorRGB, _brightness) : CRGB::Black;
        fill_solid(s_baseLeds, _numLeds, color);
        FastLED.show();
      }
      break;
    }

    case LedMode::Breathe: {
      // kUpdateIntervalMs ごとにサイン波で輝度を変化させます。
      if ((now - _lastUpdateMs) < kUpdateIntervalMs) break;
      _lastUpdateMs = now;

      // phase は 0.0〜1.0 の周期位置。-π/2 オフセットで 0 から始まります。
      const float phase = fmod((float)(now - _animStartMs), (float)_breathePeriodMs)
                        / (float)_breathePeriodMs;
      const float sinVal = (sinf(phase * 2.0f * (float)M_PI - (float)M_PI / 2.0f) + 1.0f) * 0.5f;
      const uint8_t b   = (uint8_t)(sinVal * (float)_brightness);

      fill_solid(s_baseLeds, _numLeds, scaledColor(_colorRGB, b));
      FastLED.show();
      break;
    }
  }
}

// ──────────────────────────────────────────────────────────────────────
// setMode / off
// ──────────────────────────────────────────────────────────────────────

void BaseLedController::setMode(LedMode mode, uint32_t rgb, uint8_t brightness,
                                 uint16_t blinkIntervalMs, uint16_t breathePeriodMs) {
  if (!_initialized) return;

  _mode             = mode;
  _colorRGB         = rgb;
  _brightness       = brightness;
  _blinkIntervalMs  = (blinkIntervalMs > 0) ? blinkIntervalMs : 500;
  _breathePeriodMs  = (breathePeriodMs > 0) ? breathePeriodMs : 2000;
  _animStartMs      = millis();
  _blinkOn          = false;
  _lastUpdateMs     = 0;

  // off と solid は update() を待たずに即時反映します。
  switch (mode) {
    case LedMode::Off:
      FastLED.clear(true);
      break;
    case LedMode::Solid:
      fill_solid(s_baseLeds, _numLeds, scaledColor(_colorRGB, _brightness));
      FastLED.show();
      break;
    default:
      // blink / breathe は update() ループで処理します。
      break;
  }

  Serial.printf("[LED] mode=%d rgb=0x%06X brightness=%d\n",
    (int)mode, (unsigned)rgb, brightness);
}

void BaseLedController::off() {
  setMode(LedMode::Off);
}

}  // namespace Lighting
