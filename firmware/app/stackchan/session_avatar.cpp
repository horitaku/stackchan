/**
 * @file session_avatar.cpp
 * @brief アバター表示・モーション演出・会話状態名称の実装
 *
 * P12-07: session.cpp から Avatar / motion / setConversationState 系を分離しました。
 * public API（session.h）は変更しません。
 *
 * 配置関数:
 *   - setConversationState()      : 会話状態遷移（ログ付き）
 *   - conversationStateName()     : 状態名を文字列で返すユーティリティ
 *   - handleAvatarExpression()    : avatar.expression イベント処理
 *   - handleMotionPlay()          : motion.play イベント処理（最小演出）
 *   - updateAvatarFace()          : loop() から呼ばれる口パク・表示更新
 *   - toAvatarExpression()        : 文字列 → m5avatar::Expression 変換
 */
#include "session.h"
#include <ArduinoJson.h>
#include <M5Unified.h>

namespace App {

// ──────────────────────────────────────────────────────────────────────
// 会話状態制御
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief 会話状態を遷移させます。
 *
 * 同一状態への遷移は無視します。
 * 遷移ログには遷移元・遷移先・理由を含めます。
 *
 * @param next   遷移先の状態
 * @param reason ログ出力用の理由文字列（nullptr 可）
 */
void StackchanSession::setConversationState(ConversationState next, const char* reason) {
  if (_conversationState == next) {
    return;
  }
  Serial.printf("[Conversation] State: %s -> %s reason=%s\n",
                conversationStateName(_conversationState),
                conversationStateName(next),
                reason == nullptr ? "(none)" : reason);
  _conversationState = next;
}

/**
 * @brief 会話状態を文字列表現で返します。
 *
 * ログ出力やデバッグ表示に使用します。
 *
 * @param state 対象の会話状態
 * @return 状態名の文字列リテラル
 */
const char* StackchanSession::conversationStateName(ConversationState state) const {
  switch (state) {
    case ConversationState::Idle:
      return "idle";
    case ConversationState::Listening:
      return "listening";
    case ConversationState::Thinking:
      return "thinking";
    case ConversationState::Speaking:
      return "speaking";
    case ConversationState::Interrupted:
      return "interrupted";
    case ConversationState::Error:
      return "error";
    default:
      return "unknown";
  }
}

// ──────────────────────────────────────────────────────────────────────
// イベントハンドラ
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief avatar.expression イベントを処理します。
 *
 * payload から expression を取得し、アバターの表情を更新します。
 * _avatarReady が false の場合は描画をスキップします。
 *
 * @param payloadJson イベントの payload JSON 文字列
 */
void StackchanSession::handleAvatarExpression(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* expression = payload["expression"] | "neutral";
  _expression = String(expression);
  if (_avatarReady) {
    _avatar.setExpression(toAvatarExpression(_expression));
  }
  Serial.printf("[Avatar] expression=%s\n", _expression.c_str());
}

/**
 * @brief motion.play イベントを処理します。
 *
 * フェーズ 6 では安全な最小モーション（ビープ音 + 首回転）のみ実装します。
 * _avatarReady が false の場合は描画をスキップします。
 *
 * @param payloadJson イベントの payload JSON 文字列
 */
void StackchanSession::handleMotionPlay(const String& payloadJson) {
  JsonDocument payload;
  deserializeJson(payload, payloadJson);

  const char* motion = payload["motion"] | "idle";
  _motion = String(motion);

  // フェーズ 6 では安全な最小モーション（通知 + 軽いビープ）のみ実装します。
  if (_motion == "nod") {
    M5.Speaker.tone(900, 40);
  } else if (_motion == "shake") {
    M5.Speaker.tone(700, 40);
  }

  if (_avatarReady) {
    if (_motion == "nod") {
      _avatar.setRotation(0.10f);
    } else if (_motion == "shake") {
      _avatar.setRotation(-0.10f);
    } else {
      _avatar.setRotation(0.0f);
    }
  }

  Serial.printf("[Avatar] motion=%s\n", _motion.c_str());
}

// ──────────────────────────────────────────────────────────────────────
// 定期更新（loop() から呼ばれます）
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief 口パク・表情・回転を定期更新します。
 *
 * 更新は 80ms 周期に制限し、描画処理の CPU 負荷を抑制します。
 * 再生中は口パク比率を _ttsPlayer.lipLevel() から取得して反映します。
 * モーション演出後は回転を自然復帰させます。
 */
void StackchanSession::updateAvatarFace() {
  // 更新は 80ms 周期に制限します（描画処理の負荷抑制）。
  const unsigned long now = millis();
  if (now - _lastAvatarRenderMs < 80) {
    return;
  }
  _lastAvatarRenderMs = now;

  if (!_avatarReady) {
    return;
  }

  const float lip = _ttsPlayer.lipLevel();
  _avatar.setMouthOpenRatio(lip);

  // 再生中のみ口パクメタ情報を表示し、待機時は最小ラベル表示に戻します。
  if (_ttsPlayer.state() == Audio::PlaybackState::Playing) {
    String speech = String("Req:") + _currentRequestId;
    _avatar.setSpeechText(speech.c_str());
  } else {
    _avatar.setSpeechText(_expression.c_str());
  }

  // 回転は毎周期で減衰させ、モーション演出後に自然復帰させます。
  if (_motion == "nod" || _motion == "shake") {
    _avatar.setRotation(0.0f);
    _motion = "idle";
  }
}

// ──────────────────────────────────────────────────────────────────────
// 変換ユーティリティ
// ──────────────────────────────────────────────────────────────────────

/**
 * @brief 文字列表現の表情名を m5avatar::Expression へ変換します。
 *
 * 未知の表情名は Neutral にフォールバックします。
 *
 * @param expression 表情名文字列（例: "happy", "sad", "surprised", "angry"）
 * @return 対応する m5avatar::Expression 値
 */
m5avatar::Expression StackchanSession::toAvatarExpression(const String& expression) const {
  if (expression == "happy") {
    return m5avatar::Expression::Happy;
  }
  if (expression == "sad") {
    return m5avatar::Expression::Sad;
  }
  if (expression == "surprised") {
    return m5avatar::Expression::Doubt;
  }
  if (expression == "angry") {
    return m5avatar::Expression::Angry;
  }
  return m5avatar::Expression::Neutral;
}

}  // namespace App
