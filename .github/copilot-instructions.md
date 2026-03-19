# Stackchan Rebuild Project: Copilot Instructions

## 1. このプロジェクトの目的

このリポジトリは、Stack-chan をゼロベースで再構築するためのプロジェクトです。  
機能追加よりも先に、長期運用しやすい構成、明確な責務分離、段階的に拡張できる設計を重視してください。

この指示書は、GitHub Copilot がコード提案やリファクタリングを行う際の判断基準です。

## 2. 現時点の想定アーキテクチャ

### 2.1 デバイス側（Stackchan 本体）

- M5Stack CoreS3
- タッチスクリーン
- マイクロSDカード
- マイク
- スピーカー
- 内蔵カメラ
- M5Go Bottom3
- NECO MIMI（NeoPixel）
- サーボモーター（X軸, Y軸）

### 2.2 バックエンド側

- 起動方式: docker-compose
- メイン言語: Go
- Web フレームワーク: Gin
- データベース: PostgreSQL（docker-compose で同梱）
- AI 関連コンポーネント
	- LLM: OpenAI API
	- STT: OpenAI API
	- TTS: Voicevox

### 2.3 バックエンド WebUI

- WebUI は Svelte を採用し、ビルド成果物は静的ファイルとして配信する
- 配信は Go の Web サービスが担当し、API/WebSocket と同一プロセスで提供する
- WebUI の目的
  - 各種設定の変更（音声、接続先、デバイス動作、デバッグ設定）
  - 疎通確認と機能テスト（STT/LLM/TTS 単体テスト、音声入出力テスト）
  - ランタイム状態の可視化（接続状態、キュー滞留、遅延、エラー）

## 3. 通信方式の基本方針

### 3.1 デバイス - バックエンド間通信

- 通信方式は WebSocket を第一候補として設計する
- 低遅延と双方向性を優先し、以下を満たすこと
	- 音声ストリームの継続送受信
	- 制御イベント（サーボ、表情、LED、状態通知）の即時反映
	- 再接続とセッション復帰

### 3.2 メッセージ設計指針

- バイナリとテキストを用途で分離する
	- バイナリ: 音声フレーム（Opus）
	- JSON: 制御コマンド、メタデータ、状態通知
- すべてのメッセージに最小限の共通フィールドを持たせる
	- type
	- timestamp
	- session_id
	- sequence
- 後方互換性のため version フィールドを検討する

## 4. 音声パイプライン方針

### 4.1 品質重視の音声戦略

- 音声入出力は Opus エンコード/デコードを導入し、高品質と低帯域を両立する
- 目標は「自然な会話体験」であり、以下を継続的に最適化する
	- レイテンシ（録音開始から再生開始まで）
	- 音質（ノイズ、破綻、クリッピングの抑制）
	- 発話の途切れ/詰まりの抑制

### 4.2 推奨データフロー（初期案）

1. デバイスで収音
2. Opus でフレーム化して WebSocket 送信
3. バックエンドでデコードまたは STT 入力向け変換
4. STT（OpenAI API）
5. LLM（OpenAI API）
6. TTS（Voicevox）
7. デバイスへ音声データとリップシンク用情報を送信
8. デバイスで再生と口パク同期

## 5. アバター表示とリップシンク

- 画面描画は m5stack-avatar をベースに実装する
- 「話している感じ」を出すため、最低限以下を同期する
	- 音声再生状態
	- 口形状の変化（音量または簡易音素ベース）
	- まばたき、視線、表情の状態管理
- 将来拡張を見越し、リップシンク信号は抽象化レイヤで扱う

## 6. 参照実装の扱い

- 参考リポジトリ
	- stackchan-atama
	- m5stack-avatar
	- AI_StackChan_Ex
- 方針
	- 設計意図を学ぶために参照する
	- 既存コードをそのまま複製しない
	- このプロジェクトの責務分離と拡張性を優先する

### 6.1 AI_StackChan_Ex から取り入れる観点

- 低遅延化の観点として、将来は Realtime API 相当の音声入出力直結モードを検討する
- 機能を段階導入できるよう、ハードウェア依存機能（カメラ、顔検出、ウェイクワード等）は feature flag で制御する
- 運用時の切り分けを容易にするため、状態可視化（ステータスモニタ）を WebUI の標準機能として重視する
- ユーザー拡張をしやすくするため、将来のアプリ追加を見据えたモジュール境界を維持する

## 7. Go バックエンド実装ガイド

### 7.1 設計原則

