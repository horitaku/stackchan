/**
 * @file session.cpp
 * @brief Stackchan セッション・オーケストレーション実装
 * 
 * P5-06: session.hello/welcome フロー
 * P5-07: heartbeat 送受信
 * P5-08: 最小音声送信フロー（audio.stream_open → binary → audio.end）
 * P5-09: stt.final / tts.end 受信とデバッグログ
 */
#include "session.h"
#include <ArduinoJson.h>
#include <M5Unified.h>

namespace App {

StackchanSession::StackchanSession()
    : _mic(FW_AUDIO_SAMPLE_RATE, FW_AUDIO_FRAME_MS) {}

// ──────────────────────────────────────────────────────────────────────
// Public: begin / loop
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::begin() {
  Serial.println("[Session] begin()");
  setConversationState(ConversationState::Idle, "boot completed");

  _mic.begin();
  _ttsPlayer.begin();

  // P8-07: M5Stack-Avatar の顔描画を開始します。
  _avatar.init();
  _avatar.setExpression(m5avatar::Expression::Neutral);
  _avatar.setSpeechText("Connecting...");
  _avatarReady = true;

  // WebSocket クライアントの設定
  _ws.setUrl(String(FW_WS_URL));
  _ws.setReconnectPolicy({FW_RECONNECT_BASE_MS, FW_RECONNECT_MAX_MS});

  // FW_WS_TOKEN が設定されていれば Authorization ヘッダに付与します（値はログ非出力）
#ifdef FW_WS_TOKEN
  {
    String token = String(FW_WS_TOKEN);
    if (token.length() > 0 && token != "OPTIONAL_WS_TOKEN") {
      _ws.setToken(token);
    }
  }
#endif

  // コールバック登録
  _ws.onConnected([this]() { onWSConnected(); });
  _ws.onDisconnected([this]() { onWSDisconnected(); });
  _ws.onTextMessage([this](const String& msg) { onTextMessage(msg); });

  // Wi-Fi 接続を開始します
  setState(SessionState::ConnectingWiFi);
  if (Network::connect()) {
    setState(SessionState::ConnectingWS);
    _ws.begin();
  } else {
    // 接続失敗: loop() 内で指数バックオフ再試行します
    Serial.println("[Session] WiFi connect failed at startup, will retry in loop()");
  }
}

void StackchanSession::loop() {
  // Wi-Fi 未接続時: 指数バックオフで再接続を試みます
  if (_state == SessionState::Idle || _state == SessionState::ConnectingWiFi) {
    if (!Network::isConnected()) {
      if (millis() - _lastWiFiAttemptMs >= _wifiRetryDelayMs) {
        _lastWiFiAttemptMs = millis();
        if (Network::connect(5000)) {
          setState(SessionState::ConnectingWS);
          _ws.begin();
          // 次回待機時間をリセットします
          _wifiRetryDelayMs = FW_RECONNECT_BASE_MS;
        } else {
          // 指数バックオフ: 最大 FW_RECONNECT_MAX_MS までキャップします
          _wifiRetryDelayMs = min(_wifiRetryDelayMs * 2,
                                   static_cast<unsigned long>(FW_RECONNECT_MAX_MS));
          Serial.printf("[Session] WiFi retry delay → %lu ms\n", _wifiRetryDelayMs);
        }
      }
    }
    return;
  }

  // ┌─────────────────────────────────────────────────────────────────────────┐
  // │ P8-17: 受信・消費・表示の責務分離（Producer-Consumer pattern）          │
  // ├─────────────────────────────────────────────────────────────────────────┤
  // │ Producer（受信側）: _ws.loop()                                         │
  // │   → onTextMessage() → enqueueTTSFrame() で frame を enqueue            │
  // │   → Non-blocking: フレーム受信をキューに積み込むのみ                  │
  // │                                                                         │
  // │ Consumer（消費側）: processTTSPlaybackQueue()                          │
  // │   → キューから dequeue → 40ms 分を集約 → playPCM16() で再生          │
  // │   → キュー watermark 監視（prebuffer / low-water / high-water）       │
  // │   → observability: バッファ深さ、滞留時間をログ出力                   │
  // │                                                                         │
  // │ 効果: 受信遅延が再生に直接影響しない → より安定したストリーミング      │
  // └─────────────────────────────────────────────────────────────────────────┘

  // ── Producer: WebSocket ノンブロッキング受信 ──────────────────────────────
  // 受信フレーム到達時に自動的に onTextMessage() → enqueueTTSFrame() が実行されます
  _ws.loop();

  // ── TTS 再生状態の更新 ────────────────────────────────────────────
  _ttsPlayer.update();

  // ── Consumer: キューから消費・再生 ────────────────────────────────────
  // processTTSPlaybackQueue() はキュー監視・watermark チェック・dequeue を担当します
  // 受信フロー（producer）と分離されているため、互いに独立して進捗できます
  processTTSPlaybackQueue();

  // ── Display: 口パク同期・顔表情更新 ────────────────────────────────────
  updateAvatarFace();

  // ── 状態遷移: speaking -> idle の検知 ────────────────────────────────────
  const Audio::PlaybackState nowPlaybackState = _ttsPlayer.state();
  if (_lastPlaybackState == Audio::PlaybackState::Playing &&
      nowPlaybackState == Audio::PlaybackState::Idle &&
      _conversationState == ConversationState::Speaking) {
    setConversationState(ConversationState::Idle, "tts playback finished");
  }
  _lastPlaybackState = nowPlaybackState;

  // ── 定期送信: heartbeat（Active 状態のみ） ────────────────────────────
  if (_state == SessionState::Active) {
    if (millis() - _lastHeartbeatMs >= _heartbeatIntervalMs) {
      sendHeartbeat();
    }
  }
}

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
