/**
 * @file session.h
 * @brief Stackchan セッション・オーケストレーションクラス
 * 
 * Wi-Fi → WebSocket → session.hello/welcome → heartbeat → audio 送信 の
 * 一連のフローを管理するステートマシンです。
 * 
 * firmware の責務はデバイス I/O とプロトコル処理のみです。
 * AI オーケストレーション（STT/LLM/TTS 処理）はサーバー側が担います。
 * 
 * セッション状態遷移:
 *   Idle → ConnectingWiFi → ConnectingWS → Handshaking → Active
 *                ↑__________________________↑  （切断時に再接続）
 */
#pragma once

#include <Arduino.h>
#include <Avatar.h>
#include "../../runtime/network/wifi.h"
#include "../../runtime/network/ws_client.h"
#include "../../runtime/audio/mic_reader.h"
#include "../../runtime/audio/tts_player.h"
#include "../../protocol/envelope.h"
#include "../../protocol/events.h"

namespace App {

/**
 * @brief セッション状態を表します。
 */
enum class SessionState {
  Idle,           ///< 未起動
  ConnectingWiFi, ///< Wi-Fi 接続中（失敗時は指数バックオフで再試行）
  ConnectingWS,   ///< WebSocket 接続中（失敗時は自動再接続）
  Handshaking,    ///< session.hello 送信済み、welcome 待ち
  Active,         ///< 完全接続済み（heartbeat 送信・音声送受信が可能）
  Error           ///< 致命的エラー（welcome rejected 等）
};

/**
 * @brief Stackchan セッション管理クラス。
 * setup() で begin()、loop() で loop() を呼び出してください。
 */
class StackchanSession {
 public:
  StackchanSession();

  /**
   * @brief 接続を開始します。setup() 内で 1 度呼び出します。
   */
  void begin();

  /**
   * @brief メインループ処理。loop() 内で毎フレーム呼び出します。
   */
  void loop();

  SessionState state() const { return _state; }

  /**
   * @brief テスト音声ストリームを送信します（Active 状態のみ有効）。
   * audio.stream_open → binary フレーム × frameCount → audio.end の順で送信します。
   * Phase 5: サイレンス PCM データを送信します。
   * @param frameCount 送信するフレーム数（デフォルト: 50 = 1 秒分）
   */
  void sendAudioStream(int frameCount = 50);

 private:
  Network::WsClient  _ws;
  Audio::MicReader   _mic;
  Audio::TTSPlayer   _ttsPlayer;
  Protocol::OutboundSequence _seq;

  SessionState  _state{SessionState::Idle};
  String        _sessionId{""};

  // heartbeat 管理
  unsigned long _heartbeatIntervalMs{FW_HEARTBEAT_INTERVAL_MS};
  unsigned long _lastHeartbeatMs{0};

  // Wi-Fi 再接続管理
  unsigned long _wifiRetryDelayMs{FW_RECONNECT_BASE_MS};
  unsigned long _lastWiFiAttemptMs{0};

  // 再生・アバター状態（Phase 6）
  String _currentRequestId{""};
  String _expression{"neutral"};
  String _motion{"idle"};
  unsigned long _lastAvatarRenderMs{0};
  m5avatar::Avatar _avatar;
  bool _avatarReady{false};

  // ── 内部ヘルパー ──────────────────────────────────────────────────
  void setState(SessionState next);

  // WebSocket コールバック
  void onWSConnected();
  void onWSDisconnected();
  void onTextMessage(const String& msg);

  // 送信ヘルパー
  void sendHello();
  void sendHeartbeat();
  void updateAvatarFace();
  m5avatar::Expression toAvatarExpression(const String& expression) const;
  bool decodeBase64(const String& src, uint8_t** out, size_t* outLen);

  // 受信イベントハンドラ（P6 で avatar / motion を追加）
  void handleWelcome(const String& payloadJson, const String& envelopeSessionId);
  void handleSTTFinal(const String& payloadJson);
  void handleTTSEnd(const String& payloadJson);
  void handleAvatarExpression(const String& payloadJson);
  void handleMotionPlay(const String& payloadJson);
  void handleError(const String& payloadJson);
};

}  // namespace App