- クリーンアーキテクチャを過度に厳密化しすぎず、責務分離を明確にする
- HTTP/API サーバー実装は Gin を前提とする
- I/O 境界を明示する
	- WebSocket ハンドラ
	- 音声処理（Opus）
	- STT/LLM/TTS クライアント
	- セッション/状態管理
- 依存注入を行い、外部 API をモック可能にする

### 7.2 エラーハンドリング

- ネットワーク断、API 制限、タイムアウトを前提に実装する
- リトライは指数バックオフを基本とし、無限リトライを避ける
- 利用者に影響する障害はイベントとしてデバイスへ通知する

### 7.3 ログと観測性

- 構造化ログを採用する（JSON 形式を推奨）
- 相関 ID（session_id, request_id）を全レイヤで引き回す
- 最低限の観測項目
	- STT/LLM/TTS レイテンシ
	- WebSocket 切断回数
	- 音声キュー滞留時間

## 8. プロジェクト構造（レイヤー＋ランタイム分離）

このプロジェクトは単一アプリではなく、複数ランタイムで構成されるプラットフォームです。  
一般的な `src/` 中心構成ではなく、ランタイム単位 + 契約単位で分離する。

- 推奨トップレベル構造

```txt
repo/
├ firmware/                # M5Stack / Stack-chan runtime
├ server/                  # Go + Gin AI server
├ protocol/                # WebSocket protocol contract
├ providers/               # STT / LLM / TTS adapter
├ tools/                   # 開発支援ツール
├ infra/                   # docker / deploy
├ docs/
├ examples/
└ .github/                 # Copilot / CI / automation
```

- 設計原則
  - ランタイム境界（firmware/server）と契約境界（protocol）を分離する
  - provider 実装を分離してベンダーロックを避ける
  - 拡張時に既存ランタイムを壊さない構造を優先する

### 8.1 firmware

```txt
firmware/
├ runtime/
│  ├ audio/
│  ├ display/
│  ├ motion/
│  ├ input/
│  └ network/
├ boards/
│  ├ core2/
│  └ cores3/
├ protocol/
├ app/
│  └ stackchan/
└ main.cpp
```

- 役割
  - デバイス制御（マイク、スピーカー、LCD、サーボ、タッチ）
  - WebSocket クライアント
- 重要ルール
  - firmware に AI オーケストレーションロジックを持ち込まない

#### 8.1.1 必要な環境変数一覧（firmware）

デバイス側（M5Stack / Stack-chan runtime）で利用する設定値を以下に定義する。ここでいう「環境変数」はサーバープロセスの `.env` を意味しない。firmware ではビルド時設定値として扱い、PlatformIO の `build_flags`、`platformio.ini` から参照するローカル設定ファイル、または git 管理外の `include/secrets.h` のようなヘッダで注入する。Wi-Fi や認証情報は機密情報として扱い、平文でのコミットを禁止する。

| 変数名 | 必須 | 用途 | 例 |
|--------|------|------|----|
| `FW_DEVICE_ID` | 必須 | デバイス識別子（接続時の識別） | `stackchan-cores3-01` |
| `FW_WS_URL` | 必須 | 接続先 WebSocket URL | `ws://192.168.1.10:8080/ws` |
| `FW_WS_TOKEN` | 任意 | WebSocket 認証トークン（採用時） | `replace-with-token` |
| `FW_WIFI_SSID` | 必須 | Wi-Fi SSID | `MyHomeWiFi` |
| `FW_WIFI_PASSWORD` | 必須 | Wi-Fi パスワード | `change-me` |
| `FW_AUDIO_SAMPLE_RATE` | 任意 | 収音/再生サンプルレート（Hz） | `16000` |
| `FW_AUDIO_FRAME_MS` | 任意 | 音声フレーム長（ms） | `20` |
| `FW_OPUS_BITRATE` | 任意 | Opus ビットレート（bps） | `24000` |
| `FW_LOG_LEVEL` | 任意 | firmware ログ出力レベル | `info` |
| `FW_RECONNECT_BASE_MS` | 任意 | 再接続待機の初期値（ms） | `500` |
| `FW_RECONNECT_MAX_MS` | 任意 | 再接続待機の最大値（ms） | `10000` |
| `FW_TIMEZONE` | 任意 | タイムゾーン設定 | `Asia/Tokyo` |

