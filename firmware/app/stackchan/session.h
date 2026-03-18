/**
 * @file session.h
 * @brief Stackchan セッション・オーケストレーション クラス
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
 * 
 * @section P8-17 受信・消費の責務分離（Producer-Consumer Pattern）
 * 
 * TTS 再生パイプラインは以下の役割分担で実装されています：
 * 
 * - **Producer（受信側）**: onTextMessage() → enqueueTTSFrame()
 *   - WebSocket 受信フレームをキューに enqueue（ノンブロッキング）
 *   - 受信遅延が再生フロー全体に与える影響を最小化
 * 
 * - **Consumer（消費側）**: processTTSPlaybackQueue()
 *   - キューから dequeue → 40ms 分を集約 → playPCM16() で再生
 *   - Watermark 監視（prebuffer / low-water / high-water）
 *   - バッファ深さと滞留時間をログ出力（observability 強化）
 * 
 * この分離により、受信ジッターと再生ジッターの因果関係を弱め、
 * 将来の低遅延会話実装の土台を整えます（P9以降で活用予定）。
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
 * @brief 会話体験の状態を表します。
 */
enum class ConversationState {
  Idle,
  Listening,
  Thinking,
  Speaking,
  Interrupted,
  Error,
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
   * 
   * P8-17 で責務を明確化：
   * - Producer フロー: _ws.loop() で受信フレーム → enqueueTTSFrame() で enqueue
   * - Consumer フロー: processTTSPlaybackQueue() で dequeue → 再生
   * - 受信と消費の分離により、互いに独立した進捗が可能
   */
  void loop();

  SessionState state() const { return _state; }
  ConversationState conversationState() const { return _conversationState; }

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
  ConversationState _conversationState{ConversationState::Idle};
  Audio::PlaybackState _lastPlaybackState{Audio::PlaybackState::Idle};

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

  // 受信 TTS チャンクバッファ
  uint8_t* _incomingTTSBuffer{nullptr};
  size_t _incomingTTSBufferLen{0};
  int _incomingTTSExpectedChunks{0};
  int _incomingTTSReceivedChunks{0};
  String _incomingTTSRequestId{""};

  // P8-15: 事前バッファ付き再生キュー（low-water / high-water 管理）
  // P8-17: Consumer フロー専用のリングバッファ
  static constexpr size_t kTTSFrameQueueCapacity = 32;
  static constexpr uint16_t kTTSPrebufferStartMs = 80;   ///< 再生開始前の最小バッファ
  static constexpr uint16_t kTTSLowWaterMs = 60;         ///< 警告レベル
  static constexpr uint16_t kTTSHighWaterMs = 240;       ///< ドロップレベル
  static constexpr uint16_t kTTSPlaybackBatchMs = 40;    ///< 1 回の再生バッチ長

  struct TTSFrameSlot {
    uint8_t* bytes{nullptr};
    size_t byteLen{0};
    uint16_t frameDurationMs{0};
    uint16_t samplesPerChunk{0};
    int chunkIndex{0};
  };

  TTSFrameSlot _ttsFrameQueue[kTTSFrameQueueCapacity];
  size_t _ttsFrameHead{0};
  size_t _ttsFrameTail{0};
  size_t _ttsFrameCount{0};
  uint32_t _ttsBufferedMs{0};
  bool _ttsPlaybackPrimed{false};
  bool _ttsStreamEnded{false};
  String _ttsStreamRequestId{""};
  String _ttsStreamId{""};
  int _ttsExpectedChunkIndex{0};
  uint32_t _ttsSampleRateHz{FW_AUDIO_SAMPLE_RATE};

  // P8-16: concealment（欠落補完）関連
  // 欠落検知時に挿入する最大フレーム数（80ms @ 20ms/frame）
  static constexpr int kMaxConcealmentFrames = 4;
  // 直前の正常フレームのコピーを保持して減衰コピー生成に使用します。
  uint8_t* _ttsLastGoodFrameBytes{nullptr};
  size_t   _ttsLastGoodFrameLen{0};
  // ストリーム内の欠落チャンク数と補完フレーム数の累計です（ログ用）。
  int _ttsMissingChunkCount{0};
  int _ttsConcealmentFrameCount{0};

  // ── 内部ヘルパー ──────────────────────────────────────────────────
  void setState(SessionState next);
  void setConversationState(ConversationState next, const char* reason);
  const char* conversationStateName(ConversationState state) const;

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
  void handleTTSChunk(const String& payloadJson);
  void handleTTSEnd(const String& payloadJson);
  void handleAvatarExpression(const String& payloadJson);
  void handleMotionPlay(const String& payloadJson);
  void handleConversationCancel(const String& payloadJson);
  void handleTTSStop(const String& payloadJson);
  void handleAudioStreamAbort(const String& payloadJson);
  void handleError(const String& payloadJson);
  void clearIncomingTTSBuffer();
  bool appendIncomingTTSChunk(const String& requestId, int chunkIndex, int totalChunks, const String& audioBase64);
  void clearTTSFrameQueue();
  bool enqueueTTSFrame(const String& requestId,
                       const String& streamId,
                       int chunkIndex,
                       int frameDurationMs,
                       int samplesPerChunk,
                       const String& audioBase64);
  bool dequeueTTSPlaybackBatch(uint16_t targetDurationMs, uint8_t** outBytes, size_t* outByteLen, uint16_t* outDurationMs);
  void processTTSPlaybackQueue();
  /**
   * @brief 欠落フレームに対して concealment（補完）を挿入します。
   * 直前の正常フレームが存在する場合は振幅を 50% に減衰したコピーを、
   * 存在しない場合は無音（ゼロ PCM）を挿入します。
   * @param gapCount  補完するフレーム数
   * @param frameDurationMs フレーム長（ms）
   * @param samplesPerChunk 1 フレームのサンプル数
   */
  void insertConcealmentFrames(int gapCount, int frameDurationMs, int samplesPerChunk);
};

}  // namespace App
