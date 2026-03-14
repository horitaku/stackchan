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
- CORS_ALLOWED_ORIGINS: カンマ区切り
- PROVIDER_TIMEOUT_MS: provider 呼び出しタイムアウト（既定値 3000）
- PROVIDER_MAX_ATTEMPTS: provider 呼び出し最大試行回数（既定値 2）
- PROVIDER_RETRY_BASE_DELAY_MS: provider リトライ初期待機（既定値 100）

## フェーズ 4 への引き継ぎ

- audio.chunk / audio.end は最小実装済みですが、WebSocket binary の Opus フレーム転送は未対応です。
- error code は provider 由来まで拡張済みですが、request_id 完全連携は未実装です。
- session.welcome の heartbeat_interval_ms は未設定です。
- 音声キュー滞留時間と end-to-end レイテンシの詳細メトリクス追加が必要です。
