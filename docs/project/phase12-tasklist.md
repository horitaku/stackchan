# フェーズ 12 タスクリスト（Firmware 内部リファクタリング）

## 1. このドキュメントの目的

フェーズ 12 を、Phase 11 の hardware 拡張を安全に継続するための内部リファクタリング作業として実行可能なタスクへ分解します。
本フェーズは機能追加を目的とせず、肥大化した StackchanSession の責務を整理し、変更容易性・検証容易性・将来の service 分離を進めるための土台づくりを主眼とします。

フェーズ 12 の主眼は以下です：
- StackchanSession の責務過多を整理し、今後の hardware 拡張時の影響範囲を減らす
- 外部挙動を維持したまま、実装ファイル分割と内部境界の明確化を進める
- TTS ストリーム処理、Avatar 表示、受信ディスパッチの独立性を高める
- メモリ所有権と状態遷移の見通しを改善し、回帰リスクを下げる

## 2. フェーズ 12 で固定する前提

### 2.1 このフェーズで守ること

- session.h の public API は原則維持する
- WebSocket 契約と firmware の外部挙動は変えない
- hello / welcome / heartbeat / audio uplink / tts playback / interrupt の既存フローを壊さない
- 変更は段階的に行い、まずは file split、その後に必要最小限の module split を行う

### 2.2 このフェーズで無理にやらないこと

- device.servo.move など Phase 11 の新機能実装
- protocol event の追加や payload 変更
- 大規模なクラス階層化
- 既存ランタイム挙動の最適化を名目にした仕様変更

### 2.3 現在の session.cpp の主要責務

現状の StackchanSession は少なくとも次の責務を持っています。

- 接続ライフサイクル管理
- protocol 送受信
- 会話状態遷移
- 音声 uplink 送信
- TTS streaming buffer 管理
- watermark / concealment / Opus decode
- avatar 表示更新
- motion の最小演出
- interrupt 系停止処理

本フェーズでは、これらを一気に別クラス化するのではなく、壊れにくい順に整理します。

## 3. リファクタリング方針

### 3.1 第 1 段階: 実装ファイル分割

最初は StackchanSession の public / private 構造を大きく変えず、実装だけを複数ファイルへ分ける。

想定例：

```txt
firmware/app/stackchan/
  session.cpp                  # 入口と最小 orchestration のみ
  session_connection.cpp       # begin / loop / ws connect/disconnect / heartbeat
  session_protocol.cpp         # onTextMessage / sendHello / handler dispatch
  session_audio_upload.cpp     # sendAudioStream
  session_tts_stream.cpp       # frame queue / concealment / opus / playback queue
  session_avatar.cpp           # expression / motion / lip sync / display update
```

この段階の目的：

- 可読性を上げる
- 差分レビューをしやすくする
- 将来の module split に向けて関心事を固定する

### 3.2 第 2 段階: 内部モジュール化

実装ファイル分割後、独立性の高い塊だけを internal helper または補助クラスへ抜く。

優先順は以下を推奨する。

1. TTS ストリーム処理
2. Avatar 表示と最小 motion 演出
3. 受信イベントルータ

接続ライフサイクル本体は Session の中心なので、後回しにする。

### 3.3 壊れやすい箇所の扱い

以下はリファクタ中に特に注意する。