- 運用ルール
	- M5Stack firmware は `.env` を直接読み込む前提にしない
	- firmware 用の設定は `platformio.ini` からローカル専用ファイルを `extra_configs` で読み込む方式、または git 管理外の `include/secrets.h` / `include/secrets_local.h` で管理する
	- 共有用には `platformio.ini.example`、`include/secrets.example.h` などのテンプレートを配置し、実値ではなくダミー値のみ記載する
	- CI では PlatformIO のビルド引数やシークレット注入で同じ設定名を渡せる構成を優先する
	- `FW_WIFI_PASSWORD`、`FW_WS_TOKEN` はログ出力禁止（マスク必須）とする

### 8.2 server

```txt
server/
├ cmd/
│  └ stackchan-server/
├ internal/
│  ├ session/
│  ├ conversation/
│  ├ audio/
│  ├ providers/
│  ├ tools/
│  ├ memory/
│  ├ persona/
│  ├ protocol/
│  └ web/
└ pkg/
```

- 役割
  - AI orchestration（session / STT / LLM / TTS / tools / memory）
  - API/WebSocket 提供
  - Svelte の static build 配信

### 8.3 protocol（重要）

```txt
protocol/
├ websocket/
│  ├ events.md
│  └ schemas/
├ versioning.md
└ examples/
```

- WebSocket protocol を API 契約として先に定義する
- イベント例
  - session.hello
  - audio.chunk
  - audio.end
  - stt.partial
  - stt.final
  - llm.started
  - tts.chunk
  - tts.end
  - avatar.expression
  - motion.play
  - conversation.cancel

### 8.4 providers

```txt
providers/
├ stt/
│  ├ openai/
│  └ whisper/
├ llm/
│  ├ openai/
│  ├ gemini/
│  └ ollama/
└ tts/
   ├ openai/
   ├ voicevox/
   └ elevenlabs/
```

- メリット
  - provider の追加が容易
  - vendor lock の回避
  - モックと差し替えテストが容易

### 8.5 tools

```txt
tools/
├ protocol-validator/
├ session-replay/
├ log-viewer/
└ event-generator/
```

- 例
  - `stackchan replay session.json` のような会話再現ツールを提供する

### 8.6 infra

```txt
infra/
├ docker/
│  ├ Dockerfile          # マルチステージビルド定義
│  └ docker-compose.yml  # サービス起動オーケストレーション
├ dev/
├ production/
└ terraform/
```

- docker-compose、開発環境、デプロイ定義を配置する
- PostgreSQL は docker-compose で Go サービスと同梱して起動する
- DB 接続情報は環境変数で管理し、リポジトリにパスワード等を含めない
- マイグレーション管理ツール（例: golang-migrate）を採用し、スキーマ変更を追跡する

#### 8.6.1 必要な環境変数一覧（server）

起動に必要な設定値を以下に定義する。機密情報は必ず環境変数で与え、リポジトリへ平文で保存しない。

| 変数名 | 必須 | 用途 | 例 |
|--------|------|------|----|
| `APP_ENV` | 任意 | 実行環境（local / dev / production） | `local` |
| `SERVER_ADDR` | 任意 | Go サーバーの待受アドレス | `:8080` |
| `LOG_LEVEL` | 任意 | ログ出力レベル | `info` |
| `OPENAI_API_KEY` | 必須 | OpenAI（STT/LLM）呼び出し認証キー | `sk-...` |
| `OPENAI_MODEL_CHAT` | 任意 | 会話用 LLM モデル名 | `gpt-4o-mini` |
| `OPENAI_MODEL_STT` | 任意 | STT 用モデル名 | `gpt-4o-mini-transcribe` |
| `VOICEVOX_BASE_URL` | 必須 | Voicevox API エンドポイント | `http://voicevox:50021` |
| `DATABASE_URL` | 必須 | PostgreSQL 接続 URL | `postgres://user:pass@db:5432/stackchan?sslmode=disable` |
| `SESSION_SECRET` | 必須 | セッション署名・暗号化に使う秘密鍵 | `change-me-in-production` |
| `WS_READ_TIMEOUT` | 任意 | WebSocket 読み取りタイムアウト（秒） | `30` |
| `WS_WRITE_TIMEOUT` | 任意 | WebSocket 書き込みタイムアウト（秒） | `30` |
| `CORS_ALLOWED_ORIGINS` | 任意 | CORS 許可オリジン（カンマ区切り） | `http://localhost:5173` |

- 運用ルール
	- ローカル開発では `.env` を使用してよいが、`.env` は git 管理対象に含めない
	- 共有用には `.env.example` を配置し、ダミー値のみ記載する
	- 本番環境ではシークレットマネージャーまたは CI/CD の secret 機能から注入する

#### 8.6.2 DB設計指針（PostgreSQL）

