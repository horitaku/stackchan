# Stackchan 再構築 実装計画

## 1. 目的

このドキュメントは、Stackchan 再構築プロジェクトにおける実装方針、段階的ロードマップ、計画と実績の管理を目的とします。

本プロジェクトは、firmware、server、protocol、providers、WebUI で構成されるマルチランタイム基盤です。設計の乖離や部分最適化を避けるため、プロトコルファーストの原則を維持しつつ、薄い縦スライスで開発を進めます。

## 2. プロジェクト原則

1. Protocol First
   server や firmware の実装前に、WebSocket 契約を定義します。
2. Thin Vertical Slices
   レイヤーを分断して個別実装するのではなく、最小のエンドツーエンドシナリオを 1 つずつ提供します。
3. Interface-First Provider Design
   STT、LLM、TTS は差し替え可能なインターフェースの背後に配置します。
4. Observability from the Start
   最初の実装フェーズから、構造化ログ、相関 ID、レイテンシ計測を導入します。
5. Secure Local Configuration
   秘密情報とローカル設定は Git 管理対象から除外し、共有可能な値はテンプレート例のみで管理します。
6. Expand by Addition
   将来の firmware やツールを壊さないよう、プロトコルとランタイム挙動は追加的進化を優先します。

## 3. 完了の定義

各フェーズまたは各イテレーションは、次の条件をすべて満たしたときのみ完了と見なします。

- スコープと期待挙動が文書化されていること。
- 関連するプロトコル、API、またはインターフェース契約が記述されていること。
- 実装に最低限の自動検証が含まれていること。
- トラブルシューティング用の構造化ログまたは診断情報が利用可能であること。
- 既知の制約と次アクションが記録されていること。

## 4. 推奨デリバリー順序

| フェーズ | テーマ | 主な成果 | 終了条件 |
| --- | --- | --- | --- |
| 0 | リポジトリ基盤 | トップレベル構成、ローカル用テンプレート、基本ワークフローファイル | ディレクトリ構成とプロジェクトテンプレートが準備できている |
| 1 | プロトコル v0 | 初期 WebSocket イベント定義とサンプル | 主要イベントとスキーマが文書化されている |
| 2 | サーバースケルトン | Gin サーバー、WebSocket エントリーポイント、セッションライフサイクル | hello/welcome フローがエンドツーエンドで動作する |
| 3 | Provider 境界 | STT、LLM、TTS インターフェースとモック実装 | 中核オーケストレーションがベンダー依存から分離されている |
| 4 | 最小音声パス | audio chunk 受信と制御可能な応答パイプライン | テスト入力で音声ライフサイクルが成立する |
| 5 | Firmware 接続性 | デバイス接続、再接続、状態報告 | M5Stack が接続とセッション復帰を行える |
| 6 | 再生とアバター同期 | TTS 再生連携と簡易リップシンク | デバイスが発話し、状態反映できる |
| 7 | WebUI と可観測性 | 設定 UI、診断、ランタイム可視化 | 運用者がシステムの確認とテストを実施できる |
| 8 | 統合と運用 | Docker、マイグレーション、CI、障害復旧強化 | 開発運用と実行運用が再現可能である |
| 9 | LLM 実装と会話コンテキスト統合 | OpenAI Chat Completions、Persona 設定、会話履歴管理、LLM テスト導線 | OpenAI LLM が応答し、UI から Persona 設定可能である |
| 10 | 識別とメモリー運用基盤 | 複数デバイス識別、長期記憶キー設計、記憶ガバナンス、評価導線 | セッション外記憶が安全かつ再現可能に運用できる |
| 11 | Firmware ハードウェア制御と診断導線 | デバイス抽象化、制御イベント、Hardware Test UI、状態レポート | WebUI から安全にハードウェア診断・校正・疎通確認を実施できる |
| 12 | Firmware 内部リファクタリング | StackchanSession 分解、実装ファイル整理、内部境界の明確化 | session.cpp の肥大化を抑え、今後の hardware 拡張を安全に継続できる |
| 13 | WebUI 内部リファクタリング | App.svelte 分割、UI コンポーネント境界整理、API/状態管理の責務分離 | 外部挙動を維持しながら WebUI の変更容易性と検証容易性を向上できる |

