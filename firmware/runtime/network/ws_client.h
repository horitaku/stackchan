/**
 * @file ws_client.h
 * @brief WebSocket クライアント（指数バックオフ再接続対応）
 * 
 * links2004/WebSockets ライブラリをラップし、以下の機能を提供します。
 * - テキスト / バイナリメッセージの送受信
 * - 接続 / 切断イベントのコールバック登録
 * - 指数バックオフによる自動再接続（P5-04 対応）
 * - FW_WS_TOKEN が設定されている場合の Authorization ヘッダ付与
 * 
 * セキュリティ注意:
 *   - FW_WS_TOKEN はシリアルログへの平文出力を禁止します。
 */
#pragma once

#include <Arduino.h>
#include <WebSocketsClient.h>
#include <functional>

namespace Network {

/**
 * @brief 指数バックオフ再接続ポリシーです。
 * 切断後の待機時間を baseMsDelay から始めて 2 倍ずつ増加させ、
 * maxMsDelay でキャップします。
 */
struct ReconnectPolicy {
  unsigned long baseMsDelay;  ///< 初回待機時間（ms）
  unsigned long maxMsDelay;   ///< 最大待機時間（ms）
};

using OnConnectedCallback    = std::function<void()>;
using OnDisconnectedCallback = std::function<void()>;
using OnTextCallback         = std::function<void(const String&)>;
using OnBinaryCallback       = std::function<void(const uint8_t*, size_t)>;

/**
 * @brief WebSocket クライアント（自動再接続対応）。
 * loop() を毎フレーム呼び出すことで自動再接続が機能します。
 */
class WsClient {
 public:
  WsClient();

  /**
   * @brief 接続先 URL を設定します。ws:// または wss:// をサポートします。
   * wss は Phase 5 では CA 証明書なし（insecure）で接続します。
   */
  void setUrl(const String& url);

  /**
   * @brief 認証トークンを設定します（空文字列または未設定なら無効）。
   * トークンは Authorization: Bearer ヘッダとして付与されます。
   * トークン値はシリアルログに出力しません。
   */
  void setToken(const String& token);

  /**
   * @brief 指数バックオフ再接続ポリシーを設定します。
   */
  void setReconnectPolicy(const ReconnectPolicy& policy);

  void onConnected(OnConnectedCallback cb)     { _onConnected = cb; }
  void onDisconnected(OnDisconnectedCallback cb){ _onDisconnected = cb; }
  void onTextMessage(OnTextCallback cb)         { _onText = cb; }
  void onBinaryMessage(OnBinaryCallback cb)     { _onBinary = cb; }

  /**
   * @brief 接続を開始します。begin() 後に loop() を毎フレーム呼び出してください。
   */
  void begin();

  /**
   * @brief メインループから毎フレーム呼び出します。
   * WebSocket ループ処理と再接続タイミング管理を行います。
   */
  void loop();

  /**
   * @brief テキストメッセージを送信します。
   * @return 送信成功なら true（未接続の場合は false）
   */
  bool sendText(const String& text);

  /**
   * @brief バイナリメッセージを送信します。
   * @return 送信成功なら true（未接続の場合は false）
   */
  bool sendBinary(const uint8_t* data, size_t len);

  bool isConnected() const { return _connected; }
  int reconnectAttempts() const { return _reconnectAttempts; }

 private:
  WebSocketsClient _ws;

  // 接続先情報
  String   _host;
  uint16_t _port{8080};
  String   _path{"/"};
  bool     _tls{false};
  String   _token;

  // 接続状態
  bool _connected{false};

  // 再接続管理
  ReconnectPolicy _policy{500, 10000};
  unsigned long   _reconnectDelayMs{500};  ///< 次回の待機時間
  unsigned long   _lastDisconnectMs{0};    ///< 最後に切断した時刻
  bool            _reconnectPending{false};
  int             _reconnectAttempts{0};

  OnConnectedCallback    _onConnected;
  OnDisconnectedCallback _onDisconnected;
  OnTextCallback         _onText;
  OnBinaryCallback       _onBinary;

  /**
   * @brief URL をパースして _host / _port / _path / _tls に展開します。
   */
  void parseUrl(const String& url);

  /**
   * @brief WebSocket イベントを処理します（links2004/WebSockets コールバック）。
   */
  void handleEvent(WStype_t type, uint8_t* payload, size_t length);

  /**
   * @brief 再接続を試みます。失敗時は次回の待機時間を更新します。
   */
  void tryReconnect();
};

}  // namespace Network