- 永続化対象は `sessions`、`utterances`、`conversation_events`、`runtime_metrics`、`system_settings`、`memories`、`memory_facts`、`profiles` を基本とする
- 機密情報（APIキー、Wi-Fiパスワード、アクセストークン等）は DB に保存しない
- 全主要テーブルに `id`、`created_at`、`updated_at` を持たせ、削除方針は `deleted_at` の有無で明示する
- 相関追跡のため、会話系テーブルには `session_id` と `request_id` を保持する
- 会話イベントは `conversation_events(session_id, sequence)` で順序整合性を保証する
- 命名は `snake_case` で統一し、外部キー列は `xxx_id` の形式を採用する
- 制約は `NOT NULL`、`UNIQUE`、`CHECK`、外部キーを適切に設定し、アプリ側検証に依存しすぎない
- インデックスは実クエリ起点で設計し、導入時に `EXPLAIN ANALYZE` で性能を確認する
- スキーマ変更は `golang-migrate` で管理し、`up/down` の両方を必須とする
- 破壊的変更は「追加 -> データ移行 -> 旧列削除」の段階移行で実施する
- 保持期間と削除ポリシー（物理削除/論理削除）をテーブル種別ごとに定義する
- 音声データ本体は原則オブジェクトストレージで管理し、DB にはメタデータのみ保存する

#### Dockerfile マルチステージビルド構成

本番イメージは 3 ステージ構成の Dockerfile で一貫してビルドする。
docker-compose はこの Dockerfile を呼び出すオーケストレーターとして機能する。

| ステージ | ベースイメージ | 役割 |
|----------|---------------|------|
| Stage 1 (webui-builder) | `node:lts-alpine` | Svelte を `npm run build` し `dist/` を生成 |
| Stage 2 (server-builder) | `golang:alpine` | Go バイナリをビルドし、Stage 1 の `dist/` をコピー |
| Stage 3 (runtime) | `gcr.io/distroless/static` または `alpine` | Stage 2 のバイナリのみを含む最小実行イメージ |

- Stage 1 → Stage 2 は `COPY --from=webui-builder` で静的ファイルを受け渡す
- Go バイナリは `embed.FS` で静的ファイルを埋め込み、外部ファイル依存をなくす
- 最終イメージにはソースコード・ビルドキャッシュを含めない

### 8.7 docs / examples / .github

```txt
docs/
├ architecture/
├ decisions/
├ project/
└ protocol/

examples/
├ minimal-client/
├ simple-stackchan/
└ test-bot/
```

- `.github/` には Copilot 指示、CI、自動化ルールを配置する

### 8.8 WebUI 運用ルール

- 開発時は Svelte 開発サーバーと Go バックエンドを分離起動してよい
- 本番相当ビルドは Dockerfile マルチステージビルドで完結させる（8.6 参照）
  - `docker-compose build` 一発で Svelte ビルド → Go ビルド → 最小イメージ生成まで完了する
  - Go は `embed.FS` で `webui/dist` を埋め込んで配信する
- 設定値は API 経由で保存・更新し、UI から直接ファイル書き換えしない
- UI からテスト実行した結果（成功/失敗、レイテンシ、エラー）は構造化して保持する

### 8.9 開発順序（Protocol First）

破綻しにくい実装順序は次のとおり。

1. protocol 設計
2. server 実装
3. firmware 実装

## 9. テスト戦略

- 重点
	- WebSocket プロトコル整合性
	- 音声フレーム連結処理の健全性
	- 外部 API 異常時のフォールバック
- テストレベル
	- 単体テスト: 変換ロジック、メッセージ検証、状態遷移
	- 結合テスト: WebSocket と音声パイプライン
	- UI 結合テスト: WebUI から設定更新と疎通テストを実行できること
	- 手動確認: 実機でのリップシンク、発話遅延、モーター連動

## 10. Copilot への具体的な指示

コード提案時は、以下を優先してください。

1. まずは動く最小構成を実装し、その後に拡張しやすい形へ整理する
2. WebSocket メッセージは型安全とバージョニングを考慮して定義する
3. Opus を使う音声経路では、サンプルレート/チャネル数/フレーム長を明示する
4. OpenAI/Voicevox クライアントはインターフェース化し、モック可能にする
5. タイムアウト、キャンセル、再接続を必ず設計に含める
6. 提案コードには、処理の意図が分かる簡潔なコメントを付ける
7. 破壊的変更を提案する場合は、移行手順を必ず示す
8. WebUI は「Svelte static build を Go で配信」の構成を前提に提案する
9. WebUI の設定操作とテスト実行は API 化し、画面と処理責務を分離する
10. Dockerfile はマルチステージビルド（Stage1: Node.js/Svelte → Stage2: Go + dist コピー → Stage3: 最小ランタイム）で構成し、`embed.FS` で静的ファイルを埋め込む
11. 記憶機能は Memory Orchestrator を中核に、BuildContext → PostProcess の 2 フロー構成で実装する（§12 参照）
12. 識別子は session_id（接続単位）・device_id（個体単位）・request_id（ターン単位）の責務を厳守し、user_id = device_id の 1:1 運用から始める
13. 記憶保存判定はルールベース Extractor から始め、LLM 抽出は Phase 2 以降に導入する

