/**
 * @file touch_service.cpp
 * @brief タッチ入力サービス実装（P11-04）
 */
#include "touch_service.h"

#include <M5Unified.h>

namespace Input {

void TouchService::begin() {
  _initialized = true;
  _clickedLatched = false;
  _clickArmed = false;
  _snapshot = TouchSnapshot{};
  Serial.println("[Touch] TouchService ready");
}

void TouchService::update() {
  if (!_initialized) return;

  _snapshot.clicked = false;
  _snapshot.timestampMs = millis();

  if (M5.Touch.getCount() <= 0) {
    _snapshot.touching = false;
    _snapshot.x = -1;
    _snapshot.y = -1;
    // 起動直後や押下継続状態での誤検出を避けるため、
    // 一度「未タッチ状態」を観測してからクリック判定を有効化します。
    _clickArmed = true;
    return;
  }

  auto detail = M5.Touch.getDetail(0);
  _snapshot.touching = true;
  _snapshot.x = detail.x;
  _snapshot.y = detail.y;

  if (_clickArmed && detail.wasClicked()) {
    _snapshot.clicked = true;
    _clickedLatched = true;
  }
}

bool TouchService::consumeClicked() {
  if (!_clickedLatched) {
    return false;
  }
  _clickedLatched = false;
  return true;
}

}  // namespace Input
