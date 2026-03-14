# フェーズ 2 タスクリスト（サーバースケルトン）

## 1. このドキュメントの目的

フェーズ 2（サーバースケルトン）を実行しやすくするために、作業を具体タスクへ分解して管理します。
本ドキュメントは「日次で更新する実行用リスト」です。

## 2. 運用ルール

- ステータスは `Planned`、`In Progress`、`Blocked`、`Done` を使用します。
- 作業開始時に `開始日` を記録し、完了時に `完了日` を記録します。
- `Blocked` になった場合は、必ず `ブロック理由` と `解除条件` を記載します。
- 1 タスクは原則 0.5 日から 2 日で終わる粒度に保ちます。

## 3. フェーズ 2 完了条件

次のすべてを満たしたらフェーズ 2 完了とします。

- Go モジュールとエントリーポイントが作成済みである。
- Gin ベースの HTTP サーバーと WebSocket エントリーポイントが起動できる。
- クライアントから session.hello を送信し、server から session.welcome を受信できる。
- WebSocket 受信時にエンベロープ検証が行われ、不正メッセージに error を返せる。
- direction ごとの sequence 管理（重複・逆転の検知）が実装されている。
- session_id を全ログに含む構造化ログが出力されている。
- hello/welcome フローに最低 1 つの自動テストが存在する。

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 依存 | 優先度 | 担当 | 見積 | ステータス | 開始日 | 完了日 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P2-01 | Go モジュールとディレクトリ構成を作成する | server/go.mod、server/cmd/stackchan-server/main.go | - | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-02 | Gin HTTP サーバーをブートストラップする | Gin エンジン起動、環境変数読み込み、ヘルスチェック API | P2-01 | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-03 | WebSocket エントリーポイントを実装する | server/internal/web/ws_handler.go、接続確立と切断の基本ハンドラ | P2-02 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-04 | エンベロープ検証を実装する | server/internal/protocol/envelope.go、受信メッセージの型・必須フィールド検証 | P2-03 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-05 | error イベント返却を共通化する | server/internal/protocol/error.go、error コード定義と送信ヘルパー | P2-04 | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-06 | セッション管理を実装する | server/internal/session/manager.go、session_id 生成・保持・破棄 | P2-03 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-07 | session.hello / session.welcome フローを実装する | server/internal/session/handshake.go、hello 受信→welcome 返却の振る舞い | P2-04、P2-06 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-08 | direction ごとの sequence 管理を実装する | server/internal/protocol/sequence.go、重複・逆転検知とログ出力 | P2-04 | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-09 | 構造化ログと相関 ID を導入する | server/internal/logging、JSON 形式ログ、session_id の全レイヤー引き回し | P2-06 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-10 | hello/welcome フローの自動テストを追加する | server/internal/session/handshake_test.go、最低 1 シナリオのエンドツーエンドテスト | P2-07、P2-08、P2-09 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-11 | フェーズ 3 引き継ぎ事項を整理する | server/README や docs 配下の引き継ぎメモ、Provider 境界着手前提の未決事項 | P2-07 から P2-10 | 中 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P2-12 | フェーズ 2 完了レビューを実施する | レビュー記録、未解決課題リスト | P2-01 から P2-11 | 中 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |

## 5. タスク詳細（実行手順）

### P2-01 Go モジュールとディレクトリ構成を作成する

- 作業内容
  - `server/` 配下に Go モジュールを初期化します（`go mod init`）。
  - `server/cmd/stackchan-server/main.go` を作成し、エントリーポイントを定義します。
  - `server/internal/` 配下のサブパッケージ構成（session、protocol、web、logging）を整備します。
- 完了条件
  - `go build ./...` がエラーなく通ります。
- 確認コマンド例
  - `go build ./...`
  - `Get-ChildItem server -Recurse -Name`

### P2-02 Gin HTTP サーバーをブートストラップする

- 作業内容
  - Gin エンジンを起動し、環境変数（`SERVER_ADDR`、`LOG_LEVEL`）を読み込みます。
  - `GET /healthz` エンドポイントを実装してサーバー起動確認に使います。
  - CORS ポリシー（`CORS_ALLOWED_ORIGINS`）を設定します。
- 完了条件
  - サーバーを起動し `GET /healthz` が 200 を返せます。