## 5. フェーズ詳細

### フェーズ 0. リポジトリ基盤

- 推奨トップレベル構成を作成します: firmware、server、protocol、providers、infra、docs、tools、examples。
- ignore ルールとローカル設定テンプレートを整備します。
- 共有サンプルとローカル専用ファイルの配置方針を決定します。
- 実行タスクは docs/project/phase00-tasklist.md で管理します。

### フェーズ 1. プロトコル v0

- メッセージエンベロープの項目を定義します: type、timestamp、session_id、sequence、version。
- 次の最小イベントセットから開始します:
  - session.hello
  - session.welcome
  - error
  - audio.chunk
  - audio.end
- JSON Schema とペイロード例を追加します。
- 将来拡張に向けた互換性ルールを記録します。
- 実行タスクは docs/project/phase01-tasklist.md で管理します。

### フェーズ 2. サーバースケルトン

- Go モジュールとエントリーポイントを作成します。
- Gin ベースの HTTP と WebSocket ブートストラップを追加します。
- セッション生成と hello/welcome 振る舞いを実装します。
- 構造化ログとリクエスト相関を導入します。
- 実行タスクは docs/project/phase02-tasklist.md で管理します。

### フェーズ 3. Provider 境界

- STT、LLM、TTS のインターフェースを定義します。
- まずモック Provider を追加します。
- 境界レイヤーでタイムアウトとリトライ挙動を明示します。
- 実行タスクは docs/project/phase03-tasklist.md で管理します。

### フェーズ 4. 最小音声パス

- WebSocket 経由で audio chunk を受信します。
- シーケンス整合性とライフサイクル完了を検証します。
- 将来の STT 入力に接続できるよう、入力の変換またはバッファリングを行います。
- キュー滞留と応答時間を計測します。
- 実行タスクは docs/project/phase04-tasklist.md で管理します。

### フェーズ 5. Firmware 接続性

- Wi-Fi 接続と WebSocket 再接続戦略を実装します。
- session hello とデバイス状態を送信します。
- firmware の責務はデバイス I/O とプロトコル処理に限定します。
- 実行タスクは docs/project/phase05-tasklist.md で管理します。

### フェーズ 6. 再生とアバター同期

- TTS 出力と再生制御をデバイスへ送信します。
- 簡易リップシンク抽象を導入します。
- 最小限の表情状態とモーション状態の処理を追加します。
- 実行タスクは docs/project/phase06-tasklist.md で管理します。

### フェーズ 7. WebUI と可観測性

- 設定 API とテスト実行 API を追加します。
- 診断と設定のための小規模 WebUI を構築します。
- ランタイムメトリクス、キュー状態、接続健全性を可視化します。
- 実行タスクは docs/project/phase07-tasklist.md で管理します。

### フェーズ 8. 統合と運用

- docker-compose ベースのローカル実行環境を追加します。
- PostgreSQL マイグレーション管理を導入します。
- CI チェックと統合テスト実行を追加します。
- リトライ、タイムアウト、縮退モード挙動を強化します。
- 実行タスクは docs/project/phase08-tasklist.md で管理します。

### フェーズ 9. LLM 実装と会話コンテキスト統合

- OpenAI Chat Completions API による実 LLM 統合を実装します。
- Session 内の会話履歴（utterances）を context window として管理します。
- UI から Persona / System Prompt を動的に変更可能にします。
- LLM レイテンシとトークン使用量を計測・可視化します。
- WebUI から LLM 単体テストと Stackchan 連携テストを実行可能にします。
- LLM 関連の障害復旧ランブックを追加します。
- 実行タスクは docs/project/phase09-tasklist.md で管理します。

### フェーズ 10. 識別とメモリー運用基盤

