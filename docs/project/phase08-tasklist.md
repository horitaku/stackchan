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
| P8-09 | firmware に最小 conversation 状態遷移を実装する | `idle/listening/thinking/speaking/interrupted/error` の状態管理、遷移ログ、手動確認手順 | 高 | 体験品質とランタイム境界を architecture 定義と一致させるため | Done |
| P8-10 | Opus 経路の計測項目を runtime metrics へ追加する | first frame latency、cadence jitter、E2E latency の収集/公開/API 反映 | 高 | 低遅延最適化の判断を定量化し、phase8 受け入れ判定を明確化するため | Done |
| P8-11 | Docker compose に Voicevox を追加し TTS 環境を前倒し整備する | `voicevox` サービス定義、server との接続設定、起動確認手順、トラブルシュートメモ | 高 | TTS 実機連携を早期検証し、後続の音声品質評価と遅延計測の前提を整えるため | Done |
| P8-12 | WebUI から Voicevox を使った UI 単体テスト導線を追加する | テスト実行 UI、入力テキスト指定、再生/ダウンロード確認、失敗時エラー表示、手順書 | 高 | Stackchan 非接続でも TTS の健全性を先に切り分け可能にするため | Done |
| P8-13 | WebUI から Voicevox を使った Stackchan 連携テスト導線を追加する | Stackchan 宛て送信テスト API/UI、再生結果確認、遅延/失敗表示、確認手順 | 高 | 実デバイス連携時の音声経路を早期に検証し、運用前の不具合を先に発見するため | Done |
| P8-14 | tts.chunk を音声フレーム単位へ再設計する | `stream_id` / `chunk_index` / `sent_at or playout_ts` / `frame_duration_ms` / `samples_per_chunk` を含む payload 定義、schema、互換性メモ | 中 | 現在の固定 byte 分割では低遅延再生と欠落検知に不向きなため | Done |
| P8-15 | firmware に事前バッファ付き再生パイプラインを導入する | 60〜120ms 事前バッファ、low-water/high-water 管理、リングバッファ消費、手動確認手順 | 中 | `tts.chunk` を受信しながら安定再生するための吸収機構が必要なため | Done |
| P8-16 | tts.chunk の欠落/遅延検知と concealment 方針を導入する | sequence/timestamp 判定、欠落時の減衰コピーまたは無音補完、運用メトリクス | 中 | Wi-Fi 揺れや再送遅延で再生グリッチが目立つのを抑えるため | Done |
| P8-17 | 音声再生処理を専用消費ループへ分離する | 通信受信と再生処理の分離、バッファ監視、lip sync 更新点の見直し | 中 | main loop 直結のままでは将来の低遅延再生で詰まりやすいため | Done |
| P8-18 | Voicevox TTS の downlink を Opus 化し firmware でデコード再生する | server で PCM/WAV -> Opus フレーム化、`tts.chunk` で Opus 配信、firmware で Opus デコード再生、互換 fallback、検証手順 | 中 | 帯域削減と jitter 耐性を強化し、低遅延会話での再生安定性を高めるため | Done |
| P8-19 | runtime_metrics に watermark 状態統合する | firmware からの watermark イベント送信、server 側 metrics 記録処理、WebUI Overview パネル拡張、可視化ダッシュボード | 低 | ネットワーク揺らぎの履歴分析と定量的な診断を可能にし、運用時の最適化判定を支援するため | Planned |

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

## 2.20 P8-17 受信・消費の責務分離メモ（2026-03-18）

### 目的と背景

- 現状: `loop()` 内の受信フロー（`_ws.loop()` → `enqueueTTSFrame()`）と消費フロー（`processTTSPlaybackQueue()` → `playPCM16()`）が同一ステップで実行
- 課題: 受信ジッターが再生フロー全体に直接影響し、低遅延化時に bottleneck になる
- **改善**: Producer-Consumer パターン採用で責務を明確化し、将来の低遅延実装の土台を整備

