# フェーズ 5 タスクリスト（Firmware 接続性）

## 1. このドキュメントの目的

フェーズ 5（Firmware 接続性）を実行しやすくするために、作業を具体タスクへ分解して管理します。
本ドキュメントは「日次で更新する実行用リスト」です。

## 2. 運用ルール

- ステータスは `Planned`、`In Progress`、`Blocked`、`Done` を使用します。
- 作業開始時に `開始日` を記録し、完了時に `完了日` を記録します。
- `Blocked` になった場合は、必ず `ブロック理由` と `解除条件` を記載します。
- 1 タスクは原則 0.5 日から 2 日で終わる粒度に保ちます。

## 3. フェーズ 5 完了条件

次のすべてを満たしたらフェーズ 5 完了とします。

- firmware C++ コードが PlatformIO でビルドできる（コンパイルエラーなし）。
- M5Stack CoreS3 が Wi-Fi に接続し、WebSocket サーバーへ接続できる。
- 指数バックオフによる再接続ロジックが実装されている（FW_RECONNECT_BASE_MS / FW_RECONNECT_MAX_MS に従う）。
- firmware から session.hello を送信し、server から session.welcome を受信できる。
- heartbeat_interval_ms の運用値が確定し、firmware と server の両側でキープアライブが動作する。
- 最小音声送信フロー（audio.stream_open → binary frames → audio.end）が firmware から送信できる。
- stt.final / tts.end を firmware で受信し、シリアルモニターにデバッグログが出力される。
- audio.stream_open の JSON Schema と example が protocol に追加されている。
- firmware の責務がデバイス I/O とプロトコル処理に限定されている（AI オーケストレーションロジックを持たない）。
- フェーズ 6（再生とアバター同期）への引き継ぎ事項が記録されている。

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 依存 | 優先度 | 担当 | 見積 | ステータス | 開始日 | 完了日 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P5-01 | firmware ディレクトリ構造とビルド環境を整備する | firmware/ 配下のディレクトリ構成、platformio.ini、依存ライブラリ追加 | - | 高 | Copilot | 0.5 日 | Planned | - | - |
| P5-02 | Wi-Fi 接続モジュールを実装する | firmware/runtime/network/wifi.h・wifi.cpp、接続状態管理 | P5-01 | 高 | Copilot | 1.0 日 | Planned | - | - |
| P5-03 | WebSocket クライアントを実装する | firmware/runtime/network/ws_client.h・ws_client.cpp、テキスト・バイナリ送受信基盤 | P5-01 | 高 | Copilot | 1.0 日 | Planned | - | - |
| P5-04 | 指数バックオフ再接続ロジックを実装する | firmware/runtime/network/ 再接続ロジック、FW_RECONNECT_BASE_MS / FW_RECONNECT_MAX_MS 適用 | P5-02、P5-03 | 高 | Copilot | 0.5 日 | Planned | - | - |
| P5-05 | プロトコルメッセージ送受信ヘルパーを実装する | firmware/protocol/envelope.h・envelope.cpp、sequence カウンタ管理、JSON パース | P5-03 | 高 | Copilot | 1.0 日 | Planned | - | - |
| P5-06 | session.hello/welcome フローを実装する | firmware/app/stackchan/session.h・session.cpp、hello 送信・welcome 解析・接続状態遷移 | P5-04、P5-05 | 高 | Copilot | 1.0 日 | Planned | - | - |
| P5-07 | heartbeat を実装する（firmware + server） | firmware 側キープアライブ送信、server 側タイムアウト整合確認、heartbeat_interval_ms 運用値確定 | P5-06 | 高 | Copilot | 1.0 日 | Planned | - | - |
| P5-08 | 最小音声送信フローを実装する | firmware からの audio.stream_open → binary フレーム送信 → audio.end 送信、テスト用 PCM データ活用 | P5-05、P5-06 | 高 | Copilot | 1.5 日 | Planned | - | - |
| P5-09 | stt.final / tts.end 受信とデバッグログを実装する | 受信 JSON パース、transcript / codec / duration_ms のシリアルログ出力 | P5-06、P5-08 | 高 | Copilot | 0.5 日 | Planned | - | - |
| P5-10 | audio.stream_open の JSON Schema と example を作成する | protocol/websocket/schemas/audio.stream_open.schema.json、protocol/examples/audio.stream_open.example.json | - | 中 | Copilot | 0.5 日 | Planned | - | - |
| P5-11 | firmware 疎通確認手順を文書化する | firmware/README.md の更新、シリアルモニター・wscat を使った手動確認手順 | P5-01 から P5-09 | 中 | Copilot | 0.5 日 | Planned | - | - |
| P5-12 | フェーズ 6 引き継ぎ事項を整理する | 再生・アバター同期に向けた前提条件・未決事項メモ | P5-01 から P5-11 | 中 | Copilot | 0.5 日 | Planned | - | - |
| P5-13 | フェーズ 5 完了レビューを実施する | レビュー記録、未解決課題リスト | P5-01 から P5-12 | 中 | Copilot | 0.5 日 | Planned | - | - |

