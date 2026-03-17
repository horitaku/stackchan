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
| P8-04 | runtime_metrics 永続化と可視化連携を拡張する | metrics 保存 API/処理、取得 API 拡張、運用確認手順 | 中 | 可観測性の履歴分析を可能にするため | Done |
| P8-05 | WebSocket binary Opus パスの統合を進める | binary 音声フレーム受信処理、検証ログ、互換性メモ | 中 | 低遅延化に重要だが、現行 PCM パスで最小運用は可能なため | Done |
| P8-06 | 障害復旧ランブックを整備する | 接続断、provider 遅延、設定不整合の復旧手順 | 中 | 運用時 MTTR を短縮するため | Done |
| P8-07 | firmware で M5Stack-Avatar の顔表示を先行実装する | 初期顔描画、表情切替 API、描画ループ統合、手動確認手順 | 高 | デバイス体験価値を早期に確認し、以降の音声同期実装の土台にするため | Done |
| P8-08 | interrupt 系イベントを protocol へ正式追加する | `conversation.cancel` / `tts.stop` / `audio.stream_abort` の schema・example・互換性メモ | 高 | 割り込み制御を後付けにすると firmware/server 双方の手戻りが大きいため | Done |
| P8-09 | firmware に最小 conversation 状態遷移を実装する | `idle/listening/thinking/speaking/interrupted/error` の状態管理、遷移ログ、手動確認手順 | 高 | 体験品質とランタイム境界を architecture 定義と一致させるため | Planned |
| P8-10 | Opus 経路の計測項目を runtime metrics へ追加する | first frame latency、cadence jitter、E2E latency の収集/公開/API 反映 | 高 | 低遅延最適化の判断を定量化し、phase8 受け入れ判定を明確化するため | Planned |
| P8-11 | Docker compose に Voicevox を追加し TTS 環境を前倒し整備する | `voicevox` サービス定義、server との接続設定、起動確認手順、トラブルシュートメモ | 高 | TTS 実機連携を早期検証し、後続の音声品質評価と遅延計測の前提を整えるため | Done |
| P8-12 | WebUI から Voicevox を使った UI 単体テスト導線を追加する | テスト実行 UI、入力テキスト指定、再生/ダウンロード確認、失敗時エラー表示、手順書 | 高 | Stackchan 非接続でも TTS の健全性を先に切り分け可能にするため | Done |
| P8-13 | WebUI から Voicevox を使った Stackchan 連携テスト導線を追加する | Stackchan 宛て送信テスト API/UI、再生結果確認、遅延/失敗表示、確認手順 | 高 | 実デバイス連携時の音声経路を早期に検証し、運用前の不具合を先に発見するため | Done |
| P8-14 | tts.chunk を音声フレーム単位へ再設計する | `stream_id` / `chunk_index` / `sent_at or playout_ts` / `frame_duration_ms` / `samples_per_chunk` を含む payload 定義、schema、互換性メモ | 中 | 現在の固定 byte 分割では低遅延再生と欠落検知に不向きなため | Planned |
| P8-15 | firmware に事前バッファ付き再生パイプラインを導入する | 60〜120ms 事前バッファ、low-water/high-water 管理、リングバッファ消費、手動確認手順 | 中 | `tts.chunk` を受信しながら安定再生するための吸収機構が必要なため | Planned |
| P8-16 | tts.chunk の欠落/遅延検知と concealment 方針を導入する | sequence/timestamp 判定、欠落時の減衰コピーまたは無音補完、運用メトリクス | 中 | Wi-Fi 揺れや再送遅延で再生グリッチが目立つのを抑えるため | Planned |
| P8-17 | 音声再生処理を専用消費ループへ分離する | 通信受信と再生処理の分離、バッファ監視、lip sync 更新点の見直し | 中 | main loop 直結のままでは将来の低遅延再生で詰まりやすいため | Planned |

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

- ローカル環境に Docker を導入し、`P8-01` のローカル動作検証を再開しました。
- `P8-07` 先行で確立したデバイス体験を維持しつつ、phase8 の運用タスクを順次前倒しします。

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

## 2.5 P8-01 ローカル再実行メモ（2026-03-15）

- `mise` から Docker compose を操作できるよう、次のタスクを追加しました。
  - `infra:build`
  - `infra:up`
  - `infra:down`
  - `infra:ps`
  - `infra:logs`