- malloc / realloc / free の所有権
- _ttsFrameQueue と _incomingTTSBuffer の解放タイミング
- Speaking -> Idle の状態遷移条件
- watermark 送信と queue 深さ管理
- _ttsSampleRateHz の更新と Opus decoder の再初期化条件
- Avatar 表示更新と TTSPlayer.lipLevel() の連携

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P12-01 | StackchanSession の責務マップと依存一覧を作成 | 責務一覧、状態一覧、メモリ所有権メモ、分割方針 | 高 | 先に壊れやすい依存を可視化しないと、分割時に責務漏れが起きやすいため | 完了（docs/project/phase12-p12-01-responsibility-map.md） |
| P12-02 | 実装ファイル分割の境界を定義 | file split 案、各 .cpp の責務、include 方針 | 高 | public API を維持したまま安全に分けるには、先に境界を固定する必要があるため | 完了（docs/project/phase12-p12-02-file-split-boundary.md） |
| P12-03 | connection / lifecycle 実装を分離 | session_connection.cpp、接続関連ロジック整理 | 中 | begin / loop / heartbeat を他責務から分け、以後の差分を局所化するため | 完了（firmware/app/stackchan/session_connection.cpp） |
| P12-04 | protocol send / receive 実装を分離 | session_protocol.cpp、dispatch 整理、handler 配置 | 高 | 受信イベント拡張時に session.cpp 本体が再肥大化するのを防ぐため | 完了（firmware/app/stackchan/session_protocol.cpp） |
| P12-05 | audio uplink 実装を分離 | session_audio_upload.cpp、sendAudioStream 周辺の整理 | 中 | uplink と playback の責務を分け、音声入出力の変更点を追いやすくするため | 完了（firmware/app/stackchan/session_audio_upload.cpp） |
| P12-06 | TTS ストリーム処理を分離 | session_tts_stream.cpp、queue / concealment / watermark / opus decode | 高 | 現在もっとも状態量が多く、保守負荷が高い領域であるため | 完了（firmware/app/stackchan/session_tts_stream.cpp） |
| P12-07 | Avatar / motion 表示処理を分離 | session_avatar.cpp、expression / motion / lip sync 更新 | 中 | hardware control と avatar behavior の分離方針に沿って見通しを改善するため | 完了（firmware/app/stackchan/session_avatar.cpp） |
| P12-08 | TTS ストリーム処理を補助モジュール化 | helper 構造体または内部クラス、所有権整理 | 中 | file split だけでは残る複雑性を段階的に縮退させるため | 完了（session.h に TTSStreamContext 補助構造体を追加、session_tts_stream.cpp / session_protocol.cpp のアクセスパスを _tts.xxx に変更） |
| P12-09 | 受信イベントルータの見直し | event handler table または整理済み dispatch | 中 | device 系イベント追加時の条件分岐増殖を防ぐため | 完了（session_protocol.cpp の onTextMessage() を route table ベースへ整理） |
| P12-10 | 回帰確認項目を整備 | チェックリスト、必要なら最小テスト追加、確認手順 | 高 | リファクタ単独フェーズでは機能追加より回帰防止が成果そのものになるため | 完了（本ドキュメント内に回帰確認チェックリストと確認手順を追記） |
| P12-11 | リファクタ結果を docs へ反映 | Phase 11 への引き継ぎメモ、設計境界、既知制約 | 中 | 以後の hardware service 分離が同じ前提で進められるようにするため | 完了（本ドキュメント内に完了サマリ、設計境界、既知制約、Phase 11 引き継ぎ詳細を追記） |

## 5. 実施順の推奨

### スライス A: file split だけを完了する

- P12-01
- P12-02
- P12-03
- P12-04
- P12-05
- P12-06
- P12-07

成功条件：

- session.h の public API を変えずにビルドできる
- 主要フローの外部挙動が変わらない
- session.cpp 本体が「入口 + 最小 orchestration」へ縮小される

### スライス B: 複雑な内部状態を縮退する

- P12-08
- P12-09

成功条件：

- TTS stream と event dispatch の責務境界が明文化される
- Phase 11 の device event 追加時に、巨大な if / else の延命にならない

### スライス C: 回帰確認と引き継ぎ

- P12-10
- P12-11

成功条件：

- 回帰確認手順が明文化される
- Phase 11 側で参照すべき内部境界がドキュメント化される

### P12-10 回帰確認チェックリスト

P12-03〜P12-09 の内部リファクタ後に、以下の順で確認する。
本チェックリストは「session.cpp の責務分割後も外部挙動が変わっていないこと」を確認するための最小セットである。

1. ビルド確認
- コマンド: `mise run fw:build`
- 合格条件: `SUCCESS` が出力され、`firmware.elf` と `firmware.bin` が生成されること
- 実施メモ: 2026-03-19 時点で本コマンドは成功済み

2. 起動直後の接続シーケンス確認
- 観点: `begin()` -> Wi-Fi 接続 -> WebSocket 接続 -> `session.hello` 送信 -> `session.welcome` 受信 -> `Active` 遷移
- 合格条件: `welcome accepted -> Active` ログが出て、Avatar 表示が `Ready` になること

3. heartbeat 継続送信確認
- 観点: `Active` 状態で heartbeat が一定間隔で送信されること
- 合格条件: heartbeat ログが継続的に出ること、切断時に送信が暴走しないこと

4. audio uplink 確認
- 観点: `sendAudioStream()` 実行で `audio.stream_open` -> binary frame 群 -> `audio.end` が順に送信されること
- 合格条件: `Listening` -> `Thinking` へ遷移し、送信失敗時に異常終了しないこと