## 5. タスク詳細（実行手順）

### P5-01 firmware ディレクトリ構造とビルド環境を整備する

- 作業内容
  - copilot-instructions.md に記載されたディレクトリ構成（runtime/, boards/, protocol/, app/）を作成します。
  - `platformio.ini.example` をもとに `platformio.ini` を整備し、必要な依存ライブラリを追加します。
    - `links2004/WebSockets`（WebSocket クライアント）
    - `bblanchon/ArduinoJson`（JSON シリアライズ/デシリアライズ）
    - `m5stack/M5Unified`（CoreS3 向けハードウェア統合ライブラリ）
  - `firmware/main.cpp` に最小エントリーポイント（`setup()` / `loop()`）を作成します。
  - `.gitignore` に `platformio.ini.local`、`include/secrets.h` などのローカル設定ファイルを確実に除外します。
- 完了条件
  - `pio run --target clean` がエラーなく完了する（または同等のビルド確認が取れる）。
- 確認観点
  - `firmware/` 配下のディレクトリ構成が copilot-instructions.md の 8.1 に準拠していること。
  - ローカル秘密情報がコミット対象にならないこと。

### P5-02 Wi-Fi 接続モジュールを実装する

- 作業内容
  - `firmware/runtime/network/wifi.h`・`wifi.cpp` を作成し、Wi-Fi 接続ロジックをカプセル化します。
  - `FW_WIFI_SSID` / `FW_WIFI_PASSWORD`（`include/secrets.h` から参照）を使って接続します。
  - 接続成功・失敗・再試行の状態を返す API を定義します。
  - 接続試行中はシリアルログに進捗を出力します。
- 完了条件
  - Wi-Fi 接続が確立し、IPアドレスを取得できます。
  - パスワード・SSID はログへ平文出力しません（マスク必須）。
- 確認観点
  - `FW_WIFI_PASSWORD` がシリアルログに出力されないこと。
  - 接続失敗時にリトライ回数とエラー理由がログに残ること。

### P5-03 WebSocket クライアントを実装する

- 作業内容
  - `firmware/runtime/network/ws_client.h`・`ws_client.cpp` を作成します。
  - `links2004/WebSockets` ライブラリをラップし、テキストメッセージ・バイナリメッセージの送受信 API を提供します。
  - `FW_WS_URL` への接続、切断検知、メッセージ受信コールバック登録を実装します。
  - `FW_WS_TOKEN` が設定されている場合は、接続時の Authorization ヘッダへ付与します。
- 完了条件
  - WebSocket サーバーへの接続が確立します。
  - テキスト JSON メッセージとバイナリメッセージを送受信できます。
- 確認観点
  - `FW_WS_TOKEN` がログへ平文出力されないこと。
  - 切断イベントが正しく検知されること。

### P5-04 指数バックオフ再接続ロジックを実装する

- 作業内容
  - WebSocket または Wi-Fi 切断時に指数バックオフで再接続を試みるロジックを実装します。
    - 初回待機: `FW_RECONNECT_BASE_MS`（例: 500ms）
    - 最大待機: `FW_RECONNECT_MAX_MS`（例: 10000ms）
    - `FW_RECONNECT_MAX_MS` 到達後は一定間隔（`FW_RECONNECT_MAX_MS`）でリトライを継続します。
  - 再接続試行回数と待機時間をシリアルログに記録します。
- 完了条件
  - サーバーが一時停止した状態でも、firmware がクラッシュせず自動再接続を試みます。
