/**
 * @file envelope.h
 * @brief WebSocket プロトコルエンベロープの生成・ユーティリティ
 * 
 * 送信する全 JSON メッセージは必ず buildEnvelope() を通して構築します。
 * これにより type / timestamp / session_id / sequence / version / payload の
 * 共通フィールドが正しく付与されます。
 */
#pragma once

#include <Arduino.h>

namespace Protocol {

/**
 * @brief Firmware → Server 方向の送信シーケンスカウンタ。
 * セッション確立ごとに reset() を呼び出してください。
 */
class OutboundSequence {
 public:
  /// シーケンスカウンタをリセットします（再接続時に呼び出します）
  void reset() { _seq = 0; }
  /// 次のシーケンス番号を取得します（1 から開始）
  int64_t next() { return ++_seq; }
  int64_t current() const { return _seq; }

 private:
  int64_t _seq{0};
};

/**
 * @brief UTC 時刻を RFC3339 形式（YYYY-MM-DDTHH:MM:SSZ）で返します。
 * NTP 同期前は起動からの経過時間をベースとした近似値を返します。
 */
String nowRfc3339();

/**
 * @brief ESP32 ハードウェア乱数を用いて UUID v4 を生成します。
 * バイナリフレームの stream_id 生成に使用します。
 * @return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx" 形式の 36 文字文字列
 */
String generateUUIDv4();

/**
 * @brief 共通エンベロープ + payload を JSON 文字列として構築します。
 * 
 * @param type       イベントタイプ（EventType 定数を使用してください）
 * @param sessionId  セッション ID（session.hello 前は空文字列 ""）
 * @param seq        送信シーケンス番号（OutboundSequence::next() の戻り値）
 * @param payloadJson ペイロードの JSON 文字列（"{...}" 形式）
 * @return 完全なエンベロープ JSON 文字列
 */
String buildEnvelope(const char* type, const String& sessionId,
                     int64_t seq, const String& payloadJson);

}  // namespace Protocol