- 確認コマンド例
  - `go run ./cmd/stackchan-server/`
  - `Invoke-WebRequest -Uri http://localhost:8080/healthz`

### P2-03 WebSocket エントリーポイントを実装する

- 作業内容
  - `GET /ws` を WebSocket アップグレードエンドポイントとして追加します。
  - 接続確立・切断のライフサイクルを処理する基本ハンドラを実装します。
  - タイムアウト（`WS_READ_TIMEOUT`、`WS_WRITE_TIMEOUT`）を環境変数から設定します。
- 完了条件
  - WebSocket クライアントが `/ws` に接続し、接続・切断をサーバーが認識できます。
- 確認コマンド例
  - WebSocket クライアントツール（例: `wscat`）による手動疎通確認

### P2-04 エンベロープ検証を実装する

- 作業内容
  - 受信 JSON メッセージから共通エンベロープフィールド（type、timestamp、session_id、sequence、version、payload）の存在と型を検証します。
  - 検証失敗時は error イベントを返してメッセージを破棄します（`invalid_message`、`invalid_payload`）。
  - protocol/websocket/schemas の定義を実装の参照基準にします。
- 完了条件
  - 必須フィールド欠落、型不正、version 不一致に対して適切な error を返せます。
- 確認観点
  - validation-checklist.md の「Envelope」節の全項目が通ること。

### P2-05 error イベント返却を共通化する

- 作業内容
  - error コード定数（`invalid_message`、`unsupported_version`、`invalid_sequence`、`invalid_payload`）を定義します。
  - エンベロープ付き error メッセージを生成・送信するヘルパー関数を実装します。
  - `request_type` と `request_sequence` を error payload に関連付けられるようにします。
- 完了条件
  - 全ハンドラが同一のエラー送信ヘルパーを使っています。
- 確認観点
  - error payload に `code`、`message`、`retryable` が常に含まれること。

### P2-06 セッション管理を実装する

- 作業内容
  - WebSocket 接続ごとに一意の `session_id`（UUID v4）を生成します。
  - セッションのライフサイクル（作成・アクティブ・切断）をインメモリで管理します。
  - 接続切断時にセッションを自動クリーンアップします。
- 完了条件
  - 接続から切断まで session_id が一貫して維持されます。
- 確認観点
  - 異なる接続が異なる session_id を持つこと。
  - 切断後にセッションがメモリから除去されること。

### P2-07 session.hello / session.welcome フローを実装する

- 作業内容
  - `session.hello` 受信後に payload（device_id、client_type）を検証します。
  - 検証成功時は `session.welcome`（accepted: true、server_time: 現在 UTC）を返却します。
  - 検証失敗時は `session.welcome`（accepted: false）または `error` を返却し、切断します。
  - `session.hello` 受信前に他イベントを受信した場合は `error` を返します。
- 完了条件
  - クライアントから hello を送り、welcome を受信できます。
  - device_id 欠落や無効な client_type には error が返ります。
- 確認観点
  - session_id が welcome の後に全ログに一貫して記録されること。

### P2-08 direction ごとの sequence 管理を実装する

- 作業内容
  - `firmware->server` と `server->firmware` それぞれの sequence カウンタをセッションごとに管理します。
  - sequence 重複（同じ番号が来た場合）を検知し、再処理なしにスキップします。
  - sequence の逆転（前回より小さい番号が来た場合）を検知し、warning ログを出力します。
  - 送信時に server 側の sequence を自動でインクリメントします。
- 完了条件
  - 重複 sequence を受信してもメッセージが二重処理されません。
  - 逆転 sequence 受信時に warning ログが出ます。
- 確認観点
  - validation-checklist.md の「Sequence and Ordering」節の全項目が通ること。

### P2-09 構造化ログと相関 ID を導入する

- 作業内容
  - JSON 形式の構造化ログライブラリ（例: `zerolog` または `slog`）を導入します。
  - `session_id` を全ログエントリに含めます（context 経由で引き回し）。
  - error ログには `request_type` と `request_sequence` を含めます。
  - `LOG_LEVEL` 環境変数でログレベルを制御します。
- 完了条件
  - 全ての主要ログに session_id が含まれています。
  - ログが JSON フォーマットで出力されています。
- 確認観点
  - hello/welcome フロー中のログを見て相関が追えること。

### P2-10 hello/welcome フローの自動テストを追加する

