# server

Go + Gin で API/WebSocket と実行制御を提供します。
session 管理、conversation 管理、provider 連携を段階的に実装します。

## 現在の実装範囲

- Gin ベースの HTTP サーバー起動
- GET /healthz によるヘルスチェック
- GET /ws による WebSocket 接続受付
- session.hello 受信と session.welcome 返却
- エンベロープ検証と error イベント返却
- direction ごとの sequence 管理
- session_id を含む JSON 構造化ログ
- STT / LLM / TTS の provider interface と依存注入
- mock provider による最小オーケストレーション（STT -> LLM -> TTS）
- audio.chunk 蓄積と audio.end トリガー処理
- provider エラーの protocol error マッピング

## 現在のディレクトリ構成

```text
server/
├── cmd/
│   └── stackchan-server/
│       └── main.go
├── internal/
│   ├── logging/
│   │   └── logger.go
│   ├── protocol/
│   │   ├── envelope.go
│   │   ├── error.go
│   │   ├── sequence.go
│   │   └── sequence_test.go
│   ├── providers/
│   │   ├── interfaces.go
│   │   ├── errors.go
│   │   ├── retry.go
│   │   ├── errors_test.go
│   │   ├── retry_test.go
│   │   └── mock/
│   │       ├── stt.go
│   │       ├── llm.go
│   │       └── tts.go
│   ├── conversation/
│   │   ├── orchestrator.go
│   │   └── orchestrator_test.go
│   ├── session/
│   │   ├── audio_stream.go
│   │   ├── handshake.go
│   │   ├── handshake_test.go
│   │   └── manager.go
│   └── web/
│       ├── ws_handler.go
│       └── ws_handler_test.go
├── go.mod
└── go.sum
```

## ローカル起動

1. server ディレクトリへ移動します。
2. 必要に応じて .env を配置します。
3. go run ./cmd/stackchan-server を実行します。

主要な環境変数:

- SERVER_ADDR: 既定値 :8080
- LOG_LEVEL: 既定値 info
- WS_READ_TIMEOUT: 既定値 30
- WS_WRITE_TIMEOUT: 既定値 30
- VOICEVOX_HTTP_TIMEOUT_SEC: Voicevox API 呼び出しの HTTP タイムアウト秒（既定値 45）
- CORS_ALLOWED_ORIGINS: カンマ区切り
- PROVIDER_TIMEOUT_MS: provider 呼び出しタイムアウト（既定値 3000）
- PROVIDER_MAX_ATTEMPTS: provider 呼び出し最大試行回数（既定値 2）
- PROVIDER_RETRY_BASE_DELAY_MS: provider リトライ初期待機（既定値 100）

## フェーズ 4 への引き継ぎ

- audio.chunk / audio.end は最小実装済みですが、WebSocket binary の Opus フレーム転送は未対応です。
- error code は provider 由来まで拡張済みですが、request_id 完全連携は未実装です。
- session.welcome の heartbeat_interval_ms は未設定です。
- 音声キュー滞留時間と end-to-end レイテンシの詳細メトリクス追加が必要です。

## フェーズ 7 API / WebUI

フェーズ 7 で追加したエンドポイント:

- GET /api/runtime/overview
  - 接続状態、再生状態、会話パイプライン遅延、アバター同期状態を返します。
- GET /api/settings
  - 現在の runtime 設定を返します。
- PUT /api/settings
  - 音量、表情プリセット、リップシンク係数、モーション有効化を更新します。
- POST /api/tests/pipeline
  - STT -> LLM -> TTS の最小疎通テストを実行し、遅延を返します。

WebUI:

- /ui/
  - 可観測性ダッシュボード
  - 設定変更フォーム
  - 疎通テスト実行ボタン
  - 閾値ベースアラート表示

WebUI 実装方式:

- Svelte + Vite 構成（`server/webui`）
- 本番相当は `npm run build` で `server/webui/dist` を生成し、Go が `/ui` 配下で配信

WebUI ローカル開発:

- cd server/webui
- npm install
- npm run dev
- Vite 開発サーバー: http://localhost:5173
- `/api` と `/ws` は `vite.config.js` の proxy で `http://localhost:8080` へ中継

WebUI 本番相当ビルド:

- cd server/webui
- npm install
- npm run build
- 生成物: `server/webui/dist`

## フェーズ 7 手動確認手順

1. サーバー起動
	- cd server
	- go run ./cmd/stackchan-server
2. WebUI ビルド（初回またはUI更新後）
  - cd server/webui
  - npm install
  - npm run build
3. ブラウザで /ui/ を開く
	- 例: http://localhost:8080/ui/
4. ダッシュボード確認
	- 接続状態、再生状態、遅延指標が表示されること
5. 設定更新確認
	- 音量などを変更して保存し、成功メッセージが表示されること
6. 疎通テスト確認
	- 疎通テスト実行で request_id と latency が表示されること
7. 異常表示確認
	- 接続未確立時、警告表示が出ること

## フェーズ 7 自動テスト

- cd server
- go test ./... -timeout 120s
