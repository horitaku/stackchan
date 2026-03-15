# フェーズ 8 タスクリスト（統合と運用）

## 1. このドキュメントの目的

フェーズ 8（統合と運用）を実行しやすくするために、初期バックログを整理します。
本ドキュメントはフェーズ 7 の引き継ぎ事項を具体タスクへ展開した着手版です。

## 2. 実行タスクリスト（初期バックログ）

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P8-01 | Docker マルチステージ本番導線を確立する | WebUI build -> Go build -> 最小 runtime までの一貫 Dockerfile、compose 更新 | 高 | ローカル/本番相当の再現性を最優先で確保するため | Done |
| P8-02 | CI で server テストと WebUI build を必須化する | GitHub Actions の test/build ジョブ、失敗時ログ導線 | 高 | 回帰混入を早期検知するため | Done |
| P8-03 | DB マイグレーション基盤を導入する | migration ツール設定、初期スキーマ、運用手順 | 高 | 永続化機能の先行条件であり後戻りコストが高いため | Done |
| P8-04 | runtime_metrics 永続化と可視化連携を拡張する | metrics 保存 API/処理、取得 API 拡張、運用確認手順 | 中 | 可観測性の履歴分析を可能にするため | Planned |
| P8-05 | WebSocket binary Opus パスの統合を進める | binary 音声フレーム受信処理、検証ログ、互換性メモ | 中 | 低遅延化に重要だが、現行 PCM パスで最小運用は可能なため | Planned |
| P8-06 | 障害復旧ランブックを整備する | 接続断、provider 遅延、設定不整合の復旧手順 | 中 | 運用時 MTTR を短縮するため | Planned |
| P8-07 | firmware で M5Stack-Avatar の顔表示を先行実装する | 初期顔描画、表情切替 API、描画ループ統合、手動確認手順 | 高 | デバイス体験価値を早期に確認し、以降の音声同期実装の土台にするため | Done |
| P8-08 | interrupt 系イベントを protocol へ正式追加する | `conversation.cancel` / `tts.stop` / `audio.stream_abort` の schema・example・互換性メモ | 高 | 割り込み制御を後付けにすると firmware/server 双方の手戻りが大きいため | Planned |
| P8-09 | firmware に最小 conversation 状態遷移を実装する | `idle/listening/thinking/speaking/interrupted/error` の状態管理、遷移ログ、手動確認手順 | 高 | 体験品質とランタイム境界を architecture 定義と一致させるため | Planned |
| P8-10 | Opus 経路の計測項目を runtime metrics へ追加する | first frame latency、cadence jitter、E2E latency の収集/公開/API 反映 | 高 | 低遅延最適化の判断を定量化し、phase8 受け入れ判定を明確化するため | Planned |
| P8-11 | Docker compose に Voicevox を追加し TTS 環境を前倒し整備する | `voicevox` サービス定義、server との接続設定、起動確認手順、トラブルシュートメモ | 高 | TTS 実機連携を早期検証し、後続の音声品質評価と遅延計測の前提を整えるため | Planned |
| P8-12 | WebUI から Voicevox を使った UI 単体テスト導線を追加する | テスト実行 UI、入力テキスト指定、再生/ダウンロード確認、失敗時エラー表示、手順書 | 高 | Stackchan 非接続でも TTS の健全性を先に切り分け可能にするため | Planned |
| P8-13 | WebUI から Voicevox を使った Stackchan 連携テスト導線を追加する | Stackchan 宛て送信テスト API/UI、再生結果確認、遅延/失敗表示、確認手順 | 高 | 実デバイス連携時の音声経路を早期に検証し、運用前の不具合を先に発見するため | Planned |

## 2.1 実行メモ（2026-03-15）

- P8-01
  - `infra/docker/Dockerfile` を追加し、`webui-builder` -> `server-builder` -> `runtime` の 3 ステージでビルド可能にしました。
  - `infra/docker/docker-compose.yml` を追加し、`stackchan-server` と `postgres` を同時起動できるようにしました。
- P8-02
  - `.github/workflows/ci.yml` を追加し、`server` の `go test`、`server/webui` の `npm run build`、Docker イメージビルドを CI 必須化しました。
- P8-03
  - `infra/migrations/000001_initial_schema.up.sql` / `down.sql` を追加し、`sessions`、`utterances`、`conversation_events`、`runtime_metrics`、`system_settings` の初期スキーマを導入しました。
  - `infra/migrations/README.md` に `golang-migrate` 実行手順を記載しました。

## 2.2 優先順メモ（2026-03-15 更新）

- ローカル環境に Docker が未導入のため、`P8-01` のローカル動作検証は後回しにします。
- 先行着手は `P8-07`（firmware の M5Stack-Avatar 顔表示）とし、デバイス表示体験を先に固めます。

## 2.3 P8-07 実行メモ（2026-03-15）

- `firmware/app/stackchan/session.h` に `m5avatar::Avatar` を導入し、セッション管理クラスの責務として保持しました。
- `firmware/app/stackchan/session.cpp` で Avatar 初期化（`init`）と neutral 顔の起動表示を追加しました。
- `avatar.expression` 受信時に m5stack-avatar の `Expression` へ変換して反映する処理を追加しました。
- `tts.end` 再生中の lip level を `setMouthOpenRatio` へ接続し、口パクを描画へ反映しました。

## 2.4 追加タスクの着手条件（2026-03-15）

- P8-08 interrupt 正式化
  - `protocol/websocket/events.md` にイベント定義の方向性と error semantics を追記済みであること
  - `protocol/websocket/schemas/` と `protocol/examples/` に追加先の配置方針が合意済みであること
- P8-09 firmware 状態遷移
  - `docs/architecture/conversation-state-machine.md` を基準仕様として採用済みであること
  - 既存の再生/表情実装（P8-07）を壊さない最小統合方針を確認済みであること
- P8-10 Opus 計測項目
  - `P8-05` の binary Opus パス統合タスクと連携し、計測点（受信時刻、再生開始時刻）が取得可能であること
  - `runtime_metrics` 永続化（P8-04）との項目名・単位を先に合意していること
- P8-11 Voicevox 前倒し整備
  - `infra/docker/docker-compose.yml` にサービス追加可能なネットワーク/ポート方針が合意済みであること
  - server 側の `VOICEVOX_BASE_URL` 設定値と compose サービス名の対応を確認済みであること
- P8-12 WebUI UI 単体テスト
  - `P8-11` 完了後に Voicevox API への疎通がローカル compose 上で確認できていること
  - WebUI 側でテスト入力と結果表示の最小 UI 追加方針が合意済みであること
- P8-13 WebUI Stackchan 連携テスト
  - `P8-12` の UI 単体テストで音声生成の成功が確認できていること
  - Stackchan 接続状態を確認する API（または既存 runtime overview）と連携判定条件が合意済みであること

## 3. フェーズ 7 からの前提条件

- サーバー API（runtime overview / settings / pipeline test）は実装済み。
- WebUI は Svelte ビルド成果物を Go サーバーから /ui 配信可能。
- server 側の主要自動テストは通過済み。
- フェーズ 8 では「統合再現性」「継続検証」「運用復旧性」を優先する。

## 4. 受け入れ方針（フェーズ 8）

- Docker と CI で、手元依存の少ない再現手順を先に作る。
- DB 変更は migration 前提で管理し、手動 SQL 運用を避ける。
- 可観測性は最新値中心から履歴中心へ段階拡張する。