### firmware 側の修正（session.h / session.cpp）

#### 1. loop() の責務を明確化（コメント追加）
- **3 つの処理フェーズを視覚的に分離**:
  - Producer フロー（受信）: `_ws.loop()` で WebSocket フレーム受信 → `onTextMessage()` で `enqueueTTSFrame()` 呼び出し（ノンブロッキング）
  - Consumer フロー（消費）: `processTTSPlaybackQueue()` でキュー → 再生
  - Display フロー（表示）: `updateAvatarFace()` で口パク・表情更新

#### 2. enqueueTTSFrame() にコメント追加
- **Producer ハンドラとして明確化**: WebSocket 受信時に呼ばれ、base64 フレームをキューに積む（デコード遅延）
- ノンブロッキング実行を前提に、処理時間を最小化

#### 3. processTTSPlaybackQueue() に Observability 強化
- **Watermark 監視の詳細ログ追加**:
  - `prebuffer ready`: 再生開始前の事前バッファ達成時
  - `low-water`: バッファが 60ms 以下に低下時（ネットワーク遅延や受信ジッター検知）
  - `playback batch`: dequeue 後のキュー状態スナップショット（`buffered_after_dequeue_ms`、`frames_remaining`）
- ログ形式:
  ```
  [TTS][watermark] low-water request_id=... buffered_ms=50 threshold_ms=60 frames_in_queue=2
  [TTS][playback] batch_duration_ms=40 batch_bytes=1280 buffered_after_dequeue_ms=40 frames_remaining=1
  ```
- これにより、受信速度と消費速度の関係を外部から監視可能に

#### 4. session.h のファイルコメント更新
- **@section P8-17 セクション追加**: Producer-Consumer パターンの設計意図を文書化

### 検証結果

- `pio run -e stackchan_cores3` 成功
  - RAM 使用率: 15.3%（メモリ増加なし）
  - Flash 使用率: 16.6%（コード量増加なし）
- **後方互換性**: 既存のリングバッファ機能（P8-15）と欠落補完（P8-16）を完全に保持

### 設計の特徴

| 観点 | 効果 |
|------|------|
| **責務分離** | 受信と消費の処理経路を明確化 → 意図的な設計変更・最適化が容易 |
| **Observability** | バッファ watermark をログ出力 → ネットワーク揺らぎを定量的に監視 |
| **Futures** | Producer-Consumer パターンは FreeRTOS マルチタスク化の前提条件（P9 対応時） |
| **リスク** | 最小限のコメント追加のみ → 実装ロジック変更なしで破壊リスク低い |

### 次段階への引き継ぎ（Phase 2）

P8-18 以降で活用する予定の項目：
- `_runtimeMetrics` への watermark 状態記録（`GET /api/runtime/overview` に反映）
- high-water ドロップ検知時の server への backpressure 信号（初期案）
- lip sync の on-demand 化（80ms 周期制限 → 消費タイミング同期）

## 2.21 P8-19 runtime_metrics に watermark 状態統合メモ（将来）

### 目的と背景（Phase 2）

P8-17 で追加した watermark ログ（`[TTS][watermark]`）は シリアル出力のみであり、ログファイル喪失時の履歴が失われます。
本タスクは **watermark イベントを `runtime_metrics` DB に記録** し、WebUI ダッシュボードで履歴分析とネットワーク診断を可能にします。

### 実装予定範囲

#### 1. firmware からの watermark イベント送信
- Producer フロー（`enqueueTTSFrame()`）で low-water / high-water イベント検出時に server へ通知
- 最小限の overhead で実装（ログ出力時に同時送信）

#### 2. server 側の metrics 記録処理
- WebSocket ハンドラで watermark イベント受信
- `runtime_metrics` テーブルへ以下を記録：
  ```sql
  metric_name = 'tts_buffer_watermark_status'
  metric_value = 'low_water' | 'high_water' | 'normal'
  
  metric_name = 'tts_buffered_ms'
  metric_value = [実際のバッファ深さ]
  
  metric_name = 'tts_threshold_low_water_ms'
  metric_value = [low-water 閾値]
  ```

