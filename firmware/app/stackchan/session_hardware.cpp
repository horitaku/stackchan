/**
 * @file session_hardware.cpp
 * @brief StackchanSession のハードウェア制御コマンドハンドラー実装（P11-08）
 *
 * ## 責務
 * - device.servo.* 受信イベントを ServoController へ委譲する
 * - device.servo.calibration.response を server へ送信する
 * - ハードウェア操作エラーを error イベントとして server へ通知する
 *
 * ## 設計方針
 * - ハンドラーは入力バリデーション → サービス委譲 → エラー通知のみ担当する
 * - ServoController の内部ロジック（クランプ・変換・保存）はここに書かない
 * - session.h の不変条件（hello/welcome / TTS フロー等）を一切変更しない
 *
 * ## ルーティング
 * session_protocol.cpp の onTextMessage() が payloadRoutes テーブルで
 * 各ハンドラーへディスパッチします。新しいデバイスイベントを追加する場合は
 * そのテーブルにエントリを追加してください。
 */
#include "session.h"
#include <ArduinoJson.h>

namespace App {

// ──────────────────────────────────────────────────────────────────────
// 共通エラー送信ヘルパー
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief hardware コマンドのエラーを error イベントとして server へ通知します。
 *
 * @param requestId  元コマンドの request_id（相関 ID）
 * @param code       エラーコード文字列（protocol/websocket/events.md §4.3 参照）
 * @param message    人間が読めるエラーメッセージ
 * @param retryable  再試行可能フラグ
 */
void StackchanSession::sendDeviceError(
    const String& requestId, const char* code, const char* message, bool retryable) {

  JsonDocument payload;
  payload["code"]       = code;
  payload["message"]    = message;
  payload["retryable"]  = retryable;
  // device コマンド由来を識別しやすくするため request_id を含めます。
  if (requestId.length() > 0) {
    payload["request_id"] = requestId;
  }

  String payloadStr;
  serializeJson(payload, payloadStr);

  String env = Protocol::buildEnvelope(
    Protocol::EventType::ERROR_EVENT, _sessionId, _seq.next(), payloadStr);

  _ws.sendText(env);
  Serial.printf("[HWCmd] error sent code=%s message=%s\n", code, message);
}

// ──────────────────────────────────────────────────────────────────────
// device.servo.move ハンドラー
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief device.servo.move イベントを ServoController::move() へ委譲します。
 *
 * @section バリデーション
 * - axis は "x" / "y" / "both" のいずれか（必須）
 * - axis=x のとき angle_x_deg が存在すること
 * - axis=y のとき angle_y_deg が存在すること
 * - axis=both のとき angle_x_deg と angle_y_deg の両方が存在すること
 *
 * @section エラー時の動作
 * 必須フィールド不足時は invalid_payload を server へ送信し、サーボは動かしません。
 * ServoController 内部でクランプが発生した場合は warning ログのみです（エラー送信なし）。
 */
void StackchanSession::handleDeviceServoMove(const String& payloadJson) {
  if (!_servo.isReady()) {
    Serial.println("[HWCmd] servo.move: servo not ready");
    sendDeviceError("", "device_error", "servo not ready", false);
    return;
  }

  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* axis      = payload["axis"] | "";

  // axis の必須チェックと enum バリデーション
  const bool isX    = (strcmp(axis, "x") == 0);
  const bool isY    = (strcmp(axis, "y") == 0);
  const bool isBoth = (strcmp(axis, "both") == 0);

  if (!isX && !isY && !isBoth) {
    Serial.printf("[HWCmd] servo.move: invalid axis='%s'\n", axis);
    sendDeviceError(requestId, "invalid_payload",
      "axis must be 'x', 'y', or 'both'", false);
    return;
  }

  // axis 別の必須フィールドチェック
  if ((isX || isBoth) && !payload["angle_x_deg"].is<float>()) {
    sendDeviceError(requestId, "invalid_payload",
      "angle_x_deg required when axis=x or both", false);
    return;
  }
  if ((isY || isBoth) && !payload["angle_y_deg"].is<float>()) {
    sendDeviceError(requestId, "invalid_payload",
      "angle_y_deg required when axis=y or both", false);
    return;
  }

  const float angleXDeg  = payload["angle_x_deg"] | 0.0f;
  const float angleYDeg  = payload["angle_y_deg"] | 0.0f;
  const float speedScale = payload["speed"]        | 1.0f;

  Serial.printf("[HWCmd] servo.move: axis=%s x=%.1f y=%.1f speed=%.2f request_id=%s\n",
    axis, angleXDeg, angleYDeg, speedScale, requestId);

  // ServoController に委譲します（クランプ・変換・書き込みはサービス側が担当）。
  _servo.move(axis, angleXDeg, angleYDeg, speedScale);
}

// ──────────────────────────────────────────────────────────────────────
// device.servo.calibration.get ハンドラー
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief device.servo.calibration.get を受信し、現在の校正値を response として返します。
 *
 * ServoController から校正値と現在角度を取得して
 * device.servo.calibration.response を server へ送信します。
 *
 * request_id は response で mirror して返すため必須です。
 */
void StackchanSession::handleDeviceServoCalibrationGet(const String& payloadJson) {
  if (!_servo.isReady()) {
    Serial.println("[HWCmd] calibration.get: servo not ready");
    sendDeviceError("", "device_error", "servo not ready", false);
    return;
  }

  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  if (strlen(requestId) == 0) {
    sendDeviceError("", "invalid_payload",
      "request_id required for calibration.get", false);
    return;
  }

  Serial.printf("[HWCmd] calibration.get: request_id=%s\n", requestId);

  // ServoController から校正値と現在角度を取得して response を送信します。
  sendServoCalibrationResponse(String(requestId));
}

// ──────────────────────────────────────────────────────────────────────
// device.servo.calibration.set ハンドラー
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief device.servo.calibration.set を受信し、差分更新 + 不揮発保存を行います。
 *
 * @section 差分更新セマンティクス
 * 省略されたフィールドは現在の校正値を保持します（ゼロリセット防止）。
 *
 * @section バリデーション
 * - axis は "x" または "y" のみ（"both" は不可）
 * - min_deg < max_deg（設定された場合）
 *
 * @section エラー時の動作
 * - バリデーション失敗: invalid_payload を送信して保存しない
 * - Preferences 書き込み失敗: device_error (retryable=true) を送信
 */
void StackchanSession::handleDeviceServoCalibrationSet(const String& payloadJson) {
  if (!_servo.isReady()) {
    Serial.println("[HWCmd] calibration.set: servo not ready");
    sendDeviceError("", "device_error", "servo not ready", false);
    return;
  }

  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* requestId = payload["request_id"] | "";
  const char* axis      = payload["axis"] | "";

  // axis バリデーション（"both" は calibration.set では不可）
  const bool isX = (strcmp(axis, "x") == 0);
  const bool isY = (strcmp(axis, "y") == 0);
  if (!isX && !isY) {
    Serial.printf("[HWCmd] calibration.set: invalid axis='%s'\n", axis);
    sendDeviceError(requestId, "invalid_payload",
      "axis must be 'x' or 'y' for calibration.set", false);
    return;
  }

  // 対象軸の現在の校正値をベースに差分更新します（省略フィールドは現在値を使用）。
  Actuator::ServoAxisCalibration newCal =
    isX ? _servo.calibrationX() : _servo.calibrationY();

  // 送られてきたフィールドのみを更新します。
  if (payload["center_offset_deg"].is<float>()) {
    newCal.center_offset_deg = payload["center_offset_deg"].as<float>();
  }
  if (payload["min_deg"].is<float>()) {
    newCal.min_deg = payload["min_deg"].as<float>();
  }
  if (payload["max_deg"].is<float>()) {
    newCal.max_deg = payload["max_deg"].as<float>();
  }
  if (payload["invert"].is<bool>()) {
    newCal.invert = payload["invert"].as<bool>();
  }
  if (payload["speed_limit_deg_per_sec"].is<float>()) {
    newCal.speed_limit_deg_per_sec = payload["speed_limit_deg_per_sec"].as<float>();
  }
  if (payload["soft_start"].is<bool>()) {
    newCal.soft_start = payload["soft_start"].as<bool>();
  }
  if (payload["home_deg"].is<float>()) {
    newCal.home_deg = payload["home_deg"].as<float>();
  }

  // min_deg < max_deg の制約をここでも検証します（ServoController 内でも再検証）。
  if (newCal.min_deg >= newCal.max_deg) {
    Serial.printf("[HWCmd] calibration.set: min_deg(%.1f) >= max_deg(%.1f)\n",
      newCal.min_deg, newCal.max_deg);
    sendDeviceError(requestId, "invalid_payload",
      "min_deg must be less than max_deg", false);
    return;
  }

  Serial.printf("[HWCmd] calibration.set: axis=%s center_off=%.1f min=%.1f max=%.1f invert=%d speed=%.1f home=%.1f request_id=%s\n",
    axis, newCal.center_offset_deg, newCal.min_deg, newCal.max_deg,
    newCal.invert, newCal.speed_limit_deg_per_sec, newCal.home_deg, requestId);

  // ServoController に委譲して保存します。
  const bool ok = isX
    ? _servo.setCalibrationX(newCal)
    : _servo.setCalibrationY(newCal);

  if (!ok) {
    // Preferences への書き込み失敗: 再試行可能として通知します。
    sendDeviceError(requestId, "device_error",
      "servo calibration save failed", /*retryable=*/true);
  }
  // 成功時は ack を返さず（fire-and-forget）、保存後すぐに設定を反映します。
  // エラーがない場合はサイレント成功です。
}

// ──────────────────────────────────────────────────────────────────────
// device.servo.calibration.response 送信
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief device.servo.calibration.response を firmware → server へ送信します。
 *
 * ServoController から X/Y 両軸の校正値・現在角度を取得して JSON に変換します。
 * calibration.get のハンドラーから呼ばれます。
 *
 * @param requestId  元要求の request_id（mirror して返す）
 */
void StackchanSession::sendServoCalibrationResponse(const String& requestId) {
  const auto& cx = _servo.calibrationX();
  const auto& cy = _servo.calibrationY();

  // X 軸の校正値オブジェクトを構築します。
  JsonDocument payload;
  payload["request_id"] = requestId;

  // --- X 軸 ---
  JsonObject x = payload["x"].to<JsonObject>();
  x["center_offset_deg"]         = cx.center_offset_deg;
  x["min_deg"]                   = cx.min_deg;
  x["max_deg"]                   = cx.max_deg;
  x["invert"]                    = cx.invert;
  x["speed_limit_deg_per_sec"]   = cx.speed_limit_deg_per_sec;
  x["soft_start"]                = cx.soft_start;
  x["home_deg"]                  = cx.home_deg;

  // --- Y 軸 ---
  JsonObject y = payload["y"].to<JsonObject>();
  y["center_offset_deg"]         = cy.center_offset_deg;
  y["min_deg"]                   = cy.min_deg;
  y["max_deg"]                   = cy.max_deg;
  y["invert"]                    = cy.invert;
  y["speed_limit_deg_per_sec"]   = cy.speed_limit_deg_per_sec;
  y["soft_start"]                = cy.soft_start;
  y["home_deg"]                  = cy.home_deg;

  // --- 現在角度（診断用オプション） ---
  payload["current_angle_x_deg"] = _servo.currentAngleXDeg();
  payload["current_angle_y_deg"] = _servo.currentAngleYDeg();

  String payloadStr;
  serializeJson(payload, payloadStr);

  String env = Protocol::buildEnvelope(
    Protocol::EventType::DEVICE_SERVO_CALIBRATION_RESPONSE,
    _sessionId, _seq.next(), payloadStr);

  if (_ws.sendText(env)) {
    Serial.printf("[HWCmd] calibration.response sent request_id=%s\n", requestId.c_str());
  } else {
    Serial.printf("[HWCmd] calibration.response send failed request_id=%s\n", requestId.c_str());
  }
}

}  // namespace App