- 複数 Stackchan 同時接続を前提に、識別キーの責務分離を明確化します（session_id / device_id / request_id）。
- 5 種類の記憶タイプ（Session / Episodic / Semantic / Profile / Reflection）を段階的に構築します。
- Memory Orchestrator を中核に、ContextBundle 組み立て・PostProcess・セッション要約フローを実装します。
- 記憶取得は keywordスコアリング（将来は vector 検索）を基本にし、importance / confidence / recency で重み付けします。
- memory_facts / memories / profiles の DB スキーマを追加し、sessions に device_id を永続化します。
- 記憶ガバナンス（TTL/削除 API/監査ログ）と WebUI 運用導線を整備します。
- 実行タスクは docs/project/phase10-tasklist.md で管理します。

### フェーズ 11. Firmware ハードウェア制御と診断導線

- firmware 内のハードウェア責務を runtime / app 配下のサービスへ分離し、StackchanSession はイベント受信と委譲に集中させます。
- WebSocket protocol に device 制御イベントを追加し、LED、耳 NeoPixel、サーボ、音声テスト、マイク計測、カメラ取得、状態報告を契約化します。
- WebUI は本番設定画面として肥大化させる前に、Hardware Test を中心とした診断コンソールとして拡張します。
- server には WebUI から接続中 Stackchan へ制御を中継するテスト API を追加し、既存の UI -> server -> Stackchan の流れを再利用します。
- サーボ制御は raw angle 直指定より先に logical angle + calibration + safety limit の二層モデルを導入し、個体差調整と破損防止を優先します。
- low-level の device.servo.move と high-level の motion.play を分離し、診断用制御と演出用動作を混線させません。
- 実行タスクは docs/project/phase11-tasklist.md で管理します。

### フェーズ 12. Firmware 内部リファクタリング

- firmware の外部挙動を変えずに、肥大化した StackchanSession の内部責務を整理します。
- 最初の段階では public API を維持したまま実装ファイルを分割し、connection / protocol / avatar / tts stream などの関心事を見通しよくします。
- 次の段階で、TTS ストリーム処理、Avatar 表示、イベントディスパッチなど独立性の高い塊を補助クラスまたは内部モジュールへ切り出します。
- メモリ所有権、会話状態遷移、watermark 送信、Opus decode のような壊れやすい部分は、機能追加前に責務境界を固定します。
- 本フェーズは Phase 11 の大型拡張を安全に継続するための下準備として扱い、機能追加よりも変更容易性と検証容易性を優先します。
- 実行タスクは docs/project/phase12-tasklist.md で管理します。

### フェーズ 13. WebUI 内部リファクタリング

- 外部仕様を維持したまま、肥大化した App.svelte の責務を整理します。
- 最初の段階では表示責務をパネル単位コンポーネントへ分割し、App.svelte はページ組み立て中心に縮小します。
- 次の段階で API 呼び出しを共通層へ集約し、エラーハンドリングとレスポンス整形の一貫性を高めます。
- 状態管理は runtime/settings/tests/hardware のドメイン単位で整理し、回帰確認しやすい構造へ改善します。
- 本フェーズは機能追加よりも、差分衝突の低減、レビュー性向上、後続機能追加の安全性確保を優先します。
- 実行タスクは docs/project/phase13-tasklist.md で管理します。

## 6. PDCA 運用モデル

短いイテレーションで進めます。1 つのイテレーションは、必ず 1 つの薄い縦スライスのみを対象とします。

### Plan

- ユーザーに見える機能、またはシステムに見える機能を 1 つ選定します。
- スコープ、前提、インターフェース、リスクを定義します。
- 計測可能な成功条件を設定します。

### Do

- イテレーション目標を満たす最小実装を行います。
- 目標達成を阻害しない限り、隣接スコープへ拡張しません。

### Check

- 関連する自動テストを実行します。
- プロトコル互換性とログを検証します。
- 必要に応じて、レイテンシ、再接続挙動、キュー状態を測定します。