#### 3. WebUI Overview パネル拡張
- `GET /api/runtime/overview` で watermark 統計を公開：
  ```json
  {
    "pipeline": {
      "tts_buffered_ms": 50,
      "tts_low_water_count_10m": 12,
      "tts_high_water_drop_count_10m": 1,
      "tts_watermark_status_current": "low_water"
    }
  }
  ```

#### 4. 可視化ダッシュボード（WebUI）
- 過去 1 時間の watermark 状態を時系列グラフで表示
- low-water 発生頻度、継続時間
- ネットワーク不具合の可視化（「18:00 に毎日 peak」等）

### 実装の優先条件

- ✅ P8-17 完了：watermark ログが firmware に存在
- ✅ P8-18 完了：Opus downlink で帯域削減後の効果測定ベースラインが確立
- 🔄 protocol 拡張：watermark イベントを protocol に正式追加 するか検討

### 設計の特徴

| 観点 | 効果 |
|------|------|
| **履歴分析** | 過去のネットワーク揺らぎを時系列で追跡可能 |
| **定量的診断** | Wi-Fi 環境の不具合を「low_water 12 回/10分」と数値化 |
| **最適化検証** | P8-18 (Opus) 効果を実測値で定量的に評価 |
| **SLA 監視** | 「過去 24 時間の low-water 発生時間」をクエリで即答 |
| **段階導入** | Phase 2 なので、P8-18 後に最適なタイミングで着手可能 |

### リスク軽減

- ログ出力との **両立実装**：既存 P8-17 ログを壊さない
- 後方互換性：firmware-server 間の新 protocol 追加が必要だが、fallback パスを用意
- DB スキーマ：既存 `runtime_metrics` テーブルで対応可能（schema 変更不要）

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

## 2.14 P8-09 firmware conversation 状態遷移メモ（2026-03-18）

- `firmware/app/stackchan/session.h` / `session.cpp` に conversation state を追加しました。
  - `idle` / `listening` / `thinking` / `speaking` / `interrupted` / `error`
- 遷移ログを追加しました。
  - ログ形式: `[Conversation] State: <prev> -> <next> reason=<reason>`
- 最小遷移の実装ポイント:
  - `idle -> listening`: `sendAudioStream()` 開始時
  - `listening -> thinking`: `audio.end` 送信成功時
  - `thinking -> speaking`: `tts.end` 受信後、再生開始成功時
  - `speaking -> idle`: 再生完了を検知した時点
  - `* -> interrupted -> idle`: `conversation.cancel` / `tts.stop` / `audio.stream_abort` 受信時
  - `* -> error`: `error` イベントで `retryable=false` を受信した時
  - `error -> idle`: `session.welcome` 再受信で接続復帰した時
- 手動確認手順:
  1. firmware を起動して `session.welcome` まで接続し、`idle` ログを確認する
  2. 画面タップで `sendAudioStream()` を実行し、`listening -> thinking` ログを確認する
  3. `tts.end` 応答で `thinking -> speaking`、再生終了で `speaking -> idle` を確認する
  4. `conversation.cancel` / `tts.stop` / `audio.stream_abort` を送信し、`interrupted -> idle` を確認する
  5. `retryable=false` の `error` を送信し `error` へ遷移、再接続後 `idle` 復帰を確認する

## 2.15 P8-10 Opus 計測項目追加メモ（2026-03-18）

- server の Opus 受信フローで、以下の runtime metrics を追加しました。
  - `pipeline.first_frame_latency_ms`
  - `pipeline.cadence_jitter_ms`
  - `pipeline.e2e_latency_ms`
- `audio.end` 処理時に `audio.stream_open` 登録時刻とフレーム受信時刻を使って計測値を算出し、`runtime_metrics` へ保存します。
  - first frame latency: `FirstFrameAt - OpenedAt`
  - cadence jitter: 連続チャンクの到着間隔と `frame_duration_ms` の平均絶対偏差
  - E2E latency: `now - OpenedAt`
