# P12-01 StackchanSession 責務マップと依存一覧

## 1. 目的

Phase 12 の file split を安全に進めるため、現行 `StackchanSession` の責務・状態・メモリ所有権・分割方針を先に固定します。
本ドキュメントは P12-02 以降の設計入力として利用します。

## 2. スコープ

- 対象: `firmware/app/stackchan/session.h`, `firmware/app/stackchan/session.cpp`
- 非対象: protocol 仕様変更、public API 変更、新機能追加

## 3. 責務マップ（現状）

`StackchanSession` は現在、以下の責務を単一クラス内で担っています。

1. 接続ライフサイクル管理
- `begin()`: Wi-Fi 初期接続、WS 初期化、コールバック登録、初期状態設定
- `loop()`: Wi-Fi 再接続バックオフ、WS 受信ポーリング、heartbeat 送信
- `onWSConnected()`, `onWSDisconnected()`

2. プロトコル送信
- `sendHello()`
- `sendHeartbeat()`
- `sendTTSBufferWatermark()`
- `sendAudioStream()`（`audio.stream_open` / binary / `audio.end`）

3. プロトコル受信ディスパッチ
- `onTextMessage()` が `type` に応じて各 handler へ分岐
- 対応イベント:
  - `session.welcome`
  - `stt.final`
  - `tts.chunk`
  - `tts.end`
  - `avatar.expression`
  - `motion.play`
  - `conversation.cancel`
  - `tts.stop`
  - `audio.stream_abort`
  - `error`

4. 会話状態制御
- `setConversationState()`, `conversationStateName()`
- 受信イベント、再生状態、interrupt で状態遷移を実施

5. TTS 受信バッファ（旧方式）管理
- `_incomingTTSBuffer` へのチャンク集約
- `appendIncomingTTSChunk()`, `clearIncomingTTSBuffer()`

6. TTS ストリームキュー（新方式）管理
- リングバッファ `_ttsFrameQueue` の enqueue/dequeue
- prebuffer / low-water / high-water 判定
- `clearTTSFrameQueue()`, `enqueueTTSFrame()`, `dequeueTTSPlaybackBatch()`, `dequeueTTSFrame()`

7. 欠落補完（concealment）
- 欠落フレーム検出と補完挿入
- `insertConcealmentFrames()`

8. Opus デコード管理
- デコーダ生成/破棄/再利用
- `resetOpusDecoder()`, `ensureOpusDecoder()`, `decodeOpusFrame()`

9. 再生オーケストレーション
- `processTTSPlaybackQueue()`
- codec ごとの再生分岐（PCM/Opus）
- speaking/idle 遷移制御

10. Avatar 表示と最小 motion
- `updateAvatarFace()`, `toAvatarExpression()`
- `handleAvatarExpression()`, `handleMotionPlay()`

11. 共通ユーティリティ
- `decodeBase64()`

## 4. 依存一覧

## 4.1 外部モジュール依存

- `Network::connect()`, `Network::isConnected()`, `Network::getRSSI()`
- `Network::WsClient`（URL、再接続ポリシー、send/receive）
- `Audio::MicReader`（uplink 音声取得）
- `Audio::TTSPlayer`（再生状態、口パクレベル、再生開始）
- `Protocol::buildEnvelope()`, `Protocol::generateUUIDv4()`, `Protocol::OutboundSequence`
- `ArduinoJson`（受信/送信 payload の parse/serialize）
- `mbedtls_base64_decode`（base64 展開）
- `OpusDecoder`（Opus 変換）
- `m5avatar::Avatar`, `M5.Speaker`

## 4.2 内部状態依存（主な結合）

- 接続状態 `_state` と `_sessionId` と `_seq` は密結合
- 会話状態 `_conversationState` は STT/TTS/interrupt と密結合
- 再生状態 `_lastPlaybackState` は speaking -> idle 判定に使用
- `_ttsSampleRateHz` は first chunk 推定値と `tts.end` 値で更新され、Opus デコードに影響
- `_ttsStreamRequestId`, `_ttsStreamId`, `_ttsExpectedChunkIndex` は順序整合と欠落補完の軸
- `_ttsWatermarkStatus` と cooldown は送信頻度抑制に直結

## 5. 状態一覧

## 5.1 SessionState

- `Idle`
- `ConnectingWiFi`
- `ConnectingWS`
- `Handshaking`
- `Active`
- `Error`

主な遷移:

- 起動: `Idle -> ConnectingWiFi -> ConnectingWS -> Handshaking -> Active`
- 切断: `onWSDisconnected()` で `ConnectingWS`
- welcome reject: `handleWelcome()` で `Error`

## 5.2 ConversationState

- `Idle`
- `Listening`（audio uplink 開始）
- `Thinking`（`audio.end` 送信後 / `stt.final` 受信後）
- `Speaking`（TTS 再生開始時）
- `Interrupted`（cancel/stop/abort）
- `Error`（non-retryable error）

