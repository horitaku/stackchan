/**
 * @file touch_service.h
 * @brief タッチ入力サービス（P11-04）
 *
 * StackchanSession から M5.Touch 直接依存を外すための薄いサービスです。
 * begin()/update() を呼び出し、consumeClicked() でワンショットのタップを取得します。
 */
#pragma once

#include <Arduino.h>

namespace Input {

/**
 * @brief タッチ状態のスナップショットです。
 */
struct TouchSnapshot {
  bool touching{false};
  bool clicked{false};
  int x{-1};
  int y{-1};
  unsigned long timestampMs{0};
};

/**
 * @brief M5.Touch の入力状態を取り扱うサービスです。
 */
class TouchService {
 public:
  TouchService() = default;

  /**
   * @brief サービスを初期化します。
   */
  void begin();

  /**
   * @brief 現在のタッチ状態を更新します。
   *
   * @note M5.update() の後に呼び出してください。
   */
  void update();

  /**
   * @brief クリックイベントを 1 回だけ消費します。
   * @return クリックを消費した場合 true
   */
  bool consumeClicked();

  /**
   * @brief 最新スナップショットを返します。
   */
  const TouchSnapshot& snapshot() const { return _snapshot; }

 private:
  bool _initialized{false};
  bool _clickedLatched{false};
  bool _clickArmed{false};
  TouchSnapshot _snapshot;
};

}  // namespace Input