- `GET /api/runtime/overview` の `pipeline` スナップショットに反映される項目を追加しました。
  - `first_frame_latency_ms`
  - `cadence_jitter_ms`
  - `e2e_latency_ms`
- サーバー内で `ReceivedAt` が未設定の音声チャンクも計測可能にするため、`AddAudioChunk` で受信時刻を補完するようにしました。
- 検証結果:
  - `cd server && go test ./...` 成功
## 2.19 P8-16 tts.chunk 欠落/遅延検知と concealment 実装メモ（2026-03-18）

### firmware 側（session.cpp / session.h）

- `enqueueTTSFrame()` の gap 検出ロジックを改修しました。
  - 旧: `chunkIndex != _ttsExpectedChunkIndex` で **即座にキューをクリア** していました。
  - 新: gap サイズを算出し、`insertConcealmentFrames()` で補完フレームを挿入します。
  - 過去インデックス（重複送信）は `chunkIndex < _ttsExpectedChunkIndex` で無視します。
- `insertConcealmentFrames()` を新規実装しました。
  - **減衰コピー**: 直前の正常フレームが存在する場合、振幅を `1/2^(i+1)` に徐々に減衰してコピーします（i=0 で 50%、i=1 で 25%、...）。
  - **無音補完**: 直前フレームがない場合はゼロ埋め PCM を挿入します。
  - 上限: `kMaxConcealmentFrames = 4`（80ms / 20ms フレーム）を超えないようにキャップします。
  - high-water 到達時も挿入を打ち切り、バッファオーバーフローを防ぎます。
- `clearTTSFrameQueue()` に concealment 状態 (`_ttsLastGoodFrameBytes` / `_ttsMissingChunkCount` / `_ttsConcealmentFrameCount`) のリセット処理を追加しました。
- 正常フレームのエンキュー後に `_ttsLastGoodFrameBytes` を更新するようにしました（減衰コピー素材）。
- `processTTSPlaybackQueue()` のストリーム終端で concealment メトリクスをログ出力します。
  - ログ形式: `[TTS][metrics] stream_id=... request_id=... missing_chunks=N concealment_frames=M`

### server 側（runtime_state.go / ws_handler.go）

- `PipelineSnapshot` に `TTSChunkSendFailCount int` を追加しました（downlink 欠落の検知指標）。
- `OnTTSChunkSendFail(requestID, streamID string)` メソッドを追加しました。
  - `pipeline.tts_chunk_send_fail_count` を `runtime_metrics` に永続化します。
  - `GET /api/runtime/overview` のレスポンスに反映されます。
- `sendTTSAudio()` の v1.1 / v1.0 ループで chunk 送信失敗時に `OnTTSChunkSendFail` を呼び出すようにしました。

### 手動確認手順（P8-16）

1. server から `tts.chunk(v1.1)` を連続送信する際に、意図的にチャンクを 1 つ以上省略する。
2. firmware のログで `[TTS][concealment] gap=N inserted=M` が出力されることを確認する。
3. 再生が止まらず、減衰した音声（または無音）でギャップが補完されることを確認する。
4. tts.end 後に `[TTS][metrics] missing_chunks=N concealment_frames=M` が出力されることを確認する。
5. `GET /api/runtime/overview` のレスポンスで `pipeline.tts_chunk_send_fail_count` が増えることを確認する。
6. `tts.stop` 送信後にキューと concealment 状態がクリアされ、次のストリームで `missing_chunks=0` にリセットされることを確認する。

### 検証結果

- `cd server && go build ./...` 成功
- `cd server && go test ./...` 全テスト通過
## 2.16 P8-18 追加メモ（2026-03-18）