- 確認観点
  - 無限に待機時間が伸び続けないこと（上限でキャップされること）。
  - 再接続成功時に再接続カウンタがリセットされること。

### P5-05 プロトコルメッセージ送受信ヘルパーを実装する

- 作業内容
  - `firmware/protocol/envelope.h`・`envelope.cpp` に共通エンベロープの生成・解析ロジックを実装します。
    - 生成: type、timestamp（RFC3339 UTC）、session_id、sequence、version=`"1.0"`、payload を組み立てます。
    - 解析: 受信 JSON からエンベロープを分解し、type と payload を返します。
  - firmware -> server 方向の sequence カウンタをセッションごとに管理します。
  - `ArduinoJson` を使用して JSON シリアライズ/デシリアライズを行います。
- 完了条件
  - 送受信の JSON メッセージが仕様通りのエンベロープ構造を持ちます。
  - sequence が送信ごとに単調増加します。
- 確認観点
  - timestamp が UTC ISO 8601 形式で生成されること。
  - session_id が `session.welcome` 受信後に正しくセットされること。

### P5-06 session.hello/welcome フローを実装する

- 作業内容
  - `firmware/app/stackchan/session.h`・`session.cpp` にセッション状態管理を実装します。
  - WebSocket 接続確立後に `session.hello` を自動送信します。
    - `FW_DEVICE_ID`、`client_type: "firmware"`、`protocol_capabilities` をペイロードに設定します。
  - `session.welcome` 受信後に、following を実行します。
    - `accepted: true` を確認し、`session_id` を保持します。
    - `heartbeat_interval_ms` を取得して保持します。
    - 接続状態を `Connected` へ遷移します。
  - `session.hello` 以外のメッセージを `Connected` 前に受信した場合は破棄します。
- 完了条件
  - firmware から hello を送信し、server から welcome を受信できます。
  - 接続・切断・再接続時のセッション状態遷移が正しく動作します。
- 確認観点
  - 再接続後に session_id がリセットされ、新しい hello を送信すること。
  - `accepted: false` 受信時にエラーログを出力してセッションをクローズすること。

### P5-07 heartbeat を実装する（firmware + server）

- 作業内容
  - **firmware 側**
    - `session.welcome` で受け取った `heartbeat_interval_ms` の間隔で `heartbeat` イベントを server へ送信します。
    - `heartbeat_interval_ms` が省略された場合は `15000`（15 秒）をデフォルト値とします。
  - **server 側**
    - `heartbeat` イベント受信時にログを記録し、セッションの最終活動時刻を更新します。
    - `session.welcome` の `heartbeat_interval_ms` フィールドに運用値（`15000`）を設定します。
    - `WS_READ_TIMEOUT` の値が `heartbeat_interval_ms` の 2〜3 倍以上になるよう環境変数のデフォルト値を調整します。
  - **プロトコル文書**
    - `heartbeat` イベントを `protocol/websocket/events.md` に追記します。
- 完了条件
  - firmware が 15 秒ごとに heartbeat を送信し、server がログに記録します。
  - heartbeat_interval_ms の運用値（15000ms）が確定しています。
- 確認観点
  - `heartbeat_interval_ms` × 3 未満の `WS_READ_TIMEOUT` は設定しないこと。
  - heartbeat 送信が WebSocket 切断後に停止すること。

### P5-08 最小音声送信フローを実装する

- 作業内容
  - `firmware/runtime/audio/mic_reader.h`・`mic_reader.cpp` に最小マイク読み取りインターフェースを定義します。
  - テスト用サイレンス PCM（16kHz、16bit、モノラル）を生成し、実機マイク代替として利用できるようにします（`FW_LOG_LEVEL=debug` 時のみ有効化）。
  - 以下の順でサーバーへ音声データを送信します。
    1. UUID v4 の `stream_id` を生成します。
    2. `audio.stream_open` イベント（JSON）を送信します。
    3. 先頭 36 バイトに `stream_id` ASCII 文字列を埋め込んだバイナリフレームを 20ms 間隔で送信します。
    4. 録音終了（または一定フレーム数到達）後に `audio.end` イベントを送信します。
  - 送信フレーム数と各フレームのバイトサイズをシリアルログに記録します。
- 完了条件
  - server のシリアルログに `stt.final`、`tts.end` の受信ログが出力されます（mock provider 接続時）。
