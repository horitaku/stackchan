/**
 * @file servo_calibration_store.cpp
 * @brief サーボ校正値の不揮発 read/write API 実装（P11-09）。
 */

#include "servo_calibration_store.h"

#include <Arduino.h>
#include <Preferences.h>

namespace {

static constexpr const char* kNsServoX = "servo_x";
static constexpr const char* kNsServoY = "servo_y";

static constexpr const char* kKeyCenter = "center_off";
static constexpr const char* kKeyMinDeg = "min_deg";
static constexpr const char* kKeyMaxDeg = "max_deg";
static constexpr const char* kKeyInvert = "invert";
static constexpr const char* kKeySpeed = "speed_lim";
static constexpr const char* kKeySoft = "soft_start";
static constexpr const char* kKeyHome = "home_deg";

}  // namespace

namespace Actuator {

bool ServoCalibrationStore::load(ServoAxisCalibration* calX, ServoAxisCalibration* calY) const {
  if (calX == nullptr || calY == nullptr) {
    return false;
  }

  const bool okX = loadAxis(kNsServoX, calX);
  const bool okY = loadAxis(kNsServoY, calY);
  return okX && okY;
}

bool ServoCalibrationStore::saveX(const ServoAxisCalibration& cal) const {
  return saveAxis(kNsServoX, cal);
}

bool ServoCalibrationStore::saveY(const ServoAxisCalibration& cal) const {
  return saveAxis(kNsServoY, cal);
}

bool ServoCalibrationStore::loadAxis(const char* ns, ServoAxisCalibration* cal) const {
  Preferences prefs;
  if (!prefs.begin(ns, /*readOnly=*/true)) {
    Serial.printf("[ServoCalibrationStore] WARN: Preferences.begin('%s') failed while loading\n", ns);
    return false;
  }

  cal->center_offset_deg = prefs.getFloat(kKeyCenter, cal->center_offset_deg);
  cal->min_deg = prefs.getFloat(kKeyMinDeg, cal->min_deg);
  cal->max_deg = prefs.getFloat(kKeyMaxDeg, cal->max_deg);
  cal->invert = prefs.getBool(kKeyInvert, cal->invert);
  cal->speed_limit_deg_per_sec = prefs.getFloat(kKeySpeed, cal->speed_limit_deg_per_sec);
  cal->soft_start = prefs.getBool(kKeySoft, cal->soft_start);
  cal->home_deg = prefs.getFloat(kKeyHome, cal->home_deg);

  prefs.end();
  return true;
}

bool ServoCalibrationStore::saveAxis(const char* ns, const ServoAxisCalibration& cal) const {
  Preferences prefs;
  if (!prefs.begin(ns, /*readOnly=*/false)) {
    Serial.printf("[ServoCalibrationStore] ERROR: Preferences.begin('%s') failed while saving\n", ns);
    return false;
  }

  const size_t w1 = prefs.putFloat(kKeyCenter, cal.center_offset_deg);
  const size_t w2 = prefs.putFloat(kKeyMinDeg, cal.min_deg);
  const size_t w3 = prefs.putFloat(kKeyMaxDeg, cal.max_deg);
  const size_t w4 = prefs.putBool(kKeyInvert, cal.invert);
  const size_t w5 = prefs.putFloat(kKeySpeed, cal.speed_limit_deg_per_sec);
  const size_t w6 = prefs.putBool(kKeySoft, cal.soft_start);
  const size_t w7 = prefs.putFloat(kKeyHome, cal.home_deg);

  prefs.end();

  return w1 > 0 && w2 > 0 && w3 > 0 && w4 > 0 && w5 > 0 && w6 > 0 && w7 > 0;
}

}  // namespace Actuator
