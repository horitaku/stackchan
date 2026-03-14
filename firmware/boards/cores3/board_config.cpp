/**
 * @file board_config.cpp
 * @brief M5Stack CoreS3 固有のボード初期化実装
 */
#include "board_config.h"

namespace Board {

void init() {
  // M5Unified の初期化設定を構築します
  auto cfg = M5.config();
  // シリアルボーレートを platformio.ini の monitor_speed に合わせます
  cfg.serial_baudrate = 115200;

  // M5Stack CoreS3 の初期化を実行します（Display、Touch、Speaker、Mic 等）
  M5.begin(cfg);

  // 起動メッセージをシリアルに出力します
  Serial.println("[Board] M5Stack CoreS3 initialized");
  Serial.printf("[Board]   Display: %dx%d\n",
    M5.Display.width(), M5.Display.height());

  // ディスプレイにタイトルを表示します（デバッグ用）
  M5.Display.setTextSize(2);
  M5.Display.setTextColor(TFT_WHITE, TFT_BLACK);
  M5.Display.fillScreen(TFT_BLACK);
  M5.Display.setCursor(0, 0);
  M5.Display.println("Stackchan");
  M5.Display.setTextSize(1);
  M5.Display.println("Connecting...");
}

}  // namespace Board
