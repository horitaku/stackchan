/**
 * @file envelope.cpp
 * @brief WebSocket プロトコルエンベロープの生成実装
 */
#include "envelope.h"
#include "events.h"

#include <ArduinoJson.h>
#include <time.h>
#include <esp_random.h>

namespace Protocol {

String nowRfc3339() {
  time_t t = time(nullptr);

  // time() が 0 付近を返す場合は NTP 未同期（起動直後など）です。
  // フォールバックとして起動からの経過時間を 1970-01-01T からの秒数として扱います。
  if (t < 1000000000L) {
    unsigned long ms = millis();
    unsigned long totalSec = ms / 1000;
    unsigned long hour = totalSec / 3600 % 24;
    unsigned long min  = totalSec / 60   % 60;
    unsigned long sec  = totalSec        % 60;
    char buf[25];
    snprintf(buf, sizeof(buf), "1970-01-01T%02lu:%02lu:%02luZ", hour, min, sec);
    return String(buf);
  }

  // NTP 同期済み: gmtime で UTC 変換します
  struct tm tm_info;
  gmtime_r(&t, &tm_info);
  char buf[25];
  strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%SZ", &tm_info);
  return String(buf);
}

String generateUUIDv4() {
  // ESP32 ハードウェア乱数で 128bit を生成します
  uint8_t uuid[16];
  for (int i = 0; i < 4; i++) {
    uint32_t r = esp_random();
    uuid[i * 4 + 0] = (r >> 24) & 0xFF;
    uuid[i * 4 + 1] = (r >> 16) & 0xFF;
    uuid[i * 4 + 2] = (r >>  8) & 0xFF;
    uuid[i * 4 + 3] =  r        & 0xFF;
  }

  // バージョン 4 のビットをセットします（byte[6] の上位 4bit = 0100b）
  uuid[6] = (uuid[6] & 0x0F) | 0x40;
  // バリアント 1 のビットをセットします（byte[8] の上位 2bit = 10b）
  uuid[8] = (uuid[8] & 0x3F) | 0x80;

  // "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx" 形式に整形します
  char buf[37];
  snprintf(buf, sizeof(buf),
    "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
    uuid[0], uuid[1], uuid[2], uuid[3],
    uuid[4], uuid[5],
    uuid[6], uuid[7],
    uuid[8], uuid[9],
    uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15]);

  return String(buf);
}

String buildEnvelope(const char* type, const String& sessionId,
                     int64_t seq, const String& payloadJson) {
  JsonDocument doc;
  doc["type"]       = type;
  doc["timestamp"]  = nowRfc3339();
  doc["session_id"] = sessionId;
  doc["sequence"]   = seq;
  doc["version"]    = VERSION;

  // ペイロード JSON 文字列をパースしてオブジェクトとして埋め込みます
  JsonDocument payloadDoc;
  DeserializationError err = deserializeJson(payloadDoc, payloadJson);
  if (!err) {
    doc["payload"] = payloadDoc;
  } else {
    // パース失敗時は空オブジェクトを設定します（後続処理でエラー検知されます）
    doc["payload"].to<JsonObject>();
    Serial.printf("[Envelope] payload parse error: %s\n", err.c_str());
  }

  String result;
  serializeJson(doc, result);
  return result;
}

}  // namespace Protocol
