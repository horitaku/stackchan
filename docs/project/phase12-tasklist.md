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
| P12-03 | connection / lifecycle 実装を分離 | session_connection.cpp、接続関連ロジック整理 | 中 | begin / loop / heartbeat を他責務から分け、以後の差分を局所化するため | 未着手 |
| P12-04 | protocol send / receive 実装を分離 | session_protocol.cpp、dispatch 整理、handler 配置 | 高 | 受信イベント拡張時に session.cpp 本体が再肥大化するのを防ぐため | 未着手 |
| P12-05 | audio uplink 実装を分離 | session_audio_upload.cpp、sendAudioStream 周辺の整理 | 中 | uplink と playback の責務を分け、音声入出力の変更点を追いやすくするため | 未着手 |
| P12-06 | TTS ストリーム処理を分離 | session_tts_stream.cpp、queue / concealment / watermark / opus decode | 高 | 現在もっとも状態量が多く、保守負荷が高い領域であるため | 未着手 |
| P12-07 | Avatar / motion 表示処理を分離 | session_avatar.cpp、expression / motion / lip sync 更新 | 中 | hardware control と avatar behavior の分離方針に沿って見通しを改善するため | 未着手 |
| P12-08 | TTS ストリーム処理を補助モジュール化 | helper 構造体または内部クラス、所有権整理 | 中 | file split だけでは残る複雑性を段階的に縮退させるため | 未着手 |
| P12-09 | 受信イベントルータの見直し | event handler table または整理済み dispatch | 中 | device 系イベント追加時の条件分岐増殖を防ぐため | 未着手 |
| P12-10 | 回帰確認項目を整備 | チェックリスト、必要なら最小テスト追加、確認手順 | 高 | リファクタ単独フェーズでは機能追加より回帰防止が成果そのものになるため | 未着手 |
| P12-11 | リファクタ結果を docs へ反映 | Phase 11 への引き継ぎメモ、設計境界、既知制約 | 中 | 以後の hardware service 分離が同じ前提で進められるようにするため | 未着手 |

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