- Docker build の `vite: Permission denied` は、`server/webui/node_modules` が build context に含まれていたことが原因でした。
  - リポジトリルートに `.dockerignore` を追加し、`**/node_modules` と `**/dist` を除外しました。
- 再実行結果:
  - `mise run infra:up` 成功
  - `mise run infra:ps` で `stackchan-server` / `stackchan-db` の起動を確認
  - `mise run server:healthz` で `STATUS=200` を確認

## 2.6 P8-11 Voicevox 前倒し整備メモ（2026-03-15）

- `infra/docker/docker-compose.yml` に `voicevox` サービスを追加しました。
  - image: `voicevox/voicevox_engine:cpu-ubuntu20.04-latest`
  - port: `50021:50021`
- `stackchan-server` の `depends_on` に `voicevox` を追加しました（`service_started`）。
- 検証結果:
  - `mise run infra:up` で `stackchan-voicevox` を含む全サービス起動に成功
  - `mise run infra:ps` で `stackchan-voicevox` の稼働を確認
  - `curl -fsS http://127.0.0.1:50021/version` が `"latest"` を返却
  - `mise run server:healthz` が `STATUS=200` を返却

## 2.7 P8-12 WebUI UI 単体テスト導線メモ（2026-03-15）

- server に `POST /api/tests/voicevox/ui` を追加し、Voicevox の `audio_query` / `synthesis` を呼び出して音声を返す API を実装しました。
- WebUI に Voicevox UI 単体テストパネルを追加しました。
  - 入力テキスト
  - speaker 指定
  - テスト実行
  - 生成音声の即時再生
  - JSON 結果表示
- `.env` を作成し、ローカル検証用の `VOICEVOX_BASE_URL` と `DATABASE_URL` を設定しました。
- 検証結果:
  - `go test ./...` 成功
  - `npm run build` 成功
  - `POST /api/tests/voicevox/ui` が `audio_base64` を返却することを確認

## 2.8 P8-13 WebUI Stackchan 連携テスト導線メモ（2026-03-15）

- server に `POST /api/tests/voicevox/stackchan` を追加しました。
  - Voicevox で生成した音声を接続中の Stackchan セッションへ `tts.end` として送信します。
  - 視認性向上のため `avatar.expression` と `motion.play` も同時送信します。
- WebUI に Stackchan 連携テストパネルを追加しました。
  - 入力テキスト
  - speaker / expression / motion
  - 連携テスト実行
  - JSON 結果表示
- 実行時の確認結果:
  - `go test ./...` 成功
  - `npm run build` 成功
  - Stackchan 未接続時は `{"error":"no active Stackchan session is connected"}` を返却
  - 接続断時は write エラーを返却し、server 側でアクティブセッションを自動クリア

## 2.9 音声 chunking 次段階の引き継ぎ事項（2026-03-15）

- 現在の `tts.chunk` は「巨大な `tts.end` を避けるための安全配送」が主目的であり、低遅延ストリーミング再生はまだ未着手です。
- 現状の制約:
  - chunk は音声フレーム単位ではなく固定 byte 分割です。
  - firmware は `tts.chunk` を全受信してから `tts.end` で再生開始します。
  - jitter 吸収、欠落補完、playout timestamp 管理は未実装です。
- 次段階で優先する項目:
  - `tts.chunk` に `stream_id`、`frame_duration_ms`、`samples_per_chunk`、`sent_at` または `playout_ts` を追加する
  - 20ms 単位を基本とした音声フレーム設計へ寄せる
  - firmware に 60〜120ms の事前バッファを導入する
  - low-water / high-water を持つリングバッファ再生へ移行する
  - 欠落時は停止待ちではなく concealment（減衰コピーまたは無音補完）を先に導入する
- 補足:
  - 現 transport は WebSocket/TCP なので、初期優先度は FEC よりも sequence/timestamp と事前バッファです。
  - browser/WebUI 側の AudioWorklet 相当の議論は、firmware では「通信受信と再生消費の分離タスク化」に読み替えて扱います。

## 2.10 P8-04 runtime_metrics 永続化メモ（2026-03-17）

- server に runtime metrics 永続化ストアを追加し、`DATABASE_URL` が設定されている場合に Postgres へ保存するようにしました。
- `server/internal/web/runtime_state.go` の主要更新点で `runtime_metrics` へメトリクスを書き込む処理を追加しました。
  - connection: `connection_count` / `reconnect_count`
  - pipeline: `queue_wait_ms` / `stt_latency_ms` / `llm_latency_ms` / `tts_latency_ms` / `total_latency_ms`
  - playback: `playback_start_latency_ms` / `playback_duration_ms` / `decode_error_count` / `output_error_count`