### Act

- うまくいった点、乖離した点、改善すべき点を記録します。
- 実際の結果に基づいて次イテレーション計画を更新します。

## 7. イテレーションの実施周期

- 推奨周期: 1 イテレーション 1 週間。
- 推奨スコープ: 1 イテレーションあたり 1 つのエンドツーエンド機能。
- 推奨レビュー観点:
  - 契約レビュー
  - 実装レビュー
  - 検証レビュー
  - 振り返りと次スライス選定

## 8. 初期イテレーション提案

### イテレーション 1: セッションハンドシェイク

- 目標: クライアントとサーバー間で信頼性の高い hello/welcome フローを確立する。
- スコープ:
  - session.hello と session.welcome のプロトコル定義
  - サーバー側セッションエントリーポイント
  - 最小クライアントまたはテストハーネス接続
  - セッションライフサイクルの構造化ログ
- 成功条件:
  - クライアントが接続し、welcome 応答を受信できる
  - session_id が一貫してログ記録される
  - このフローが最低 1 つの自動テストでカバーされる

### イテレーション 2: 音声受け入れ

- 目標: audio chunk を受け入れ、基本的な音声ストリームライフサイクルを完了する。
- スコープ:
  - audio.chunk と audio.end の処理
  - sequence 検証
  - 将来の STT 連携に向けたバッファリングまたは変換境界
- 成功条件:
  - 音声フレームを順序通り受信できる
  - ストリーム終端を正しく認識できる
  - 時間計測とエラーケースがログで観測できる

## 9. 計画対実績トラッカー

この表は各イテレーションの開始時と終了時に更新します。

| イテレーション | 計画スコープ | 計画終了条件 | 実績結果 | ギャップ | 次アクション | ステータス |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | セッションハンドシェイク | hello/welcome がエンドツーエンドで動作 | Go + Gin の WebSocket サーバーで hello/welcome を実装し、自動テストで検証完了 | heartbeat_interval_ms は未確定 | Provider 境界を定義する | 完了 |
| 2 | 音声受け入れ | audio chunk のライフサイクルを処理 | audio.chunk 蓄積、audio.end トリガー、STT -> LLM -> TTS の最小オーケストレーションを mock provider で実装し自動テスト通過 | WebSocket binary 転送と詳細メトリクスは未実装 | フェーズ 4 で音声品質とレイテンシ計測を強化する | 完了 |

## 10. 作業バックログ

