/**
 * @file servo_calibration.h
 * @brief サーボ 1 軸分の校正モデル定義。
 */
#pragma once

namespace Actuator {

/**
 * @brief 1 軸のサーボ校正パラメーターを保持します。
 *
 * 値は firmware 側で保持し、WebUI（server）は論理角度のみ送信します。
 */
struct ServoAxisCalibration {
  float center_offset_deg{0.0f};        ///< 機体中央のズレ補正値（度）-45〜45
  float min_deg{-45.0f};                ///< 論理角度の最小値（度）-90〜0
  float max_deg{45.0f};                 ///< 論理角度の最大値（度）0〜90
  bool  invert{false};                  ///< サーボ回転方向の反転フラグ
  float speed_limit_deg_per_sec{60.0f}; ///< 角速度の上限（度/秒）1〜360
  bool  soft_start{true};               ///< ソフトスタート有効フラグ
  float home_deg{0.0f};                 ///< ホーム位置（論理角度・度）
};

}  // namespace Actuator
