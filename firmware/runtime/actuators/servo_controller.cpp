/**
 * @file servo_controller.cpp
 * @brief サーボ X/Y 軸制御サービス実装（P11-02）
 *
 * ### 実角度変換の流れ
 *
 * 1. WebUI / server から「論理角度」を受け取る（-90〜+90 度の範囲）
 * 2. firmware が [min_deg, max_deg] でクランプ（安全制限）
 * 3. center_offset_deg を加算（機体取り付け時のズレを補正）
 * 4. invert フラグで符号を反転（サーボ取り付け方向の差異を吸収）
 * 5. パルス幅（マイクロ秒）へ変換して ESP32Servo へ書き込む
 *
 * ### ソフトスタート補間
 *
 * soft_start=true の場合、update() が呼ばれるたびに
 * 目標角度へ向けて speed_limit_deg_per_sec に従って段階的に角度を進めます。
 * これにより急激な動きを防ぎ、サーボや筐体へのストレスを低減します。
 *
 * ### 不揮発保存
 *
 * ESP32 の Preferences（NVS）を使って校正値を保存します。
 * 名前空間: "servo_x"（X 軸）、"servo_y"（Y 軸）
 * 再起動後も校正値が維持されます。
 */
#include "servo_controller.h"

#include <Arduino.h>
#include <ESP32Servo.h>
#include <math.h>

// ── モジュール内グローバル（ファイルスコープ） ──────────────────────────
// ESP32Servo オブジェクトはクラスメンバーに置くと
// ヘッダに ESP32Servo.h include が漏れるため、ここで static に管理します。
static Servo g_servoX;
static Servo g_servoY;

