# P12-02 実装ファイル分割境界 定義

## 1. 目的

Phase 12 の第1段階（file split）を安全に進めるため、`StackchanSession` の実装分割境界を固定します。
本定義は public API（`session.h`）を変更せず、外部挙動を維持したまま `session.cpp` を縮小するための実装ガイドです。

## 2. 前提と制約

- `firmware/app/stackchan/session.h` の public API は変更しない
- WebSocket 契約（event type / payload）は変更しない
- hello / welcome / heartbeat / audio uplink / tts playback / interrupt の外部挙動を維持する
- 分割対象は「実装配置」のみであり、機能追加は行わない

## 3. 分割ターゲット

分割後の想定ファイル構成は以下とします。

```txt
firmware/app/stackchan/
  session.cpp
  session_connection.cpp
  session_protocol.cpp
  session_audio_upload.cpp
  session_tts_stream.cpp
  session_avatar.cpp
```

補足:

- `session_handlers_control.cpp` は P12-09（受信ルータ整理）で必要になった時点で追加する
- P12-03〜P12-07 は本ファイル境界に沿って順次実装する

## 4. 境界定義（責務と関数マッピング）

## 4.1 session.cpp（入口 + 最小 orchestration）

責務:

- クラス定義の実体化（constructor）
- `begin()` と `loop()` の高レベル制御
- 他ファイル実装への委譲点を保持

配置関数:

- `StackchanSession::StackchanSession()`
- `StackchanSession::begin()`
- `StackchanSession::loop()`

境界ルール:

- 直接の JSON parse / dispatch 詳細を持たない
- TTS queue の内部操作（enqueue/dequeue/decoder管理）を持たない
- Avatar 表情変換ロジックを持たない

## 4.2 session_connection.cpp（接続ライフサイクル）

責務:

- セッション接続状態管理
- WS connect/disconnect 時の初期化・クリア処理

配置関数:

- `setState()`
- `onWSConnected()`
- `onWSDisconnected()`

境界ルール:

- protocol payload の組み立ては持たない（`sendHello()` 側へ委譲）
- conversation state の個別業務遷移は最小に留める

## 4.3 session_protocol.cpp（送受信とディスパッチ）

責務:

- protocol送信ヘルパー
- 受信 envelope の parse と event dispatch
- 軽量な受信 handler（state更新中心）

配置関数:

- `sendHello()`
- `sendHeartbeat()`
- `sendTTSBufferWatermark()`
- `onTextMessage()`
- `handleWelcome()`
- `handleSTTFinal()`
- `handleConversationCancel()`
- `handleTTSStop()`
- `handleAudioStreamAbort()`
- `handleError()`

境界ルール:

- TTS queue の細部処理は呼び出しだけにして実装は置かない
- Avatar 描画更新本体は持たない
- audio uplink 本体は持たない

## 4.4 session_audio_upload.cpp（audio uplink）

責務:

- uplink の open/binary/end 送信シーケンス
- uplink 開始時の会話状態遷移（listening/thinking）

配置関数:

- `sendAudioStream()`

境界ルール:

- 下りTTS再生・queue制御を持たない
- protocol受信処理を持たない

## 4.5 session_tts_stream.cpp（TTSストリーム処理の中核）

責務:

- tts.chunk / tts.end の処理
- 旧バッファ方式と新フレームキュー方式の受信集約
- prebuffer / watermark / concealment / Opus decode
- TTS再生開始判定と drain 完了処理
- 動的メモリの解放責務を集中管理

配置関数:

- `handleTTSChunk()`
- `handleTTSEnd()`
- `decodeBase64()`
- `clearIncomingTTSBuffer()`
- `appendIncomingTTSChunk()`
- `clearTTSFrameQueue()`
- `enqueueTTSFrame()`
- `dequeueTTSPlaybackBatch()`
- `dequeueTTSFrame()`
- `insertConcealmentFrames()`
- `resetOpusDecoder()`
- `ensureOpusDecoder()`
- `decodeOpusFrame()`
- `processTTSPlaybackQueue()`

境界ルール:

- 接続ライフサイクル制御を持たない
- WebSocket callback 登録処理を持たない

## 4.6 session_avatar.cpp（表示と最小 motion）

責務:

- avatar expression 更新
- motion 演出（最小実装）
- lip sync 更新

配置関数:

- `handleAvatarExpression()`
- `handleMotionPlay()`
- `updateAvatarFace()`
- `toAvatarExpression()`
- `setConversationState()`
- `conversationStateName()`

境界ルール:

- protocol受信ディスパッチ本体を持たない
- queue/decoder の内部状態を直接変更しない

補足:

- `setConversationState()` の配置は `session_protocol.cpp` でも成立するが、現状は avatar 表示文言更新と連携して参照頻度が高いため、まず `session_avatar.cpp` に置く
- 将来 P12-09 で遷移管理を分離する際に再配置を検討する

## 5. include 方針

## 5.1 共通ルール

- 各分割 `.cpp` は先頭で `session.h` のみを include することを原則とする
- 各 `.cpp` 専用の追加 include は「そのファイルで直接使うものだけ」に限定する
- 他分割 `.cpp` を include しない（実装ファイル間依存禁止）
- 依存方向は `session.h` を中心に一方向とする

## 5.2 ファイル別の追加 include 指針

- `session.cpp`
  - 追加 include 最小（必要時のみ）
- `session_connection.cpp`
  - 原則追加なし
- `session_protocol.cpp`
  - `ArduinoJson.h`（JSON parse/serialize があるため）
- `session_audio_upload.cpp`
  - `ArduinoJson.h`（open/end payload 組み立てがあるため）
- `session_tts_stream.cpp`
  - `ArduinoJson.h`
  - `mbedtls/base64.h`
  - `opus.h`（`extern "C"`）
- `session_avatar.cpp`
  - 原則追加なし（`session.h` 経由依存で完結）

## 5.3 依存逆流の禁止事項

- `session_protocol.cpp` から queue 内部配列を直接触らない
- `session_avatar.cpp` から decode/queue API を直接操作しない
- `session_audio_upload.cpp` から WS callback 系を変更しない

## 6. 実施順（P12-03〜P12-07の入力）

1. P12-03: `session_connection.cpp` 抽出
2. P12-04: `session_protocol.cpp` 抽出
3. P12-05: `session_audio_upload.cpp` 抽出
4. P12-06: `session_tts_stream.cpp` 抽出
5. P12-07: `session_avatar.cpp` 抽出

各段階の完了条件:

- firmware がビルドできる
- public API 差分なし
- 挙動回帰なし（hello/welcome/heartbeat/audio uplink/tts/interrupt）

## 7. 回帰リスクと事前ガード

1. メモリ解放漏れ・二重解放
- ガード: free責務を `session_tts_stream.cpp` に集中し、関連関数を分散配置しない

2. speaking -> idle 遷移の欠落
- ガード: `loop()` と `processTTSPlaybackQueue()` の双方条件を維持したまま移設する

3. watermark 送信頻度の破綻
- ガード: `_ttsWatermarkStatus` と cooldown 判定を一塊で移設する

4. サンプルレート不整合
- ガード: first chunk 推定と `tts.end` 更新の両経路を削らない

5. interrupt 反映漏れ
- ガード: cancel/stop/abort の停止3点セット（`_ttsPlayer.stop()`, `clearTTSFrameQueue()`, `clearIncomingTTSBuffer()`）を保持する

## 8. 受け入れ条件（P12-02）

- 分割先ファイルと責務が 1 対 1 で定義されている
- 主要関数の配置先が明示されている
- include 方針が明文化されている
- P12-03 以降が設計迷いなく着手できる
