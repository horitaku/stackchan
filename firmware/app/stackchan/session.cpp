/**
 * @file session.cpp
 * @brief Stackchan セッション・オーケストレーション実装
 * 
 * P5-06: session.hello/welcome フロー
 * P5-07: heartbeat 送受信
 * P5-08: 最小音声送信フロー（audio.stream_open → binary → audio.end）
 * P5-09: stt.final / tts.end 受信とデバッグログ
 * P11-08: hardware command router 追加（ServoController 統合）
 */
#include "session.h"
#include <ArduinoJson.h>
#include <M5Unified.h>

#ifndef FW_TOUCH_DEBUG_AUDIO_ENABLED
#define FW_TOUCH_DEBUG_AUDIO_ENABLED 0
#endif

namespace App {

namespace {
constexpr unsigned long kDebugAudioTriggerCooldownMs = 2500;
}

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

  // P11-08: サーボコントローラを初期化します。
  // GPIO ピンは FW_SERVO_X_PIN / FW_SERVO_Y_PIN のビルドフラグで指定します（デフォルト: 1, 2）。
  // Preferences から校正値を自動復元し、起動時にホーム位置へ移動します。
  _servo.begin(FW_SERVO_X_PIN, FW_SERVO_Y_PIN);

  // P11-03: M5GO Bottom3 LED と NECO MIMI NeoPixel を初期化します。
  // FW_NECO_MIMI_ENABLED=0 の場合、_ear.begin() は何もしません（オプションHW）。
  _led.begin(FW_LED_NUM);
  _ear.begin(FW_NECO_MIMI_NUM_LEDS);
  _touch.begin();
  _camera.begin();

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

  // ── Input: タッチ入力更新（P11-04） ─────────────────────────────────────
  _touch.update();
#if FW_TOUCH_DEBUG_AUDIO_ENABLED
  if (_touch.consumeClicked()) {
    const unsigned long now = millis();

    if (_conversationState != ConversationState::Idle) {
      Serial.printf("[Session] touch debug trigger ignored: conversation_state=%s\n",
        conversationStateName(_conversationState));
    } else if ((now - _lastDebugAudioTriggerMs) < kDebugAudioTriggerCooldownMs) {
      Serial.printf("[Session] touch debug trigger ignored: cooldown=%lu ms remaining\n",
        kDebugAudioTriggerCooldownMs - (now - _lastDebugAudioTriggerMs));
    } else {
      _lastDebugAudioTriggerMs = now;
      Serial.println("[Session] touch clicked -> sendAudioStream() for debug");
      sendAudioStream(50);
    }
  }
#endif

  // ── TTS 再生状態の更新 ────────────────────────────────────────────
  _ttsPlayer.update();
  // ── P11-08: サーボ補間処理 ──────────────────────────────────
  // soft_start が有効な場合、目標角度へ向けて況渡に角度を進めます。
  _servo.update();  // ── P11-03: LED アニメーション更新 ────────────────────────
  // blink / breathe / rainbow のアニメーションを進めます。
  _led.update();
  _ear.update();  // ── Consumer: キューから消費・再生 ────────────────────────────────────
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

}  // namespace App
