/**
 * @file servo_calibration_store.h
 * @brief サーボ校正値の不揮発 read/write API（P11-09）。
 */
#pragma once

#include "servo_calibration.h"

namespace Actuator {

/**
 * @brief NVS（Preferences）へサーボ校正値を保存/読み出しするストアです。
 */
class ServoCalibrationStore {
 public:
  /**
   * @brief X/Y 両軸の校正値を読み出します。
   *
   * 既存値がないキーは、呼び出し側が渡したデフォルト値を保持します。
   */
  bool load(ServoAxisCalibration* calX, ServoAxisCalibration* calY) const;

  /**
   * @brief X 軸校正値を保存します。
   */
  bool saveX(const ServoAxisCalibration& cal) const;

  /**
   * @brief Y 軸校正値を保存します。
   */
  bool saveY(const ServoAxisCalibration& cal) const;

 private:
  bool loadAxis(const char* ns, ServoAxisCalibration* cal) const;
  bool saveAxis(const char* ns, const ServoAxisCalibration& cal) const;
};

}  // namespace Actuator
