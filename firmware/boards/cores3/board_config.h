/**
 * @file board_config.h
 * @brief M5Stack CoreS3 固有のボード初期化
 * 
 * CoreS3 に固有のピン定義・初期設定を集約します。
 * 他のボード（Core2 等）に移植する場合はこのファイルのみ差し替えてください。
 */
#pragma once

#include <M5Unified.h>

namespace Board {

  /**
   * @brief CoreS3 ボードの初期化を行います。
   * M5Unified の初期化・シリアル開始・ディスプレイ設定を実施します。
   * setup() の先頭で 1 度呼び出してください。
   */
  void init();

}  // namespace Board
