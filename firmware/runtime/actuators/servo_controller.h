/**
 * @file servo_controller.h
 * @brief サーボ X/Y 軸制御サービス（P11-02）
 *
 * ## 責務
 * - 論理角度 → 実角度変換（校正値の適用）
 * - 安全制限（min_deg / max_deg / speed_limit_deg_per_sec）
 * - ソフトスタート補間（update() の定期呼び出しで動作）
 * - 校正値の不揮発保存と読み出し（ESP32 Preferences）
 * - device.servo.move / calibration.get / calibration.set に対応
 *
 * ## 呼び出し方
 * 1. begin(pinX, pinY)     – setup() 内で 1 度だけ呼ぶ
 * 2. update()               – loop() 内で毎フレーム呼ぶ（補間処理に必要）
 * 3. move(...)              – device.servo.move 受信時に呼ぶ
 * 4. goHome()               – ホーム位置へ戻す
 * 5. calibrationX/Y()       – 現在の校正値を取得して calibration.response を構築
 * 6. setCalibrationX/Y()    – 校正値を更新して Preferences に保存
 *
 * ## 安全モデル
 * 外部から受け取る論理角度は firmware が校正値でクランプしてから実角度へ変換します。
 * WebUI や server は raw angle（物理パルス幅）を直接送ることはありません。
 *
 * @note GPIO ピン番号はビルドフラグ FW_SERVO_X_PIN / FW_SERVO_Y_PIN で
 *       上書きできます。デフォルトは CoreS3 + M5Go Bottom3 構成を想定しています。
 */
#pragma once

#include <Arduino.h>

#include "servo_calibration.h"
#include "servo_calibration_store.h"

// ── サーボピン番号のデフォルト定義 ──────────────────────────────────────
// platformio.ini の build_flags で上書き可能です。
// 例: -DFW_SERVO_X_PIN=17 -DFW_SERVO_Y_PIN=18
#ifndef FW_SERVO_X_PIN
  #define FW_SERVO_X_PIN 1   // CoreS3 Port A (SDA): 左右方向
#endif
#ifndef FW_SERVO_Y_PIN
  #define FW_SERVO_Y_PIN 2   // CoreS3 Port A (SCL): 上下方向
#endif

namespace Actuator {

// ── ServoController ───────────────────────────────────────────────────────
/**
 * @brief サーボ X/Y 2 軸を管理するサービスクラス。
 *
 * StackchanSession はこのクラスを保持し、受信した device.servo.* イベントを
 * move() / setCalibrationX() / setCalibrationY() / goHome() へ委譲します。
 */
class ServoController {
 public:
  ServoController() = default;

  // ── 初期化 / ループ ──────────────────────────────────────────────────

  /**
   * @brief サーボを初期化し、Preferences から校正値を読み出します。
   *
   * @param pinX X 軸（左右）のサーボ GPIO ピン番号
   * @param pinY Y 軸（上下）のサーボ GPIO ピン番号
   */
  void begin(int pinX = FW_SERVO_X_PIN, int pinY = FW_SERVO_Y_PIN);

  /**
   * @brief 補間処理を進め、サーボパルスを更新します。
   * loop() 内で毎フレーム呼び出してください。
   */
  void update();

  // ── 制御 ─────────────────────────────────────────────────────────────

  /**
   * @brief サーボを指定した論理角度へ移動します（device.servo.move 対応）。
   *
   * 角度は [min_deg, max_deg] でクランプされてから校正値を適用します。
   * クランプが発生した場合は warning ログを出力します。
   *
   * @param axis       "x", "y", "both" を指定します
   * @param angleXDeg  X 軸の目標論理角度（度）。axis="x" または "both" のとき有効
   * @param angleYDeg  Y 軸の目標論理角度（度）。axis="y" または "both" のとき有効
   * @param speedScale 速度倍率（0.1〜3.0）。校正値の speed_limit_deg_per_sec が絶対上限
   */
  void move(const char* axis, float angleXDeg, float angleYDeg, float speedScale = 1.0f);

  /**
   * @brief 両軸をホーム位置へ戻します。
   * 内部で move("both", calibX.home_deg, calibY.home_deg) を呼び出します。
   */
  void goHome();

