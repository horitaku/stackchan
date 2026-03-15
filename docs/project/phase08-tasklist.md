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

## 3. フェーズ 7 からの前提条件

- サーバー API（runtime overview / settings / pipeline test）は実装済み。
- WebUI は Svelte ビルド成果物を Go サーバーから /ui 配信可能。
- server 側の主要自動テストは通過済み。
- フェーズ 8 では「統合再現性」「継続検証」「運用復旧性」を優先する。

## 4. 受け入れ方針（フェーズ 8）

- Docker と CI で、手元依存の少ない再現手順を先に作る。
- DB 変更は migration 前提で管理し、手動 SQL 運用を避ける。
- 可観測性は最新値中心から履歴中心へ段階拡張する。
