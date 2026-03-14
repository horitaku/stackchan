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
│   ├── session/
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

## フェーズ 3 への引き継ぎ

- audio.chunk / audio.end は受信前提のゲートだけがあり、実処理は未実装です。
- error code は最小セットのみ実装済みで、provider 由来エラー体系は未定義です。
- session.welcome の heartbeat_interval_ms は未設定です。
- Provider 境界は session / protocol / web から分離する構成で追加してください。
- WebSocket のバイナリ音声転送は将来拡張です。現状は JSON テキストメッセージのみ対応です。