  // ── 状態取得 ─────────────────────────────────────────────────────────

  /**
   * @brief 現在の X 軸論理角度（写像後の値。補間中は補間途中の値）を返します。
   */
  float currentAngleXDeg() const { return _currentXDeg; }

  /**
   * @brief 現在の Y 軸論理角度を返します。
   */
  float currentAngleYDeg() const { return _currentYDeg; }

  /**
   * @brief X 軸の校正値を返します（calibration.response 構築用）。
   */
  const ServoAxisCalibration& calibrationX() const { return _calibX; }

  /**
   * @brief Y 軸の校正値を返します。
   */
  const ServoAxisCalibration& calibrationY() const { return _calibY; }

  /**
   * @brief 初期化が完了しているかを返します。
   */
  bool isReady() const { return _ready; }

  // ── 校正値の更新・保存 ────────────────────────────────────────────────

  /**
   * @brief X 軸の校正値を差分更新して Preferences に保存します（calibration.set 対応）。
   * 省略フィールドは現在値を維持します。
   *
   * @param cal 更新する校正値（フィールドをすべて設定してから渡してください）
   * @return 保存成功: true、失敗: false
   */
  bool setCalibrationX(const ServoAxisCalibration& cal);

  /**
   * @brief Y 軸の校正値を差分更新して Preferences に保存します。
   *
   * @param cal 更新する校正値
   * @return 保存成功: true、失敗: false
   */
  bool setCalibrationY(const ServoAxisCalibration& cal);

 private:
  // ── 内部状態 ─────────────────────────────────────────────────────────

  bool   _ready{false};

  // 各軸の GPIO ピン番号
  int    _pinX{FW_SERVO_X_PIN};
  int    _pinY{FW_SERVO_Y_PIN};

  // 校正値（Preferences から復元 or デフォルト）
  ServoAxisCalibration _calibX;
  ServoAxisCalibration _calibY;

  // 現在の論理角度（補間の「現在地」）
  float  _currentXDeg{0.0f};
  float  _currentYDeg{0.0f};

  // 目標論理角度（補間の「目標地」）
  float  _targetXDeg{0.0f};
  float  _targetYDeg{0.0f};

  // 前回 update() の呼び出し時刻（補間ステップ計算用）
  unsigned long _lastUpdateMs{0};

  // ── 内部ヘルパー ─────────────────────────────────────────────────────

  /**
   * @brief 論理角度を 1 軸のサーボに書き込みます。
   * center_offset_deg → invert → パルス幅変換 の順で適用します。
   *
   * @param pin    GPIO ピン番号
   * @param logicalDeg 論理角度（min/max クランプ済みであること）
   * @param calib  適用する校正値
   */
  void writeServo(int pin, float logicalDeg, const ServoAxisCalibration& calib);

  /**
   * @brief 論理角度を [min_deg, max_deg] でクランプします。
   * クランプが発生した場合は warning ログを出力します。
   *
   * @param deg    クランプ対象の角度
   * @param calib  適用する校正値
   * @param label  ログ出力用の軸名称（"X" または "Y"）
   * @return クランプ後の角度
   */
  float clamp(float deg, const ServoAxisCalibration& calib, const char* label) const;

  /**
   * @brief Preferences から両軸の校正値を読み出します。
   * 保存値が存在しない場合はデフォルト値を使用します。
   */
  void loadCalibration();

  /**
   * @brief サーボ校正ストア（NVS read/write API）。
   */
  ServoCalibrationStore _calibrationStore;

  /**
   * @brief 論理角度をサーボパルス幅（マイクロ秒）へ変換します。
   *
   * 変換式:
   *   physical_deg = (logicalDeg + center_offset_deg) * (invert ? -1 : 1)
   *   us = 1500 + (physical_deg / 90.0) * 1000   [範囲: 500〜2500 us]
   *
   * @param logicalDeg  校正前の論理角度（min/max クランプ済み）
   * @param calib       適用する校正値
   * @return パルス幅（マイクロ秒）
   */
  int logicalToMicros(float logicalDeg, const ServoAxisCalibration& calib) const;
};

}  // namespace Actuator