| ID | 作業項目 | 優先度 | 担当 | 依存関係 | メモ | ステータス |
| --- | --- | --- | --- | --- | --- | --- |
| P0-1 | トップレベルのリポジトリ構成を作成 | 高 | Copilot | - | 最小フォルダー構成を作成済み | 完了 |
| P1-1 | プロトコルエンベロープを定義 | 高 | Copilot | P0-1 | バージョニング規則を含めて定義済み | 完了 |
| P1-2 | hello/welcome イベントを定義 | 高 | Copilot | P1-1 | 例とスキーマを追加済み | 完了 |
| P2-1 | Go サーバーをブートストラップ | 高 | Copilot | P1-2 | Gin + WebSocket エントリーポイントを作成済み | 完了 |
| P2-2 | セッション管理を追加 | 高 | Copilot | P2-1 | 相関 ID 付きで実装済み | 完了 |
| P3-1 | Provider インターフェースを定義 | 高 | Copilot | P2-2 | STT、LLM、TTS を interface-first で実装済み | 完了 |
| P3-2 | Provider 呼び出しポリシーを導入 | 高 | Copilot | P3-1 | timeout / retry / cancel / error mapping を実装済み | 完了 |
| P4-1 | audio バイナリ転送を実装 | 高 | Copilot | P3-2 | WebSocket binary と Opus フレーム受け渡し | 完了 |
| P8-08 | interrupt 系イベントを protocol へ正式追加 | 高 | Copilot | P4-1 | `conversation.cancel` / `tts.stop` / `audio.stream_abort` を schema-first で追加 | 完了 |
| P8-09 | firmware に最小 conversation 状態遷移を実装 | 高 | Copilot | P8-08 | `idle/listening/thinking/speaking/interrupted/error` の遷移を最小導入 | 完了 |
| P8-10 | Opus 経路の計測項目を runtime metrics へ追加 | 高 | Copilot | P8-05, P8-04 | first frame / cadence jitter / E2E latency を収集して可視化へ接続 | 完了 |
| P8-11 | Docker compose に Voicevox を追加し TTS 環境を前倒し整備 | 高 | Copilot | P8-01 | `voicevox` サービス追加、`VOICEVOX_BASE_URL` 接続確認、起動/復旧手順を整備 | 完了 |
| P8-12 | WebUI から Voicevox を使った UI 単体テスト導線を追加 | 高 | Copilot | P8-11 | テキスト入力 -> 音声生成 -> UI 内確認の最小テスト導線を追加 | 完了 |
| P8-13 | WebUI から Voicevox を使った Stackchan 連携テスト導線を追加 | 高 | Copilot | P8-12 | Stackchan 連携時の再生結果と遅延を確認できるテスト導線を追加 | 完了 |
| P8-14 | tts.chunk を音声フレーム単位へ再設計 | 中 | Copilot | P8-13 | `stream_id` / `frame_duration_ms` / `samples_per_chunk` / `playout_ts` を含む chunk 契約へ更新 | 完了 |
| P8-15 | firmware に事前バッファ付き再生パイプラインを導入 | 中 | Copilot | P8-14 | 60〜120ms 事前バッファ、low-water/high-water、リングバッファ消費を導入 | 完了 |
| P8-16 | tts.chunk の欠落/遅延検知と concealment 方針を導入 | 中 | Copilot | P8-15 | sequence/timestamp 管理と欠落時の補完方針を追加 | 完了 |
| P8-17 | 音声再生処理を専用消費ループへ分離 | 中 | Copilot | P8-15 | 通信受信と再生処理を分離し、将来の低遅延化に備える | 完了 |
| P10-01 | 識別キー方針を protocol と server へ明文化 | 高 | Copilot | P9-06 | session_id（接続）、device_id（個体）、request_id（ターン）の責務を固定し、衝突時ルールを定義 | 未着手 |
| P10-02 | sessions に device_id / user_id を追加 | 高 | Copilot | P10-01 | migration 追加、handshake 更新、session 再接続時に同一デバイス追跡可能化 | 未着手 |
| P10-03 | Profile Memory スキーマと CRUD API | 高 | Copilot | P10-02 | profiles テーブル、GET/PUT /api/memory/profile、WebUI 設定画面 | 未着手 |
| P10-04 | Semantic Memory（memory_facts）スキーマと初期抽出 | 高 | Copilot | P10-02 | memory_facts テーブル、FactRepository、ルールベース Extractor | 未着手 |
| P10-05 | Memory Orchestrator の骨組み実装 | 高 | Copilot | P10-03, P10-04 | orchestrator.go、ContextBundle、Prompt Builder の骨組み | 未着手 |
| P10-06 | セッション要約機能 | 中 | Copilot | P10-05 | Summarizer、sessions.last_summary 更新、20 メッセージ／トークン閾値トリガー | 未着手 |
| P10-07 | Episodic Memory スキーマと保存 | 中 | Copilot | P10-05 | memories テーブル（type=episode）、PostProcess 連携 | 未着手 |
| P10-08 | 記憶 Retriever のスコアリング実装 | 中 | Copilot | P10-07 | retriever.go、scorer.go、importance/confidence/recency の重み計算 | 未着手 |
| P10-09 | 同時接続制御と認可検証 | 高 | Copilot | P10-01 | 同一 device_id 接続ポリシー（reject/kick）、エラーイベント設計 | 未着手 |
| P10-10 | Reflection Memory と LLM 要約連携 | 低 | Copilot | P10-06 | Summarizer の LLM 連携、memories type=reflection 保存 | 未着手 |
| P10-11 | 記憶ガバナンス実装 | 高 | Copilot | P10-04 | TTL 設定、削除 API、論理削除、監査ログ | 未着手 |
| P10-12 | WebUI 記憶管理導線 | 中 | Copilot | P10-11 | 記憶参照・削除 UI、facts 編集 UI、テスト API | 未着手 |
| P10-13 | 記憶品質評価と回帰テスト | 中 | Copilot | P10-08 | key fact 再現テスト、誤想起テスト、CI 組込 | 未着手 |
| P10-14 | ランブックと監査導線の整備 | 中 | Copilot | P10-11, P10-09 | memory 運用 runbook、障害切り分け手順、アクセスログ確認手順 | 未着手 |
| P11-01 | firmware のハードウェア責務をサービスへ分離 | 高 | Copilot | P8-17 | servo / lighting / touch / camera の責務を StackchanSession から外し、委譲中心へ整理 | 未着手 |
| P11-02 | device 制御イベントを protocol v0 に追加 | 高 | Copilot | P11-01 | device.led.set / device.ears.set / device.servo.move / device.state.report などを schema-first で定義 | 未着手 |
| P11-03 | server に hardware test API を追加 | 高 | Copilot | P11-02 | WebUI から接続中セッションへ制御を中継する API 群を実装 | 未着手 |
| P13-01 | App.svelte の責務マップと分割境界を定義 | 高 | Copilot | P12-11 | Overview/Settings/Tests/Hardware の境界、props/events 方針を固定 | 未着手 |
| P13-02 | WebUI パネルをコンポーネントへ分割 | 高 | Copilot | P13-01 | App.svelte をページオーケストレーション中心へ縮小 | 未着手 |
| P13-03 | API 呼び出し層を共通化 | 高 | Copilot | P13-02 | lib/api 配下に runtime/settings/tests/hardware API を整理 | 未着手 |
| P13-04 | 状態管理のドメイン分離 | 中 | Copilot | P13-03 | runtime/settings/tests/hardware 単位で状態を追跡しやすく整理 | 未着手 |
| P13-05 | WebUI 回帰確認手順と引き継ぎ docs を整備 | 高 | Copilot | P13-04 | 外部挙動維持の確認手順と既知制約を文書化 | 未着手 |
| P11-04 | WebUI Hardware Test 画面を追加 | 高 | Copilot | P11-03 | Servo / LED / Audio / Camera の即時診断導線を実装 | 未着手 |
| P11-05 | firmware 状態レポートと診断ログを強化 | 中 | Copilot | P11-03 | RSSI / heap / angle / calibration / mic level / speaker busy を可視化へ接続 | 未着手 |
| P12-01 | StackchanSession の責務マップを作成 | 高 | Copilot | P8-17 | connection / protocol / avatar / tts stream / audio uplink の依存を棚卸しし、分割順を固定 | 未着手 |
| P12-02 | session.cpp を実装ファイル単位で分割 | 高 | Copilot | P12-01 | public API を維持したまま session_connection / session_protocol / session_tts_stream などへ整理 | 未着手 |
| P12-03 | TTS ストリーム処理を内部モジュール化 | 高 | Copilot | P12-02 | frame queue / concealment / watermark / opus decode の責務を session 本体から縮退 | 未着手 |
| P12-04 | Avatar 表示と motion 演出を分離 | 中 | Copilot | P12-02 | 表情更新、lip sync、表示更新周期、最小 motion 演出を独立させる | 未着手 |
| P12-05 | 受信イベントルータを整理 | 中 | Copilot | P12-02 | onTextMessage の dispatch と handler 群を見通しの良い構造へ再編 | 未着手 |
| P12-06 | リファクタ後の回帰確認導線を追加 | 高 | Copilot | P12-03, P12-04, P12-05 | hello/welcome、heartbeat、tts playback、interrupt の回帰を検証可能にする | 未着手 |

