/**
 * @file main.cpp
 * @brief Stackchan firmware エントリーポイント
 * 
 * M5Stack CoreS3 向けのファームウェアです。
 * Wi-Fi 接続 → WebSocket 接続 → セッション確立 → 音声送受信 の一連のフローを管理します。
 * 
 * 操作方法:
 *   - タッチスクリーンをタップ: TouchService 経由でテスト音声ストリームを送信します（デバッグ用）
 */
#include <Arduino.h>
#include "boards/cores3/board_config.h"
#include "app/stackchan/session.h"

// Stackchan セッション管理インスタンス
App::StackchanSession stackchan;

/**
 * @brief 初期化処理。起動時に 1 度だけ呼び出されます。
 */
void setup() {
  // ボード固有の初期化（M5Unified 初期化・シリアル開始）
  Board::init();
  delay(500);  // 起動安定待ち

  Serial.println("=== Stackchan Firmware v0.5.0 ===");
  Serial.printf("  Device ID : %s\n", FW_DEVICE_ID);
  Serial.printf("  WS URL    : %s\n", FW_WS_URL);
  Serial.printf("  Log Level : %s\n", FW_LOG_LEVEL);

  // セッション接続を開始します（Wi-Fi → WebSocket → hello/welcome）
  stackchan.begin();
}

/**
 * @brief メインループ処理。無限に繰り返し呼び出されます。
 */
void loop() {
  // M5Stack の入力状態を更新します（タッチ・ボタン等）
  M5.update();

  // セッション状態の維持と heartbeat 送信を行います
  stackchan.loop();
}