5. TTS PCM 一括再生確認
- 観点: 旧方式（`audio_base64` を `tts.end` に含む経路）が引き続き再生できること
- 合格条件: `tts.end playback started` ログが出て、再生完了後に `Speaking` -> `Idle` へ戻ること

6. TTS ストリーム再生確認
- 観点: `tts.chunk` の連続受信で prebuffer 到達後に再生開始し、`tts.end` は drain 完了マーカーとして扱われること
- 合格条件: `prebuffer ready` と `playback` ログが出て、stream 終了後に `tts stream drained` へ到達すること

7. watermark / バッファ深さ確認
- 観点: low-water / high-water / normal の各状態が条件どおり送信されること
- 合格条件: 同一状態の連続送信が cooldown により抑制され、復帰時に `normal` が送信されること

8. interrupt 系確認
- 観点: `conversation.cancel` / `tts.stop` / `audio.stream_abort` で停止手順が共通に維持されていること
- 合格条件: `_ttsPlayer.stop()`、TTS queue 破棄、incoming buffer 破棄が行われ、最終的に `Idle` へ戻ること

9. Avatar / motion 確認
- 観点: `avatar.expression` と `motion.play` の反映、および再生中の lip sync が維持されること
- 合格条件: expression 変更、nod/shake 演出、再生中の mouth open 更新が観測できること

10. 再接続確認
- 観点: Wi-Fi または WebSocket 切断後に `ConnectingWS` へ戻り、再度 hello/welcome を完了できること
- 合格条件: 再接続後に `_seq.reset()` が効いた新規セッションとして復帰すること

11. 重点メモリ所有権確認
- 観点: TTS stop / stream abort / disconnect 後に queue / incoming buffer / opus decoder の cleanup が走ること
- 合格条件: 二重解放や解放漏れを疑うクラッシュがないこと、連続会話でも再生が継続できること

12. 最終受け入れ判定
- 合格条件: 上記 1〜11 のうち必須項目 1〜8 がすべて通過し、P12-03〜P12-09 の file split / module split が外部挙動を壊していないと判断できること

### P12-10 実施メモ

- 自動テストの新規追加は今回未実施
- 理由: 現時点の firmware 側にはセッション処理を単体で差し替え検証できるテスト土台が未整備であり、今回の最小成果は「反復可能な回帰手順の明文化」に置くため
- 次段階でテストを追加する場合は、`Protocol::buildEnvelope` 入出力、event dispatch、TTSStreamContext の queue 操作を優先候補とする

### P12-11 リファクタ結果サマリ

Phase 12 で実施した構造変更は以下のとおり。

1. `session.cpp` を「入口 + 最小 orchestration」へ縮小
- 残存責務: constructor、`begin()`、`loop()`
- 目的: ランタイム全体の進行管理だけを残し、個別機能の差分を局所化する

2. 接続責務を分離
- 実装先: `session_connection.cpp`
- 主な関数: `setState()`、`onWSConnected()`、`onWSDisconnected()`
- 効果: 再接続・セッション初期化まわりの差分が他責務へ波及しにくくなった

3. protocol 送受信責務を分離
- 実装先: `session_protocol.cpp`
- 主な関数: `sendHello()`、`sendHeartbeat()`、`sendTTSBufferWatermark()`、`onTextMessage()`
- 効果: protocol event 追加時の変更点が routing 層へ集約された

4. audio uplink を分離
- 実装先: `session_audio_upload.cpp`
- 主な関数: `sendAudioStream()`
- 効果: uplink と downlink/TTS の変更点を独立して追跡できるようになった

5. TTS ストリーム処理を分離
- 実装先: `session_tts_stream.cpp`
- 主な関数: chunk/end 処理、queue 管理、concealment、Opus decode、playback queue
- 効果: もっとも状態量の多い領域を 1 ファイルへ閉じ込め、所有権追跡を容易にした

6. Avatar / motion を分離
- 実装先: `session_avatar.cpp`
- 主な関数: `handleAvatarExpression()`、`handleMotionPlay()`、`updateAvatarFace()`、`setConversationState()`
- 効果: hardware control 候補と avatar behavior の責務を混線させずに拡張できるようになった

7. TTS 専用状態を `TTSStreamContext` に集約
- 実装先: `session.h`
- 効果: queue / incoming buffer / decoder / watermark / concealment の所有権境界が明確になり、将来の helper class 化へ進みやすくなった

8. 受信イベントルータを route table ベースへ整理
- 実装先: `session_protocol.cpp`
- 効果: `device.*` イベント追加時に if / else 連鎖を延命せず、ルータの変更点をテーブルに限定できるようになった

