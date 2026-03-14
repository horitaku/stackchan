/**
 * @file wifi.h
 * @brief Wi-Fi 接続管理モジュール
 * 
 * FW_WIFI_SSID / FW_WIFI_PASSWORD（include/secrets.h で定義）を使って
 * Wi-Fi に接続します。NTP 同期も合わせて実施します。
 * 
 * セキュリティ注意:
 *   - FW_WIFI_PASSWORD はシリアルログへの平文出力を禁止します。
 *   - FW_WIFI_SSID は必要最小限の長さでのみ表示します。
 */
#pragma once

#include <Arduino.h>

namespace Network {

/**
 * @brief Wi-Fi 接続状態を表します。
 */
enum class WiFiState {
  Disconnected,  ///< 切断済み
  Connecting,    ///< 接続試行中
  Connected,     ///< 接続済み
  Failed,        ///< 接続失敗（タイムアウト等）
};

/**
 * @brief Wi-Fi に接続します。接続後に NTP で時刻を同期します。
 * @param timeoutMs 接続タイムアウト（ミリ秒）、デフォルト 15 秒
 * @return 接続成功なら true
 */
bool connect(unsigned long timeoutMs = 15000);

/**
 * @brief 現在の接続状態を返します。
 */
bool isConnected();

/**
 * @brief Wi-Fi 接続を切断します。
 */
void disconnect();

/**
 * @brief 現在の Wi-Fi 状態を返します。
 */
WiFiState getState();

/**
 * @brief 現在の RSSI（信号強度）を dBm で返します。
 * 接続中でない場合は 0 を返します。
 */
int getRSSI();

}  // namespace Network
