# tools

開発支援用ツールを格納するディレクトリです。
プロトコル検証、ログ解析、セッション再生を段階的に追加します。

## mise 補助スクリプト

- `tools/mise/fw.cjs`
	- firmware の build / upload / monitor を OS 非依存で実行します。
- `tools/mise/server.cjs`
	- server の run / restart / healthz を OS 非依存で実行します。

Node.js CommonJS で実装しているため、Linux（Raspberry Pi を含む）と Windows で同じタスク名を使えます。