- 作業内容
  - WebSocket テストクライアントを使ったエンドツーエンドテストを実装します。
  - 正常系（hello→welcome 受信）を 1 ケース以上カバーします。
  - 異常系（device_id 欠落、無効 client_type）の error 返却を確認するテストを追加します。
  - sequence 管理の単体テスト（重複・逆転の検知）を追加します。
- 完了条件
  - `go test ./...` がパスします。
  - 正常系と主要な異常系がテストでカバーされています。
- 確認コマンド例
  - `go test ./... -v`

### P2-11 フェーズ 3 引き継ぎ事項を整理する

- 作業内容
  - Provider 境界（STT、LLM、TTS インターフェース）の着手前提と未決事項をまとめます。
  - フェーズ 2 で判明した設計上の課題や仮定をメモに記録します。
  - audio.chunk 受信のバイナリ転送方式（フェーズ 1 持ち越し課題）について調査結果を記録します。
- 完了条件
  - フェーズ 3 着手時のブロッカーが明確です。
- 確認観点
  - セッション管理と Provider 連携の境界が明記されていること。

### P2-12 フェーズ 2 完了レビューを実施する

- 作業内容
  - 完了条件の満たし込み確認を行います。
  - 未解決事項をフェーズ 3 へ引き継ぎます。
- 完了条件
  - 合意済みのレビュー結果が記録されています。
- 確認観点
  - フェーズ 3（Provider 境界）の作業開始判断が可能であること。

## 6. 前提と設計メモ

### 採用技術候補

| 用途 | 候補 | 備考 |
| --- | --- | --- |
| WebSocket ライブラリ | `gorilla/websocket` | 実績・サポートが豊富 |
| 構造化ログ | `zerolog` または `slog`（標準） | JSON 出力、context 連携を重視 |
| UUID 生成 | `google/uuid` | session_id に使用 |
| 設定読み込み | `godotenv` または `os.Getenv` | `.env` サポートとシンプルさのバランス |

### セッション状態遷移

```text
接続確立
  └── セッション作成（session_id 付与）
        └── session.hello 受信待ち
              ├── session.hello 受信 → 検証 → session.welcome 送信 → Connected 状態
              ├── 他イベント受信 → error 返却 → 切断
              └── タイムアウト → 切断
Connected 状態
  └── audio.chunk / audio.end / error 受信可能
切断
  └── セッションクリーンアップ
```

### ディレクトリ構成（フェーズ 2 最小構成）

```text
server/
├── cmd/
│   └── stackchan-server/
│       └── main.go
├── internal/
│   ├── web/
│   │   └── ws_handler.go
│   ├── session/
│   │   ├── manager.go
│   │   ├── handshake.go
│   │   └── handshake_test.go
│   ├── protocol/
│   │   ├── envelope.go
│   │   ├── error.go
│   │   └── sequence.go
│   └── logging/
│       └── logger.go
├── go.mod
└── go.sum
```

## 7. ブロッカー管理

| 日付 | タスク ID | ブロック理由 | 解除条件 | オーナー | 状態 |
| --- | --- | --- | --- | --- | --- |
| - | - | - | - | - | - |

## 7.1 フェーズ 2 完了レビュー記録

- レビュー日: 2026-03-14
- レビュー結果: フェーズ 2 の完了条件をすべて満たす
- 確認事項:
  - Go モジュール、Gin サーバー、WebSocket エントリーポイントを作成済み
  - session.hello / session.welcome の往復動作を自動テストで確認済み
  - エンベロープ検証、error 応答、sequence 管理を実装済み
  - session_id を含む JSON 構造化ログを導入済み
  - go build ./... と go test ./... -v -timeout 30s が通過済み
- 未解決課題:
  - audio.chunk / audio.end の実処理はフェーズ 4 で実装する
  - heartbeat_interval_ms の既定値はフェーズ 3 以降で確定する
  - provider 由来エラーコード体系はフェーズ 3 で拡張する

## 8. フェーズ 2 実績メモ

- 開始日: 2026-03-14
- 目標完了日: 2026-03-14
- 完了日: 2026-03-14
- 主な学び: protocol first で定義した最小契約があったことで、hello/welcome と sequence 管理を薄い縦スライスとして安全に実装できた
- 次フェーズへの持ち越し: Provider 境界の定義、heartbeat_interval_ms の運用方針、audio chunk の実処理設計
