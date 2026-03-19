/**
 * @file session_connection.cpp
 * @brief StackchanSession の接続ライフサイクル実装
 */
#include "session.h"

namespace App {

// ──────────────────────────────────────────────────────────────────────
// Private: 接続状態と WebSocket コールバック
// ──────────────────────────────────────────────────────────────────────

void StackchanSession::setState(SessionState next) {
  if (_state == next) return;
  Serial.printf("[Session] State: %d → %d\n", (int)_state, (int)next);
  _state = next;
}

void StackchanSession::onWSConnected() {
  setState(SessionState::Handshaking);
  // 再接続時はシーケンスをリセットして新しい hello を送信します。
  _seq.reset();
  _sessionId = "";
  sendHello();
}

void StackchanSession::onWSDisconnected() {
  setState(SessionState::ConnectingWS);
  _ttsPlayer.stop();
  clearTTSFrameQueue();
  clearIncomingTTSBuffer();
  // WsClient 側の指数バックオフ再接続がこの後に継続します。
}

}  // namespace App