- API 拡張として `GET /api/runtime/metrics` を追加しました。
  - クエリ: `session_id` / `request_id` / `metric_name` / `from` / `to` / `limit`
  - `from` / `to` は RFC3339 形式
- `DATABASE_URL` 未設定時は、既存動作を維持しつつ runtime metrics 永続化を無効化するようにしています。

## 2.12 P8-06 障害復旧ランブック整備メモ（2026-03-17）

- `docs/runbooks/` ディレクトリを新設し、3 本のランブックと統合インデックスを追加しました。
  - [connection-failure.md](../runbooks/connection-failure.md): Wi-Fi 断、WebSocket 接続失敗、ハンドシェイク失敗、heartbeat タイムアウト、サーバー再起動後の復旧
  - [provider-latency.md](../runbooks/provider-latency.md): STT/LLM/TTS タイムアウト、Voicevox 停止、レート制限、リトライ設定調整
  - [configuration-mismatch.md](../runbooks/configuration-mismatch.md): 環境変数ミス、DB 接続エラー、API キー不正、firmware 設定ミス、CORS エラー、metrics 永続化無効
  - [README.md](../runbooks/README.md): クイックチェックコマンド、ログ確認手順、エンドポイット早見表
- 各ランブックには実際のログ出力例、診断コマンド、復旧手順、エスカレーション基準を記載しました。
- `GET /api/runtime/overview` / `GET /api/runtime/metrics` を活用した定量的な確認手順を組み込みました。
- `docs/runbooks/configuration-mismatch.md` §9 に起動前設定チェックリスト（ワンライナー）を追加しました。

## 2.11 P8-05 WebSocket binary Opus パス統合メモ（2026-03-17）

- server 側に音声ユーティリティを追加しました（`server/internal/audio/`）。
  - codec 検証（`pcm`/`opus`）
  - フレームサイズ検証（PCM 期待値、Opus 最小サイズ）
  - STT 入力に使えるコンテナ生成ヘルパー（Opus OGG / PCM WAV）
- `server/internal/web/ws_handler.go` の更新:
  - `audio.stream_open` で codec の妥当性を検証
  - binary 受信時に `stream_id` / `codec` / `payload_bytes` / `frame_index` をログ出力
  - first frame 到達時に `first_frame_latency_ms` をログ出力
  - `audio.stream_open` 前に binary が到達した場合は warning を明示
- `server/internal/session/audio_stream.go` の更新:
  - `BinaryStreamMeta` に `OpenedAt` と `FirstFrameAt` を追加
  - `AudioChunk` に `ReceivedAt` を保持（server 内部計測用）
- 互換性方針:
  - 既存 PCM binary 経路は維持（後方互換）
  - Opus は binary payload で受信可能
  - STT provider 側へ渡す直前の decode/変換は次段階（P8-10）で統合

## 2.13 P8-08 interrupt 系イベント正式追加メモ（2026-03-18）

- protocol に interrupt 系 3 イベントを正式追加しました。
  - `conversation.cancel`
  - `tts.stop`
  - `audio.stream_abort`
- 追加成果物:
  - `protocol/websocket/schemas/conversation.cancel.schema.json`
  - `protocol/websocket/schemas/tts.stop.schema.json`
  - `protocol/websocket/schemas/audio.stream_abort.schema.json`
  - `protocol/examples/conversation.cancel.example.json`
  - `protocol/examples/tts.stop.example.json`
  - `protocol/examples/audio.stream_abort.example.json`
- ドキュメント更新:
  - `protocol/websocket/events.md` に定義、error semantics、互換性メモを追記
  - `protocol/websocket/validation-checklist.md` にイベント別検証項目を追記
  - `protocol/versioning.md` に phase8 追加の運用方針を追記

## 3. フェーズ 7 からの前提条件

- サーバー API（runtime overview / settings / pipeline test）は実装済み。
- WebUI は Svelte ビルド成果物を Go サーバーから /ui 配信可能。
- server 側の主要自動テストは通過済み。
- フェーズ 8 では「統合再現性」「継続検証」「運用復旧性」を優先する。

## 4. 受け入れ方針（フェーズ 8）

- Docker と CI で、手元依存の少ない再現手順を先に作る。
- DB 変更は migration 前提で管理し、手動 SQL 運用を避ける。
- 可観測性は最新値中心から履歴中心へ段階拡張する。