- 確認観点
  - `stream_id` が先頭 36 バイトに ASCII 文字列として格納されること。
  - `audio.stream_open` が最初のバイナリフレームよりも先に送信されること。
  - フェーズ 5 では raw PCM 転送を許容し、Opus エンコードはフェーズ 6 以降に延期する。

### P5-09 stt.final / tts.end 受信とデバッグログを実装する

- 作業内容
  - `firmware/protocol/` にイベントタイプ別の受信ディスパッチロジックを追加します。
  - `stt.final` 受信時に `transcript` および `confidence` をシリアルログに出力します。
  - `tts.end` 受信時に `request_id`、`codec`、`duration_ms`、`sample_rate_hz` をシリアルログに出力します。
  - `audio_base64` フィールドのデコードは本タスクのスコープ外とし、フェーズ 6 へ持ち越します。
- 完了条件
  - firmware がサーバーからの `stt.final`・`tts.end` を受信し、シリアルモニターにその内容が表示されます。
- 確認観点
  - `audio_base64` の巨大データがシリアルバッファを溢れさせないようにサイズをログへの出力から除外すること。
  - `request_id` で送信ストリームと受信結果を紐付けてログに残すこと。

### P5-10 audio.stream_open の JSON Schema と example を作成する

- 作業内容
  - `protocol/websocket/schemas/audio.stream_open.schema.json` を作成します。
    - 必須: `stream_id`（string）、`codec`（enum: opus、pcm）、`sample_rate_hz`（enum: 16000、24000、48000）
    - 任意: `frame_duration_ms`（enum: 10、20、40、60）、`channel_count`（enum: 1、2）
  - `protocol/examples/audio.stream_open.example.json` を作成します。
  - `protocol/websocket/events.md` の 5.1 節（audio.stream_open）のフィールド定義を schema と整合させます。
- 完了条件
  - JSON Schema が他の schema ファイル（envelope.schema.json 等）と同じ構造・形式で作成されています。
- 確認観点
  - `$schema`、`title`、`description`、`required`、`properties` が含まれること。
  - `codec: "pcm"` をフェーズ 5 向けに許容し、`codec: "opus"` はフェーズ 6 以降で有効化される旨をコメントで記述すること。

### P5-11 firmware 疎通確認手順を文書化する

- 作業内容
  - `firmware/README.md` を更新し、以下の内容を追記します。
    - 環境セットアップ手順（PlatformIO 拡張機能のインストール、依存ライブラリのインストール）
    - ローカル設定ファイルの準備方法（`platformio.ini.local` および `include/secrets.h` のコピー手順）
    - シリアルモニターを使った動作確認方法（接続状態ログ、hello/welcome ログの見方）
    - `wscat` を使ったサーバー側の手動疎通確認コマンド例
    - トラブルシューティング FAQ（Wi-Fi 接続失敗、WebSocket 接続失敗、heartbeat 途絶）
- 完了条件
  - README を読んだ開発者が環境構築から疎通確認まで実施できる手順になっています。
- 確認観点
  - 機密情報（SSID、パスワード）を README に記載しないこと。
  - コマンド例が Windows（PowerShell）および macOS/Linux の両方で動作すること（環境差があれば注記する）。

### P5-12 フェーズ 6 引き継ぎ事項を整理する

- 作業内容
  - フェーズ 6（再生とアバター同期）に向けた前提条件と未決事項をまとめます。
  - 以下の持ち越し事項を記録します。
    - Opus エンコード/デコード導入（firmware 側: `libopus` または同等ライブラリ）
    - `audio_base64` のデコードと M5Stack スピーカーへの再生実装
    - m5stack-avatar との統合（リップシンク、表情、まばたき）
    - フェーズ 5 で計測したレイテンシの初期値記録
  - `audio.stream_open` の `codec` フィールドを `"opus"` へ変更する際の移行手順を記録します。
- 完了条件
  - フェーズ 6 着手時のブロッカーが明確です。
- 確認観点
  - server 側・firmware 側・protocol 側の未解決事項がそれぞれ明示されていること。

### P5-13 フェーズ 5 完了レビューを実施する

- 作業内容
  - フェーズ 5 完了条件の満たし込み確認を行います。
  - 未解決事項をフェーズ 6 へ引き継ぎます。
- 完了条件
  - 合意済みのレビュー結果が記録されています。
