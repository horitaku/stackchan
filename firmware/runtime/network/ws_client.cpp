/**
 * @file ws_client.cpp
 * @brief WebSocket クライアント（指数バックオフ再接続対応）実装
 */
#include "ws_client.h"

namespace Network {

WsClient::WsClient() {}

void WsClient::setUrl(const String& url) {
  parseUrl(url);
}

void WsClient::setToken(const String& token) {
  _token = token;
  // トークン値はログに出力しません（セキュリティ要件）
  Serial.printf("[WS] Token configured: %s\n", token.isEmpty() ? "none" : "(set)");
}

void WsClient::setReconnectPolicy(const ReconnectPolicy& policy) {
  _policy = policy;
  _reconnectDelayMs = policy.baseMsDelay;
}

void WsClient::parseUrl(const String& url) {
  // wss:// または ws:// を判定します
  bool isTls = url.startsWith("wss://");
  _tls       = isTls;

  // スキームを除去して host:port/path を取り出します
  String rest     = url.substring(isTls ? 6 : 5);
  int    pathStart = rest.indexOf('/');
  String hostPort  = (pathStart >= 0) ? rest.substring(0, pathStart) : rest;
  _path            = (pathStart >= 0) ? rest.substring(pathStart) : "/";

  // ホストとポートを分割します
  int colonPos = hostPort.indexOf(':');
  if (colonPos >= 0) {
    _host = hostPort.substring(0, colonPos);
    _port = (uint16_t)hostPort.substring(colonPos + 1).toInt();
  } else {
    _host = hostPort;
    _port = isTls ? 443 : 80;
  }

  Serial.printf("[WS] URL parsed → host=%s port=%d path=%s tls=%d\n",
    _host.c_str(), _port, _path.c_str(), (int)_tls);
}

void WsClient::begin() {
  // WebSocket イベントコールバックを登録します
  _ws.onEvent([this](WStype_t type, uint8_t* payload, size_t length) {
    handleEvent(type, payload, length);
  });

  // 認証トークンが設定されている場合は Authorization ヘッダを付与します
  if (!_token.isEmpty()) {
    String header = "Authorization: Bearer " + _token;
    _ws.setExtraHeaders(header.c_str());
  }

  // 接続を開始します
  if (_tls) {
    // Phase 5 では CA 証明書なし（insecure）で TLS 接続します
    _ws.beginSSL(_host.c_str(), _port, _path.c_str());
  } else {
    _ws.begin(_host.c_str(), _port, _path.c_str());
  }
  Serial.printf("[WS] Connecting to %s:%d%s\n", _host.c_str(), _port, _path.c_str());
}

void WsClient::loop() {
  // WebSocket の受信処理と内部ステート管理を実行します
  _ws.loop();

  // 再接続待機タイマーが到達したら再接続を試みます
  if (_reconnectPending && !_connected) {
    if (millis() - _lastDisconnectMs >= _reconnectDelayMs) {
      tryReconnect();
    }
  }
}

void WsClient::tryReconnect() {
  _reconnectAttempts++;
  Serial.printf("[WS] Reconnect attempt #%d (waited %lu ms)\n",
    _reconnectAttempts, _reconnectDelayMs);

  // 接続を再試行します
  _ws.begin(_host.c_str(), _port, _path.c_str());
  _reconnectPending = false;

  // 指数バックオフ: 次回の待機時間を 2 倍にし、maxMsDelay でキャップします
  _reconnectDelayMs = min(_reconnectDelayMs * 2, _policy.maxMsDelay);
}

bool WsClient::sendText(const String& text) {
  if (!_connected) {
    Serial.println("[WS] sendText: not connected, skipping");
    return false;
  }
  return _ws.sendTXT(text.c_str(), text.length());
}

bool WsClient::sendBinary(const uint8_t* data, size_t len) {
  if (!_connected) {
    Serial.println("[WS] sendBinary: not connected, skipping");
    return false;
  }
  return _ws.sendBIN(data, len);
}

void WsClient::handleEvent(WStype_t type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      _connected        = true;
      _reconnectPending = false;
      _reconnectAttempts = 0;
      // 再接続成功: 待機時間を初期値にリセットします
      _reconnectDelayMs = _policy.baseMsDelay;
      Serial.printf("[WS] Connected (attempts=%d)\n", _reconnectAttempts);
      if (_onConnected) _onConnected();
      break;

    case WStype_DISCONNECTED:
      _connected        = true;  // ループ終了後に false → 下行で上書き
      _connected        = false;
      _lastDisconnectMs = millis();
      _reconnectPending = true;
      Serial.printf("[WS] Disconnected. Next retry in %lu ms\n", _reconnectDelayMs);
      if (_onDisconnected) _onDisconnected();
      break;

    case WStype_TEXT:
      if (_onText) {
        _onText(String(reinterpret_cast<char*>(payload), length));
      }
      break;

    case WStype_BIN:
      if (_onBinary) {
        _onBinary(payload, length);
      }
      break;

    case WStype_ERROR:
      Serial.printf("[WS] Error: %.*s\n", (int)length, reinterpret_cast<char*>(payload));
      break;

    case WStype_PING:
      Serial.println("[WS] PING received");
      break;

    case WStype_PONG:
      Serial.println("[WS] PONG received");
      break;

    default:
      break;
  }
}

}  // namespace Network