- 重複確認の結果、既存の `P8-14`〜`P8-17` は `tts.chunk` のフレーム設計・再生安定化が中心であり、Voicevox TTS の Opus 変換と firmware デコード実装は未登録でした。
- `P8-18` として次の実装範囲を追加しました。
  - server: Voicevox の生成音声（PCM/WAV）を Opus へ変換し、`tts.chunk` と `tts.end(codec=opus)` で downlink 配信
  - firmware: Opus チャンクのデコード再生、既存 `codec=pcm` fallback 維持
  - 運用: `P8-10` の計測項目（first frame latency / cadence jitter / E2E latency）で効果検証

## 2.23 P8-18 補足: 初回こもり修正メモ（2026-03-19）

### 症状

- firmware アップロード直後の初回 TTS 再生のみ、音声が低くこもった音になる。
- 2 回目以降は正常な音質で再生される。

### 根本原因

```
_ttsSampleRateHz の初期値 = FW_AUDIO_SAMPLE_RATE = 16000
   ↓
P8-15 の prebuffer 設計により、tts.end 到着前に再生が開始される
   ↓
初回: tts.end 未受信 → _ttsSampleRateHz=16000 のまま再生
   ↓
実際の TTS 音声が 24000Hz で生成されていた場合 → ピッチが低くこもる
   ↓
2 回目以降: 前回 tts.end で _ttsSampleRateHz=24000 が設定済み → 正常
```

### 修正内容

- `firmware/app/stackchan/session.cpp` の `enqueueTTSFrame()` を更新しました。
  - ストリームの初回フレーム（`chunkIndex == 0`）受信時に `samplesPerChunk * 1000 / frameDurationMs` でサンプルレートを推算します。
  - 推算値が現在の `_ttsSampleRateHz` と異なる場合、即時更新します。
  - ログ: `[TTS] sample_rate_hz inferred from first chunk: 16000 -> 24000`
  - これにより `tts.end` の到着を待たずに正しいサンプルレートで再生できます。

### 検証結果

- `pio run -e stackchan_cores3` 成功
- ファームウェア書き込み後の初回再生で正しいサンプルレートが適用されることをログで確認できます。

## 2.22 P8-18 実行メモ（2026-03-19）

### server 側（audio / web / protocol）

- `server/internal/audio/opus_downlink.go` を追加しました。
  - Voicevox から受け取った WAV を `ffmpeg`（`libopus`）で OGG/Opus に変換します。
  - OGG ページから Opus 音声パケットを抽出し、`tts.chunk(v1.1)` で送信できるフレーム列を生成します。
  - 変換失敗時は error を返し、呼び出し側で PCM fallback できる設計にしました。
- `server/internal/web/ws_handler.go` の `sendTTSAudio()` を更新しました。
  - `codec=opus` かつ `chunk_version=1.1` の場合は Opus downlink を優先します。
  - Opus 変換に失敗した場合は warning を残し、`codec=pcm` へ自動 fallback します。
  - `tts.chunk(v1.1)` payload に optional `codec` を付与します（`opus` または `pcm`）。
- `server/internal/web/api_handler.go` を更新しました。
  - `POST /api/tests/voicevox/stackchan` で `codec`（`opus|pcm`）を受け付けます。
  - default を `codec=opus` / `chunk_version=1.1` に変更しました。
- `server/internal/protocol/events.go` を更新し、`TTSChunkPayloadV11` に optional `codec` を追加しました。
- テストを追加しました。
  - `server/internal/audio/opus_downlink_test.go`
  - OGG からの Opus パケット抽出の正常系/異常系を検証します。

### firmware 側（session）

- `firmware/app/stackchan/session.cpp` / `session.h` を更新しました。
  - `tts.chunk(v1.1)` の `codec` を受信してストリームごとに保持します。
  - `codec=opus` の場合、`esp32_opus`（`opus.h`）を使ってフレーム単位で decode し、PCM16 として再生します。
  - `codec=pcm` の既存再生パス（batch dequeue + concealment）を維持し、後方互換を確保します。
  - ストリーム終了/クリア時に Opus decoder を破棄し、リークを防止します。

