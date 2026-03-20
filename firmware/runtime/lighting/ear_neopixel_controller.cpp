/**
 * @file ear_neopixel_controller.cpp
 * @brief NECO MIMI（NeoPixel 耳）制御サービス実装（P11-03）
 *
 * ### オプションハードウェア対応
 *
 * FW_NECO_MIMI_ENABLED マクロで動作を切り替えます。
 *
 * - FW_NECO_MIMI_ENABLED=0（デフォルト）
 *   begin() は _initialized=false のまま何もせず return します。
 *   update() / setMode() / off() は _initialized チェックで early return します。
 *   → StackchanSession は常に begin() / update() / setMode() を呼んで安全です。
 *
 * - FW_NECO_MIMI_ENABLED=1（platformio.ini.local で有効化）
 *   FastLED に WS2812B ストリップを登録し、アニメーションを提供します。
 *
 * ### rainbow モード
 *
 * FastLED の fill_rainbow() で全 LED の色相を周期的にシフトします。
 * rainbowPeriodMs で 1 周期の時間を制御します。
 */
#include "ear_neopixel_controller.h"

#include <FastLED.h>
#include <math.h>

// NECO MIMI LED データ配列。FW_NECO_MIMI_NUM_LEDS はコンパイル時定数です。
// FW_NECO_MIMI_ENABLED=0 でもこの宣言はコンパイルされますが、
// addLeds() の呼び出しは #if ガードの内側にあるため、実際にはゼロオーバーヘッドです。
static CRGB s_earLeds[FW_NECO_MIMI_NUM_LEDS];

namespace Lighting {

// ──────────────────────────────────────────────────────────────────────
// 内部ヘルパー
// ──────────────────────────────────────────────────────────────────────

/** @brief 0xRRGGBB と輝度スケールから CRGB を生成します。 */
static CRGB earScaledColor(uint32_t rgb, uint8_t brightness) {
  uint8_t r = (uint8_t)(((rgb >> 16) & 0xFF) * brightness / 255);
  uint8_t g = (uint8_t)(((rgb >>  8) & 0xFF) * brightness / 255);
  uint8_t b = (uint8_t)((rgb         & 0xFF) * brightness / 255);
  return CRGB(r, g, b);
}

// ──────────────────────────────────────────────────────────────────────
// begin / update
// ──────────────────────────────────────────────────────────────────────

void EarNeoPixelController::begin(int numLeds) {
#if FW_NECO_MIMI_ENABLED
  _numLeds = min(numLeds, FW_NECO_MIMI_NUM_LEDS);

  // WS2812B を FW_NECO_MIMI_PIN（コンパイル時定数）に登録します。
  // BaseLedController と同一の FastLED インスタンスに追加されます。
  // FastLED.show() はすべての登録済みストリップを一括で更新します。
  FastLED.addLeds<WS2812B, FW_NECO_MIMI_PIN, GRB>(s_earLeds, _numLeds);
  FastLED.clear(true);

  _initialized = true;
  Serial.printf("[EAR] EarNeoPixelController ready: pin=%d num=%d\n",
    FW_NECO_MIMI_PIN, _numLeds);
#else
  // NECO MIMI 未使用: 警告ログのみでハードウェアは一切操作しません。
  Serial.println("[EAR] NECO MIMI disabled (FW_NECO_MIMI_ENABLED=0)");
#endif
}

void EarNeoPixelController::update() {
  if (!_initialized) return;

  const unsigned long now = millis();

  switch (_mode) {
    case EarMode::Off:
    case EarMode::Solid:
      // setMode() で即時反映済みです。
      break;

    case EarMode::Blink: {
      const bool shouldBeOn = (((now - _animStartMs) / _blinkIntervalMs) % 2) == 0;
      if (shouldBeOn != _blinkOn) {
        _blinkOn = shouldBeOn;
        const CRGB color = _blinkOn ? earScaledColor(_colorRGB, _brightness) : CRGB::Black;
        fill_solid(s_earLeds, _numLeds, color);
        FastLED.show();
      }
      break;
    }

    case EarMode::Breathe: {
      if ((now - _lastUpdateMs) < kUpdateIntervalMs) break;
      _lastUpdateMs = now;

      const float phase = fmod((float)(now - _animStartMs), (float)_breathePeriodMs)
                        / (float)_breathePeriodMs;
      const float sinVal = (sinf(phase * 2.0f * (float)M_PI - (float)M_PI / 2.0f) + 1.0f) * 0.5f;
      const uint8_t b   = (uint8_t)(sinVal * (float)_brightness);

      fill_solid(s_earLeds, _numLeds, earScaledColor(_colorRGB, b));
      FastLED.show();
      break;
    }

    case EarMode::Rainbow: {
      // kUpdateIntervalMs ごとに色相を進めます。
      if ((now - _lastUpdateMs) < kUpdateIntervalMs) break;
      _lastUpdateMs = now;

      // rainbowPeriodMs で 0〜255 の hue が一周します。
      const uint8_t hue = (uint8_t)(
        (uint32_t)((now - _animStartMs) % _rainbowPeriodMs) * 256 / _rainbowPeriodMs
      );
      // 全 LED に等間隔の色相を割り当てます（fill_rainbow は FastLED の組み込み関数）。
      fill_rainbow(s_earLeds, _numLeds, hue, 256 / max(_numLeds, 1));
      // 輝度スケール適用（fill_rainbow は輝度を考慮しないため、後から乗算します）。
      for (int i = 0; i < _numLeds; i++) {
        s_earLeds[i].nscale8(_brightness);
      }
      FastLED.show();
      break;
    }
  }
}

// ──────────────────────────────────────────────────────────────────────
// setMode / off
// ──────────────────────────────────────────────────────────────────────

void EarNeoPixelController::setMode(EarMode mode, uint32_t rgb, uint8_t brightness,
                                     uint16_t blinkIntervalMs, uint16_t breathePeriodMs,
                                     uint16_t rainbowPeriodMs) {
  if (!_initialized) return;

  _mode             = mode;
  _colorRGB         = rgb;
  _brightness       = brightness;
  _blinkIntervalMs  = (blinkIntervalMs > 0) ? blinkIntervalMs : 500;
  _breathePeriodMs  = (breathePeriodMs > 0) ? breathePeriodMs : 2000;
  _rainbowPeriodMs  = (rainbowPeriodMs > 0) ? rainbowPeriodMs : 3000;
  _animStartMs      = millis();
  _blinkOn          = false;
  _lastUpdateMs     = 0;

  switch (mode) {
    case EarMode::Off:
      fill_solid(s_earLeds, _numLeds, CRGB::Black);
      FastLED.show();
      break;
    case EarMode::Solid:
      fill_solid(s_earLeds, _numLeds, earScaledColor(_colorRGB, _brightness));
      FastLED.show();
      break;
    default:
      // blink / breathe / rainbow は update() ループで処理します。
      break;
  }

  Serial.printf("[EAR] mode=%d rgb=0x%06X brightness=%d\n",
    (int)mode, (unsigned)rgb, brightness);
}

void EarNeoPixelController::off() {
  setMode(EarMode::Off);
}

}  // namespace Lighting