## 11. 今後の拡張候補（メモ）

- VAD（Voice Activity Detection）導入
- ノイズ抑制とエコーキャンセル
- リップシンクを音素ベースへ高精度化
- カメラ入力を使った視線・表情インタラクション
- オフライン時の限定機能モード
- Realtime API 相当の低遅延会話モード
- ローカル推論モード（クラウド API 非依存）の検証
- 記憶検索を keyword → Qdrant（vector embedding）へ高精度化
- 家族スコープ（scope=family）による複数ユーザー記憶の分離
- 感情タグ・時間帯連動ペルソナ

## 12. 識別とメモリー基盤の設計方針（フェーズ 10〜）

### 12.1 識別子の責務

| 識別子 | ライフタイム | 主な用途 |
| --- | --- | --- |
| session_id | WebSocket 接続単位（再接続で再発行） | ライブ接続管理、短期記憶スコープ |
| device_id | firmware 個体（半永久） | 再接続追跡、長期記憶の紐づけ主軸 |
| request_id | 1 会話ターン | STT/LLM/TTS/metrics の相関突合 |
| user_id | 運用単位（現時点は device_id と 1:1） | 記憶スコープの論理単位 |

- 現時点では `user_id = device_id` で 1:1 運用から始める
- 家族複数運用は将来 user_id を独立させて対応する
- 受信識別子を無条件に信用しない（server-side で検証する）
- 同一 device_id の二重接続ポリシーを明示する（reject または旧接続切断）

### 12.2 記憶タイプ（5 種類）

| タイプ | 保存先 | ライフタイム |
| --- | --- | --- |
| Session Memory（短期） | インメモリ / utterances テーブルの直近 N 件 | セッション内のみ |
| Episodic Memory（出来事） | memories テーブル（type=episode） | 長期、TTL 設定可 |
| Semantic Memory（事実） | memory_facts テーブル（key-value） | 長期、upsert で更新 |
| Profile Memory（ペルソナ） | profiles テーブル | 半永久（明示変更のみ更新） |
| Reflection Memory（要約） | memories テーブル（type=reflection） | 長期、要約のたびに更新 |

### 12.3 Memory Orchestrator パターン

```go
// Orchestrator は会話の文脈構築と記憶更新の中核を担います。
type Orchestrator interface {
    // BuildContext は LLM 呼び出し前に記憶・プロファイル・会話履歴を集約します。
    BuildContext(ctx context.Context, userID, sessionID, input string) (*ContextBundle, error)
    // PostProcess は LLM 応答後に記憶候補を抽出・保存し、必要に応じてセッションを要約します。
    PostProcess(ctx context.Context, userID, sessionID, userInput, assistantOutput string) error
}
```

- `server/internal/memory/` に orchestrator / extractor / retriever / summarizer を配置する
- `server/internal/prompt/` に Prompt Builder / テンプレートを配置する

### 12.4 記憶スコアリング（Retriever）

```
total_score =
  0.50 * keyword_match     ← content / summary との一致度
  0.20 * recency_score     ← 直近 7 日は加点
  0.20 * importance        ← 書き込み時に付与（0-1）
  0.10 * confidence        ← 書き込み時に付与（0-1）
```

将来は `0.50 * vector_similarity` に置き換える（Qdrant 導入時）。

### 12.5 段階的実装順序

| ステップ | 追加する機能 |
| --- | --- |
| Step 1 | Profile Memory + Semantic Facts（memory_facts）+ Session Summary |
| Step 2 | Episodic Memory + Retriever スコアリング |
| Step 3 | Reflection Memory + LLM 要約連携 |
| Step 4 | Embedding 検索（Qdrant）、家族スコープ、感情タグ |

詳細タスクは `docs/project/phase10-tasklist.md` を参照。

---

このファイルは初版です。実装が進んだら、実際のディレクトリ構成、プロトコル定義、運用ポリシーに合わせて更新してください。