- 確認観点
  - フェーズ 6（再生とアバター同期）の作業開始判断が可能であること。

## 6. 前提と設計メモ

### 現状の実装状況（フェーズ 4 からの引き継ぎ）

| 項目 | 状態 | 備考 |
| --- | --- | --- |
| WebSocket サーバー（Go/Gin） | 実装済み | server/internal/web/ws_handler.go |
| session.hello / session.welcome | 実装済み（server 側） | server/internal/session/handshake.go |
| audio.stream_open 受信（server 側） | 実装済み | ws_handler.go |
| binary フレーム受信と AudioChunk 変換 | 実装済み | server/internal/session/audio_stream.go |
| stt.final / tts.end 送信（server 側） | 実装済み | ws_handler.go |
| audio.stream_open JSON Schema | **未作成** | P5-10 で対応 |
| heartbeat イベント定義・実装 | **未実装** | P5-07 で対応 |
| firmware C++ 実装 | **未実装** | P5-01〜P5-09 で対応 |
| Opus エンコード/デコード | **未実装** | フェーズ 6 以降 |

### 採用技術候補（firmware）

| 用途 | 候補 | 備考 |
| --- | --- | --- |
| PlatformIO フレームワーク | espressif32 + Arduino | M5Stack CoreS3 対応 |
| UI・ハードウェア統合 | `m5stack/M5Unified` | CoreS3 対応の公式統合ライブラリ |
| WebSocket クライアント | `links2004/WebSockets` | ESP32 向け実績が豊富 |
| JSON シリアライズ | `bblanchon/ArduinoJson` v7 | メモリ効率に優れる |
| UUID 生成 | `ESP.getEfuseMac()` ＋ 自前実装 | 起動時 stream_id 生成に使用 |

### firmware ディレクトリ構成

```text
firmware/
├── main.cpp                    // エントリーポイント（setup / loop）
├── platformio.ini              // ビルド設定（git 管理対象）
├── platformio.ini.example      // テンプレート
├── platformio.ini.local        // ローカル設定（git 除外）
├── include/
│   ├── secrets.example.h       // テンプレート
│   └── secrets.h               // ローカル認証情報（git 除外）
├── runtime/
│   ├── network/
│   │   ├── wifi.h
│   │   ├── wifi.cpp
│   │   ├── ws_client.h
│   │   └── ws_client.cpp
│   └── audio/
│       ├── mic_reader.h
│       └── mic_reader.cpp
├── boards/
│   └── cores3/
│       ├── board_config.h      // CoreS3 固有ピン定義・初期化
│       └── board_config.cpp
├── protocol/
│   ├── envelope.h
│   ├── envelope.cpp
│   └── events.h               // イベント type 定数定義
└── app/
    └── stackchan/
        ├── session.h
        └── session.cpp
```

### バイナリフレームフォーマット（フェーズ 4 確定仕様）

```text
[ bytes  0-35 ]  stream_id（UUID ASCII 文字列、固定 36 バイト）
[ bytes 36-   ]  音声データ（フェーズ 5: raw PCM、フェーズ 6 以降: Opus）
```

- `audio.stream_open` JSON イベントを必ず先に送信してからバイナリフレームを送信すること。

### heartbeat イベント定義（P5-07 で events.md に追記）

```text
heartbeat（firmware -> server）
  payload:
    uptime_ms: integer (required)   // firmware 起動からの経過時間（ms）
    rssi: integer (optional)        // Wi-Fi 信号強度（dBm）
```

### セッション状態遷移（firmware 側）

```text
起動
  └── Wi-Fi 接続
        ├── 失敗 → 指数バックオフ再試行
        └── 成功 → WebSocket 接続
              ├── 失敗 → 指数バックオフ再試行
              └── 成功 → session.hello 送信
                    ├── welcome (accepted=false) → ログ出力・切断
                    └── welcome (accepted=true) → Connected 状態
                          ├── heartbeat 送信（定期）
                          ├── audio.stream_open → binary frames → audio.end 送信
                          │     └── stt.final / tts.end 受信 → ログ出力
                          └── 切断 → 指数バックオフ再接続
```

## 7. ブロッカー管理

| 日付 | タスク ID | ブロック理由 | 解除条件 | オーナー | 状態 |
| --- | --- | --- | --- | --- | --- |
| - | - | - | - | - | - |