主な遷移トリガ:

- `sendAudioStream()` で `Idle -> Listening -> Thinking`
- `processTTSPlaybackQueue()` または `handleTTSEnd()` で `Thinking -> Speaking`
- 再生完了検知で `Speaking -> Idle`
- `handleConversationCancel()/handleTTSStop()/handleAudioStreamAbort()` で `Interrupted -> Idle`

## 6. メモリ所有権メモ（重要）

## 6.1 `_incomingTTSBuffer` 系

- 生成: `appendIncomingTTSChunk()` 内 `realloc()`
- 解放: `clearIncomingTTSBuffer()`
- 代表的な解放契機:
  - request 切替
  - chunk 不整合
  - interrupt 系 (`conversation.cancel`, `tts.stop`, `audio.stream_abort`)
  - WS 切断
  - `handleTTSEnd()` で再生に渡した後

注意点:

- `_incomingTTSBuffer` と局所 `decoded`（`decodeBase64` 戻り値）の所有者が分岐するため、二重解放を避ける条件分岐が必要

## 6.2 `_ttsFrameQueue` 系

- 生成: `enqueueTTSFrame()` / `insertConcealmentFrames()` の `malloc()`
- 移譲: `dequeueTTSFrame()` は ownership を呼び出し側へ移譲
- 解放:
  - `dequeueTTSPlaybackBatch()` 内で pop 時に `free()`
  - Opus 経路では `processTTSPlaybackQueue()` 内で `frame.bytes` を `free()`
  - 全量破棄は `clearTTSFrameQueue()`

注意点:

- `dequeueTTSFrame()` 後の `frame.bytes` 解放責務は caller 側
- queue overflow / high-water drop 時に `decoded` を即時 `free()` する分岐が存在

## 6.3 Opus デコーダ

- 保持: `_ttsOpusDecoder`
- 生成: `ensureOpusDecoder()`
- 破棄: `resetOpusDecoder()` / `clearTTSFrameQueue()`

注意点:

- `_ttsSampleRateHz` 更新とデコーダ再生成条件が連動
- sample rate 不整合時は decode 失敗リスク

## 6.4 concealment バッファ

- 保持: `_ttsLastGoodFrameBytes`
- 更新: `enqueueTTSFrame()`（PCM 時）
- 解放: `clearTTSFrameQueue()`（都度上書き前にも解放）

## 7. 壊れやすい依存ポイント

1. speaking -> idle 遷移が複数経路に分散
- `loop()` の再生状態監視
- `processTTSPlaybackQueue()` の stream drained 判定

2. interrupt 系での停止手順重複
- `_ttsPlayer.stop()`
- `clearTTSFrameQueue()`
- `clearIncomingTTSBuffer()`

3. 旧方式（`_incomingTTSBuffer`）と新方式（`_ttsFrameQueue`）の併存
- request 単位で分岐するため、将来 split 時に責務混線しやすい

4. sample rate 決定経路が複数
- first chunk 推定
- `tts.end` の `sample_rate_hz`

5. watermark 送信条件
- status 変化 + cooldown という状態依存ロジック

## 8. P12-02 へ引き渡す分割方針（初版）

以下の単位で file split する方針を推奨します（public API は不変）。

1. `session.cpp`
- 入口のみ: constructor, `begin()`, `loop()` のオーケストレーション最小化

2. `session_connection.cpp`
- `setState()`, `onWSConnected()`, `onWSDisconnected()`, Wi-Fi/WS 再接続補助

3. `session_protocol.cpp`
- `onTextMessage()` と送信系（`sendHello()`, `sendHeartbeat()`, `sendTTSBufferWatermark()`）
- 各 handler へのディスパッチ

4. `session_audio_upload.cpp`
- `sendAudioStream()`

5. `session_tts_stream.cpp`
- `handleTTSChunk()`, `handleTTSEnd()`
- queue 管理、concealment、Opus、watermark、再生処理
- `decodeBase64()` を含める（TTS 側依存が高いため）

6. `session_avatar.cpp`
- `handleAvatarExpression()`, `handleMotionPlay()`, `updateAvatarFace()`, `toAvatarExpression()`

7. `session_handlers_control.cpp`（任意）
- interrupt と error 系（`handleConversationCancel`, `handleTTSStop`, `handleAudioStreamAbort`, `handleError`）
- 状態遷移と停止処理の共通化準備

## 9. 受け入れ観点（P12-01 完了条件）

- 現行責務が漏れなく列挙されている
- 2 種類の状態機械（Session/Conversation）の主要遷移が明文化されている
- 動的メモリの所有者と解放契機が追跡できる
- P12-02 がこの資料を入力として file split 境界を定義できる