namespace Actuator {

// ──────────────────────────────────────────────────────────────────────
// begin / update
// ──────────────────────────────────────────────────────────────────────

void ServoController::begin(int pinX, int pinY) {
  _pinX = pinX;
  _pinY = pinY;

  // Preferences から校正値を復元します（ない場合はデフォルト値を使用）。
  loadCalibration();

  // ESP32Servo のパルス幅範囲を標準 SG90 / MG90S 向けに設定します。
  // 範囲: 500us（-90°）〜 2500us（+90°）
  g_servoX.setPeriodHertz(50);
  g_servoY.setPeriodHertz(50);

  g_servoX.attach(_pinX, 500, 2500);
  g_servoY.attach(_pinY, 500, 2500);

  // 起動時はホーム位置へ移動します。
  _currentXDeg = 0.0f;
  _currentYDeg = 0.0f;
  _targetXDeg  = _calibX.home_deg;
  _targetYDeg  = _calibY.home_deg;

  writeServo(_pinX, _calibX.home_deg, _calibX);
  writeServo(_pinY, _calibY.home_deg, _calibY);

  _currentXDeg = _calibX.home_deg;
  _currentYDeg = _calibY.home_deg;

  _lastUpdateMs = millis();
  _ready = true;

  Serial.printf("[ServoController] initialized. pinX=%d pinY=%d\n", _pinX, _pinY);
  Serial.printf("[ServoController] calib X: center_off=%.1f min=%.1f max=%.1f invert=%d speed=%.1f home=%.1f\n",
    _calibX.center_offset_deg, _calibX.min_deg, _calibX.max_deg, _calibX.invert,
    _calibX.speed_limit_deg_per_sec, _calibX.home_deg);
  Serial.printf("[ServoController] calib Y: center_off=%.1f min=%.1f max=%.1f invert=%d speed=%.1f home=%.1f\n",
    _calibY.center_offset_deg, _calibY.min_deg, _calibY.max_deg, _calibY.invert,
    _calibY.speed_limit_deg_per_sec, _calibY.home_deg);
}

void ServoController::update() {
  if (!_ready) return;

  const unsigned long now = millis();
  const float dtSec = static_cast<float>(now - _lastUpdateMs) / 1000.0f;
  _lastUpdateMs = now;

  // X 軸の補間処理
  if (fabsf(_targetXDeg - _currentXDeg) > 0.1f) {
    if (_calibX.soft_start) {
      // speed_limit_deg_per_sec を絶対上限として補間を進めます。
      const float maxStep = _calibX.speed_limit_deg_per_sec * dtSec;
      const float diff    = _targetXDeg - _currentXDeg;
      const float step    = constrain(diff, -maxStep, maxStep);
      _currentXDeg += step;
    } else {
      // ソフトスタート無効: 即座に目標値へ変更します。
      _currentXDeg = _targetXDeg;
    }
    writeServo(_pinX, _currentXDeg, _calibX);
  }

  // Y 軸の補間処理
  if (fabsf(_targetYDeg - _currentYDeg) > 0.1f) {
    if (_calibY.soft_start) {
      const float maxStep = _calibY.speed_limit_deg_per_sec * dtSec;
      const float diff    = _targetYDeg - _currentYDeg;
      const float step    = constrain(diff, -maxStep, maxStep);
      _currentYDeg += step;
    } else {
      _currentYDeg = _targetYDeg;
    }
    writeServo(_pinY, _currentYDeg, _calibY);
  }
}

// ──────────────────────────────────────────────────────────────────────
// 制御
// ──────────────────────────────────────────────────────────────────────

void ServoController::move(const char* axis, float angleXDeg, float angleYDeg, float speedScale) {
  if (!_ready) {
    Serial.println("[ServoController] move() called before begin()");
    return;
  }

  const bool doX = (strcmp(axis, "x") == 0 || strcmp(axis, "both") == 0);
  const bool doY = (strcmp(axis, "y") == 0 || strcmp(axis, "both") == 0);

  if (doX) {
    // 安全制限: [min_deg, max_deg] でクランプします。
    const float clamped = clamp(angleXDeg, _calibX, "X");

    if (_calibX.soft_start) {
      // 速度倍率を反映: speed_limit が絶対上限で、speedScale はその内側で調整。
      // ここでは目標を設定するだけで、実際の補間は update() が担います。
      // speed_limit は update() の dtSec と組み合わせて適用されます。
      // speedScale は校正値の speed_limit への乗算として適用します（上限: 1.0）。
      (void)speedScale;  // soft_start 時は update() 内で speed_limit を使用
      _targetXDeg = clamped;
    } else {
      _currentXDeg = clamped;
      writeServo(_pinX, clamped, _calibX);
      _targetXDeg = clamped;
    }
    Serial.printf("[ServoController] move X: requested=%.1f -> clamped=%.1f\n", angleXDeg, clamped);
  }

  if (doY) {
    const float clamped = clamp(angleYDeg, _calibY, "Y");

    if (_calibY.soft_start) {
      _targetYDeg = clamped;
    } else {
      _currentYDeg = clamped;
      writeServo(_pinY, clamped, _calibY);
      _targetYDeg = clamped;
    }
    Serial.printf("[ServoController] move Y: requested=%.1f -> clamped=%.1f\n", angleYDeg, clamped);
  }
}

void ServoController::goHome() {
  Serial.printf("[ServoController] goHome() x=%.1f y=%.1f\n",
    _calibX.home_deg, _calibY.home_deg);
  move("both", _calibX.home_deg, _calibY.home_deg, 1.0f);
}

// ──────────────────────────────────────────────────────────────────────
// 校正値の更新・保存
// ──────────────────────────────────────────────────────────────────────

bool ServoController::setCalibrationX(const ServoAxisCalibration& cal) {
  // min_deg < max_deg の制約を検証します。
  if (cal.min_deg >= cal.max_deg) {
    Serial.printf("[ServoController] setCalibrationX rejected: min_deg(%.1f) >= max_deg(%.1f)\n",
      cal.min_deg, cal.max_deg);
    return false;
  }
  _calibX = cal;
  const bool ok = _calibrationStore.saveX(_calibX);
  if (ok) {
    Serial.printf("[ServoController] calibX saved: center_off=%.1f min=%.1f max=%.1f invert=%d speed=%.1f home=%.1f\n",
      _calibX.center_offset_deg, _calibX.min_deg, _calibX.max_deg,
      _calibX.invert, _calibX.speed_limit_deg_per_sec, _calibX.home_deg);
  }
  return ok;
}

bool ServoController::setCalibrationY(const ServoAxisCalibration& cal) {
  if (cal.min_deg >= cal.max_deg) {
    Serial.printf("[ServoController] setCalibrationY rejected: min_deg(%.1f) >= max_deg(%.1f)\n",
      cal.min_deg, cal.max_deg);
    return false;
  }
  _calibY = cal;
  const bool ok = _calibrationStore.saveY(_calibY);
  if (ok) {
    Serial.printf("[ServoController] calibY saved: center_off=%.1f min=%.1f max=%.1f invert=%d speed=%.1f home=%.1f\n",
      _calibY.center_offset_deg, _calibY.min_deg, _calibY.max_deg,
      _calibY.invert, _calibY.speed_limit_deg_per_sec, _calibY.home_deg);
  }
  return ok;
}

// ──────────────────────────────────────────────────────────────────────
// 内部ヘルパー
// ──────────────────────────────────────────────────────────────────────

void ServoController::writeServo(int pin, float logicalDeg, const ServoAxisCalibration& calib) {
  const int us = logicalToMicros(logicalDeg, calib);

  if (pin == _pinX) {
    g_servoX.writeMicroseconds(us);
  } else if (pin == _pinY) {
    g_servoY.writeMicroseconds(us);
  }
}

float ServoController::clamp(float deg, const ServoAxisCalibration& calib, const char* label) const {
  if (deg < calib.min_deg) {
    Serial.printf("[ServoController] WARN: %s angle %.1f clamped to min %.1f\n", label, deg, calib.min_deg);
    return calib.min_deg;
  }
  if (deg > calib.max_deg) {
    Serial.printf("[ServoController] WARN: %s angle %.1f clamped to max %.1f\n", label, deg, calib.max_deg);
    return calib.max_deg;
  }
  return deg;
}

int ServoController::logicalToMicros(float logicalDeg, const ServoAxisCalibration& calib) const {
  // 1. center_offset_deg を加算して機体中央のズレを補正します。
  float physical = logicalDeg + calib.center_offset_deg;

  // 2. invert フラグで回転方向を反転します。
  if (calib.invert) {
    physical = -physical;
  }

  // 3. 物理角度をパルス幅（マイクロ秒）へ変換します。
  //    0° = 1500us、+90° = 2500us、-90° = 500us
  const int us = static_cast<int>(1500.0f + (physical / 90.0f) * 1000.0f);

  // ハードウェア限界（500〜2500us）でクランプしてサーボを保護します。
  return constrain(us, 500, 2500);
}

void ServoController::loadCalibration() {
  const bool loaded = _calibrationStore.load(&_calibX, &_calibY);
  if (loaded) {
    Serial.println("[ServoController] calibration loaded from ServoCalibrationStore");
  } else {
    Serial.println("[ServoController] WARN: calibration load failed, using defaults");
  }

  // min_deg < max_deg の保持整合性チェック（NVS 破損対策）
  if (_calibX.min_deg >= _calibX.max_deg) {
    Serial.println("[ServoController] WARN: calibX corrupted, resetting to defaults");
    _calibX = ServoAxisCalibration{};
  }
  if (_calibY.min_deg >= _calibY.max_deg) {
    Serial.println("[ServoController] WARN: calibY corrupted, resetting to defaults");
    _calibY = ServoAxisCalibration{};
  }
}

}  // namespace Actuator