## 11. 意思決定ログ

専用 ADR 構成を導入するまでは、プロジェクトレベルの意思決定をここに記録します。

| 日付 | 決定事項 | 理由 | 影響 |
| --- | --- | --- | --- |
| 2026-03-14 | Firmware のローカル設定はランタイム .env 読み込みに依存しない | M5Stack と PlatformIO のワークフローはビルド時設定中心であるため | 秘密情報とローカル設定はローカル設定ファイルまたは無視対象ヘッダーで管理する |
| 2026-03-14 | プロジェクト実行はプロトコルファーストの段階的デリバリーに従う | firmware と server 間の統合作業の手戻りを減らせるため | 契約定義が実装開始のゲート成果物になる |
| 2026-03-14 | フェーズ 0 の基盤整備を完了し、次フェーズへ移行する | ディレクトリ構成、テンプレート、運用ルール、初期導線が揃ったため | フェーズ 1 の protocol v0 定義に着手可能になった |
| 2026-03-14 | フェーズ 1 のプロトコル v0 定義を完了し、次フェーズへ移行する | イベント定義、スキーマ、サンプル、互換性ルール、検証観点が揃ったため | フェーズ 2 のサーバースケルトン実装に着手可能になった |
| 2026-03-14 | フェーズ 2 のサーバースケルトン実装を完了し、次フェーズへ移行する | hello/welcome フロー、エンベロープ検証、sequence 管理、構造化ログ、自動テストが揃ったため | フェーズ 3 の Provider 境界定義に着手可能になった |
| 2026-03-14 | フェーズ 3 の Provider 境界実装を完了し、次フェーズへ移行する | interface 定義、DI、mock provider、リトライ/タイムアウト、エラー変換、最小オーケストレーション、自動テストが揃ったため | フェーズ 4 の最小音声パス品質強化に着手可能になった |
| 2026-03-15 | フェーズ 6 の再生とアバター同期を完了し、次フェーズへ移行する | `tts.end` 再生、簡易リップシンク、表情/モーション受信、実機疎通、引き継ぎ事項整理が揃ったため | フェーズ 7 の WebUI 可視化と設定 API 実装に着手可能になった |
| 2026-03-15 | フェーズ 7 の WebUI と可観測性を完了し、次フェーズへ移行する | 可観測性 API、設定 API、Svelte WebUI、疎通テスト導線、アラート表示、確認手順、フェーズ 8 バックログ整理が揃ったため | フェーズ 8 の統合と運用強化（Docker/CI/DB migration）に着手可能になった |
| 2026-03-15 | 音声輸送の正規ルートを `binary + opus` とし、`pcm` は開発互換 fallback として扱う | 後工程での decode 境界、計測基準、イベント互換の手戻りを抑えるため | protocol と firmware/server 実装の判断基準が統一される |
| 2026-03-15 | docs の参照系を `project/` と `architecture/` の 2 層に整理する | 計画文書と設計基準の混在を防ぎ、仕様の真実点を明確化するため | 状態遷移やランタイム境界の仕様を architecture 側で固定できる |
| 2026-03-15 | provider 実行コードは当面 `server/internal/providers` に集約し、トップレベル `providers/` は契約管理に限定する | 実行主体が server に集中している現段階での責務分離を明確にするため | provider 実装置き場の二重管理リスクを回避できる |

## 12. 更新ルール

- フェーズ開始、完了、または重要な変更時にこのファイルを更新します。
- バックログは追加型で管理し、完了項目を結果記録なしに削除しません。
- 各イテレーション終了時に、計画対実績トラッカーへ実績を反映します。
- ハードウェア制約やレイテンシ結果により計画を変更した場合は、理由を明示的に記録します。

## 13. 参照優先順位

- 実行順序、タスク、進捗判定
  - `docs/project/`
- ランタイム境界、状態遷移、音声輸送の基準仕様
  - `docs/architecture/`
- 実装詳細や起動手順
  - 各ランタイム配下 README（`server/README.md`、`firmware/README.md` など）