### P12-11 設計境界（Phase 11 以降で守る線引き）

Phase 11 以降で `device.*` 系イベントや hardware service を追加する際は、以下の境界を維持する。

1. `session.cpp`
- 役割: 進行管理のみ
- 禁止: JSON parse、device 固有制御、TTS queue 内部操作の再流入

2. `session_connection.cpp`
- 役割: Wi-Fi / WebSocket / session lifecycle
- 禁止: device event の個別処理、audio / avatar / protocol payload 詳細の混在

3. `session_protocol.cpp`
- 役割: envelope parse、route、送信 helper
- 許容: handler への委譲、route table 追加
- 禁止: queue 配列や avatar 状態の直接更新ロジックを厚く持つこと

4. `session_audio_upload.cpp`
- 役割: uplink 専用
- 禁止: downlink 再生制御や device command dispatch の混在

5. `session_tts_stream.cpp`
- 役割: downlink 音声、queue、decoder、concealment、watermark
- 禁止: protocol routing の再実装、接続ライフサイクルの再流入

6. `session_avatar.cpp`
- 役割: 表示、motion 演出、会話状態表示との同期
- 禁止: TTS queue / decoder / binary stream の内部状態操作

### P12-11 既知制約

現時点で解消していない制約を以下に固定する。

1. firmware 側の自動テスト土台は未整備
- 回帰確認は現在 runbook/チェックリスト中心で運用する

2. `sendAudioStream()` はブロッキング送信
- `delay(FW_AUDIO_FRAME_MS)` を含むため、将来の低遅延化ではタスクベース化が必要

3. TTS 旧方式と新方式が併存
- `tts.end` 一括再生経路と `tts.chunk` ストリーム経路の双方をまだ保持している

4. interrupt 系停止処理は個別 handler に重複がある
- `conversation.cancel` / `tts.stop` / `audio.stream_abort` は今後共通 helper 化の余地がある

5. route table 化は最小整理に留めている
- handler シグネチャ差（payload のみ / envelope session_id 付き）のため、完全一体化はしていない

6. `TTSStreamContext` は helper 構造体であり独立クラスではない
- 所有権境界は改善したが、振る舞いとデータはまだ `StackchanSession` に所属している

### P12-11 Phase 11 引き継ぎ詳細

Phase 11 の hardware 拡張を再開する際は、以下の順で実施する。

1. `device.*` event の追加先は `session_protocol.cpp` の route table に限定する
- まず route を追加し、処理本体は専用 handler へ委譲する

2. device service は avatar / TTS と混ぜない
- servo、LED、touch、camera は `session_avatar.cpp` や `session_tts_stream.cpp` に直接書き込まない

3. hardware command router を導入する場合の基準
- protocol routing 層: event 名で分岐する
- device service 層: 実機制御 API を提供する
- session 層: 状態遷移と service 呼び出し順序だけを管理する

4. 新規 handler 追加時の判断基準
- protocol 解釈だけなら `session_protocol.cpp`
- 画面表示や最小演出なら `session_avatar.cpp`
- 音声再生・バッファ・decoder に触るなら `session_tts_stream.cpp`
- uplink 送信だけなら `session_audio_upload.cpp`

5. 今後の優先改善候補
- interrupt 停止処理の共通 helper 化
- `TTSStreamContext` の独立 helper / class 化
- device command router と hardware service の明示的な分離
- protocol routing の table 拡張時に compile-time safety を高める仕組みの導入

## 6. 受け入れ方針（フェーズ 12）

- session.h の public API を壊さずに session.cpp の実装責務が分割されていること
- hello / welcome / heartbeat / sendAudioStream / tts playback / interrupt の既存挙動が維持されること
- TTS ストリーム処理のメモリ所有権と cleanup タイミングが追跡しやすくなっていること
- Avatar 表示更新と protocol 受信処理が、今後の hardware service 追加を妨げない構造になっていること
- Phase 11 の hardware command router と device service 導入が、このフェーズの整理結果を前提に進められること

## 7. Phase 11 への引き継ぎ

- session.cpp 本体には orchestration と委譲だけを残す
- device.* event の追加先は、整理済みの protocol routing 層へ載せる
- servo / led / touch / camera の service は、TTS や avatar の責務と混ぜない
- hardware control と avatar behavior の分離を、構造レベルで維持する

---

本ドキュメントは初版です。実装が進んだら、実際の分割単位、helper 構成、検証手順、Phase 11 への依存関係に合わせて更新してください。