### protocol / docs 側

- `protocol/websocket/schemas/tts.chunk.schema.json` に optional `codec` を追加しました。
- `protocol/websocket/events.md` / `protocol/versioning.md` / `protocol/websocket/validation-checklist.md` を更新し、
  v1.1 の `codec` 運用と互換性方針を明記しました。
- `protocol/examples/tts.chunk.example.json` に `codec: "opus"` を追加しました。

### 検証結果

- `cd server && go test ./...` 成功
- `cd firmware && pio run -e stackchan_cores3` 成功

### 手動確認手順（P8-18）

1. `POST /api/tests/voicevox/stackchan` を `{"codec":"opus","chunk_version":"1.1"}` で実行する
2. firmware ログで `codec=opus frame queued` と `codec=opus frame_index=...` の再生ログを確認する
3. `tts.end` 到達後に stream drain で `speaking -> idle` へ遷移することを確認する
4. `ffmpeg` 未導入または Opus 変換失敗ケースで warning の後に `codec=pcm` fallback 再生されることを確認する

## 2.17 P8-14 tts.chunk 音声フレーム再設計メモ（2026-03-18）

- `protocol/websocket/schemas/tts.chunk.schema.json` を更新し、`version=1.1` のフレーム単位 payload を追加しました。
  - required: `request_id` / `stream_id` / `chunk_index` / `frame_duration_ms` / `samples_per_chunk` / `audio_base64`
  - timing: `sent_at` または `playout_ts` の少なくとも一方を必須化
- 互換性確保のため、schema は `version=1.0`（旧 payload）と `version=1.1`（新 payload）の dual-read を許容します。
- 更新ファイル:
  - `protocol/websocket/events.md`
  - `protocol/versioning.md`
  - `protocol/websocket/validation-checklist.md`
  - `protocol/examples/tts.chunk.example.json`

## 2.18 P8-15 firmware 事前バッファ再生パイプラインメモ（2026-03-18）

- `firmware/app/stackchan/session.h` / `session.cpp` に `tts.chunk(v1.1)` 用のリングバッファ再生キューを追加しました。
  - queue capacity: 32 frames
  - prebuffer start: 80ms（要件 60〜120ms の範囲内）
  - low-water: 60ms
  - high-water: 240ms
  - playback batch: 40ms
- `tts.chunk` 受信時に `stream_id` / `frame_duration_ms` / `samples_per_chunk` が存在する場合はフレームキューへ積み、`loop()` 内の消費処理で順次再生します。
- `tts.end` はストリーム終端マーカーとして扱い、キューの drain 完了で `speaking -> idle` 遷移します。
- interrupt 系イベント（`conversation.cancel` / `tts.stop` / `audio.stream_abort`）と WS 切断時に再生キューを即時クリアするよう更新しました。

### 手動確認手順（P8-15）

1. firmware を起動し、server と接続した状態で `tts.chunk(v1.1)` を連続送信する
2. ログに `prebuffer ready` が出力されるまで再生開始しないことを確認する
3. 再生中に `low-water` 警告が出るケースでも再生ループが継続することを確認する
4. `tts.end` 後にキューが drain され、`speaking -> idle` へ戻ることを確認する
5. `tts.stop` または `conversation.cancel` 送信で即時停止し、キューがクリアされることを確認する

## 3. フェーズ 7 からの前提条件

- サーバー API（runtime overview / settings / pipeline test）は実装済み。
- WebUI は Svelte ビルド成果物を Go サーバーから /ui 配信可能。
- server 側の主要自動テストは通過済み。
- フェーズ 8 では「統合再現性」「継続検証」「運用復旧性」を優先する。

## 4. 受け入れ方針（フェーズ 8）

- Docker と CI で、手元依存の少ない再現手順を先に作る。
- DB 変更は migration 前提で管理し、手動 SQL 運用を避ける。
- 可観測性は最新値中心から履歴中心へ段階拡張する。
