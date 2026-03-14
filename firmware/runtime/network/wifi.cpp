/**
 * @file wifi.cpp
 * @brief Wi-Fi 接続管理モジュール実装
 */
#include "wifi.h"
#include <WiFi.h>
// Wi-Fi 認証情報は git 管理外の include/secrets.h に記述します
#include "secrets.h"  // FW_WIFI_SSID, FW_WIFI_PASSWORD

namespace Network {

bool connect(unsigned long timeoutMs) {
  // SSID のみ表示（パスワードはログに出力しません）
  Serial.printf("[WiFi] Connecting to SSID: %s\n", FW_WIFI_SSID);

  WiFi.mode(WIFI_STA);
  WiFi.begin(FW_WIFI_SSID, FW_WIFI_PASSWORD);

  unsigned long startMs = millis();
  int dotCount = 0;
  while (WiFi.status() != WL_CONNECTED) {
    if (millis() - startMs > timeoutMs) {
      Serial.println();
      Serial.printf("[WiFi] Connection timed out after %lu ms\n", timeoutMs);
      return false;
    }
    // 500ms ごとに進捗ドットを出力します
    delay(500);
    Serial.print(".");
    if (++dotCount % 40 == 0) Serial.println();
  }

  Serial.println();
  Serial.printf("[WiFi] Connected. IP: %s  RSSI: %d dBm\n",
    WiFi.localIP().toString().c_str(), WiFi.RSSI());

  // UTC で NTP 同期を開始します（非同期 — 同期完了は数秒後）
  configTime(0, 0, "pool.ntp.org", "time.cloudflare.com");
  Serial.println("[WiFi] NTP sync requested (UTC, pool.ntp.org)");

  return true;
}

bool isConnected() {
  return WiFi.status() == WL_CONNECTED;
}

void disconnect() {
  WiFi.disconnect(true);
  Serial.println("[WiFi] Disconnected");
}

WiFiState getState() {
  switch (WiFi.status()) {
    case WL_CONNECTED:   return WiFiState::Connected;
    case WL_CONNECT_FAILED:
    case WL_CONNECTION_LOST:
    case WL_DISCONNECTED: return WiFiState::Disconnected;
    default:             return WiFiState::Connecting;
  }
}

int getRSSI() {
  if (!isConnected()) return 0;
  return WiFi.RSSI();
}

}  // namespace Network
