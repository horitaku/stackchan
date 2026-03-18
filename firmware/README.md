# firmware

M5Stack 向けのランタイムとデバイス制御層を管理します。
責務はデバイス I/O とプロトコル処理に限定し、AI オーケストレーションは持ち込みません。

## 1. 前提条件

- [PlatformIO IDE](https://platformio.org/install/ide?install=vscode) または PlatformIO Core CLI
- M5Stack CoreS3 本体
- USB-C ケーブル（データ転送対応のもの）
- 同一ネットワーク上で稼働している stackchan サーバー（`server/` 参照）

## 2. ローカル設定ファイルの準備

Wi-Fi 認証情報やサーバー URL は git 管理外のファイルで管理します。

### 2.1 platformio.ini.local

```ini
; このファイルは git 管理外です。platformio.ini.local.example をコピーして作成してください。
[env:m5stack-cores3]
build_flags =
  ${env.build_flags}
  -DFW_WIFI_SSID='"YourSSID"'
  -DFW_WIFI_PASSWORD='"YourPassword"'
  -DFW_WS_URL='"ws://192.168.1.10:8080/ws"'
  -DFW_DEVICE_ID='"stackchan-cores3-01"'
```

### 2.2 include/secrets.h

```bash
cp include/secrets.example.h include/secrets.h
# include/secrets.h を編集して実際の値を入力してください
```

> **注意**: `platformio.ini.local` と `include/secrets.h` は `.gitignore` で無視されています。
> 実値を絶対にコミットしないでください。

## 3. ビルドとフラッシュ

### 3.0 PlatformIO 実行方式（Linux / Windows）

- Linux（Raspberry Pi 含む）では `pio` または `python3 -m platformio` を使用します。
- Windows では `py -m platformio` を標準コマンドとして使用します。
- `mise` タスク（`mise run fw:*`）を使う場合は、OS ごとの違いを吸収して実行します。

```bash
# Linux: ビルドのみ
pio run -e stackchan_cores3

# Linux: pio が PATH にない場合
python3 -m platformio run -e stackchan_cores3

# Windows: ビルドのみ
py -m platformio run -e stackchan_cores3

# Linux: ビルド + フラッシュ
pio run -e stackchan_cores3 --target upload

# Windows: ビルド + フラッシュ
py -m platformio run -e stackchan_cores3 --target upload

# Linux: シリアルモニター（例: /dev/ttyACM0）
pio device monitor --baud 115200 --port /dev/ttyACM0

# Windows: シリアルモニター（例: COM5）
py -m platformio device monitor --baud 115200
```

## 4. 起動後の疎通確認

### 4.1 シリアルモニターで確認すること

正常起動時のログ出力順序:

```text
[I] Wi-Fi connecting... SSID=YourSSID
[I] Wi-Fi connected. IP=192.168.x.x
[I] NTP sync started
[I] WebSocket connecting to ws://192.168.1.10:8080/ws
[I] WebSocket connected
[I] Sending session.hello device_id=stackchan-cores3-01
[I] session.welcome received accepted=true session_id=xxxxxxxx-...
[I] Heartbeat sent uptime_ms=15000
```

### 4.2 wscat でサーバー側から確認する

サーバーが受信しているメッセージをターミナルから確認できます。

```bash
# wscat のインストール（初回のみ）
npm install -g wscat

# WebSocket 接続（サーバーが localhost:8080 で起動している場合）
wscat -c ws://localhost:8080/ws

# 手動で session.hello を送信して session.welcome が返ることを確認
> {"type":"session.hello","timestamp":"2026-01-01T00:00:00Z","session_id":"","sequence":1,"version":"1.0","payload":{"device_id":"test-01","client_type":"test_harness","protocol_capabilities":{"audio_chunk":true,"audio_end":true}}}
```

### 4.3 タッチパネルで音声送信テスト

CoreS3 の画面をタッチすると `sendAudioStream(50)` が実行されます（フェーズ 5 ではダミー PCM）。
サーバー側のログに以下が出れば成功です:

```text
binary stream registered stream_id=... codec=pcm sample_rate_hz=16000
audio stream consumed, starting orchestration
```

## 5. ディレクトリ構成

```text
firmware/
├ main.cpp                        # エントリーポイント（setup/loop）
├ platformio.ini                  # PlatformIO ビルド設定
├ platformio.ini.example          # ローカル設定テンプレート
├ boards/
│  └ cores3/
│     ├ board_config.h/.cpp       # M5Stack CoreS3 ハードウェア初期化
├ runtime/
│  ├ network/
│  │  ├ wifi.h/.cpp               # Wi-Fi 接続・NTP 同期
│  │  └ ws_client.h/.cpp          # WebSocket クライアント（再接続付き）
│  └ audio/
│     └ mic_reader.h/.cpp         # マイク読み取り（Phase 5 はダミー PCM）
├ protocol/
│  ├ events.h                     # イベント type 文字列定数
│  └ envelope.h/.cpp              # エンベロープ構築・UUID・タイムスタンプ
├ app/
│  └ stackchan/
│     └ session.h/.cpp            # セッション状態マシン（メインロジック）
└ include/
   ├ secrets.example.h            # シークレットテンプレート
   └ secrets.h                    # ローカル専用（git 管理外）
```

## 6. トラブルシューティング

| 症状 | 確認事項 |
| ------ | --------- |
| Wi-Fi 接続が失敗する | `FW_WIFI_SSID` / `FW_WIFI_PASSWORD` が正しいか確認。2.4GHz のみ対応 |
| WebSocket 接続が失敗する | `FW_WS_URL` のアドレスがサーバーの IP と一致しているか確認。サーバーが起動しているか確認 |
| `session.welcome` が返ってこない | サーバーのログを確認。`session.hello` が到達しているか確認 |
| タッチしても音声送信ログが出ない | シリアルモニターで `State != Active` が出ていないか確認 |
| ビルドエラー: `secrets.h not found` | `include/secrets.h` を作成したか確認（`secrets.example.h` をコピー） |
| `platformio.ini.local not found` の警告 | 警告は無視可能。ただし Wi-Fi/WS の設定値が反映されないため要確認 |
| `self-signed certificate in certificate chain` / `CERTIFICATE_VERIFY_FAILED` | 企業プロキシ配下の TLS 検証エラー。対処手順は [docs/project/secrets-operations.md](docs/project/secrets-operations.md) の「企業プロキシ環境での TLS エラー対応（PlatformIO）」を参照 |
| `pio` が見つからない（`command not found`） | Linux は `python3 -m platformio` を利用。Windows は `py -m platformio` を利用 |

## 7. Phase 6 に向けた注意事項

- 現フェーズでは送信側は引き続き**ダミー PCM（無音）**です。実マイク収音は後続タスクで実装します
- `firmware/runtime/audio/mic_reader.cpp` の `readFrame()` に `M5.Mic.record()` を追加予定です
- Opus エンコード送信は後続タスクで導入し、再生側は Phase 6 最小実装として PCM 再生を優先します

## 8. Phase 6 最小確認（再生とアバター同期）

### 8.1 firmware 側で確認するログ

`tts.end` 受信後に次のログが出ることを確認します。

```text
[TTS] request_id=... playback started codec=pcm duration_ms=...
[TTSPlayer] playback started sample_rate=16000 duration_est_ms=...
[Avatar] expression=happy
[Avatar] motion=nod
```

### 8.2 画面表示で確認する内容

- 画面下部に `Expr` と `Motion` の状態が表示されること
- `Playback` が `Playing` 相当の値に遷移すること
- 口開閉バー（緑）が再生中に変化すること

### 8.3 手動テストシナリオ

1. サーバーを起動して firmware を接続します。
2. 画面タップで音声ストリームを送信します。
3. `stt.final` / `tts.end` を受信した後に再生ログが出ることを確認します。
4. サーバー側から `avatar.expression` と `motion.play` を送信し、状態が反映されることを確認します。

`wscat` 送信例:

```json
{"type":"avatar.expression","timestamp":"2026-03-15T10:00:08Z","session_id":"<session_id>","sequence":100,"version":"1.0","payload":{"request_id":"stream-001","expression":"happy","intensity":0.8}}
```

```json
{"type":"motion.play","timestamp":"2026-03-15T10:00:09Z","session_id":"<session_id>","sequence":101,"version":"1.0","payload":{"request_id":"stream-001","motion":"nod","speed":1.0}}
```

## 9. P8-07 最小確認（M5Stack-Avatar 顔表示先行）

### 9.1 期待挙動

- 起動直後に M5Stack-Avatar の顔が表示されること
- `session.welcome` 受信後に待機状態へ遷移し、顔表示が継続すること
- `avatar.expression` イベントで表情が切り替わること
- `tts.end` による再生中、口開閉が追従すること

### 9.2 画面での確認手順

1. firmware を起動し、顔描画が表示されることを確認します。
2. サーバー接続完了後、顔が維持されることを確認します。
3. サーバーから `avatar.expression` を送信し、表情が変わることを確認します。
4. 音声再生を発火させ、口パクが動くことを確認します。

`avatar.expression` 送信例:

```json
{"type":"avatar.expression","timestamp":"2026-03-15T11:30:00Z","session_id":"<session_id>","sequence":120,"version":"1.0","payload":{"request_id":"stream-002","expression":"happy","intensity":0.9}}
```

### 9.3 注意点

- `surprised` は m5stack-avatar の都合で `Doubt` 表情へマッピングされます。
- モーション演出（`nod`/`shake`）は顔回転を瞬間的に与え、次周期でニュートラル回転へ戻します。

## 10. P8-09 最小確認（conversation 状態遷移）

### 10.1 期待ログ

`session.cpp` では次の形式で状態遷移ログを出力します。

```text
[Conversation] State: idle -> listening reason=audio capture started
[Conversation] State: listening -> thinking reason=audio.end sent
[Conversation] State: thinking -> speaking reason=tts.end playback started
[Conversation] State: speaking -> idle reason=tts playback finished
```

割り込み時は次のように `interrupted` を経由して `idle` へ戻ります。

```text
[Conversation] State: speaking -> interrupted reason=tts.stop received
[Conversation] State: interrupted -> idle reason=tts.stop applied
```

### 10.2 手動確認シナリオ

1. firmware を起動し、`session.welcome` 受信後に `idle` であることを確認します。
2. 画面タップで音声送信を開始し、`listening -> thinking` のログを確認します。
3. サーバー応答で `tts.end` が到達したとき、`thinking -> speaking` を確認します。
4. 再生完了後に `speaking -> idle` が出ることを確認します。
5. 途中で割り込みイベントを送り、`interrupted -> idle` への遷移を確認します。

### 10.3 割り込みイベント送信例（wscat）

`conversation.cancel`:

```json
{"type":"conversation.cancel","timestamp":"2026-03-18T10:24:31Z","session_id":"<session_id>","sequence":42,"version":"1.0","payload":{"request_id":"req_001","reason":"user_interrupt","source":"touch"}}
```

`tts.stop`:

```json
{"type":"tts.stop","timestamp":"2026-03-18T10:24:32Z","session_id":"<session_id>","sequence":77,"version":"1.0","payload":{"request_id":"req_001","reason":"interrupted","clear_queue":true}}
```

`audio.stream_abort`:

```json
{"type":"audio.stream_abort","timestamp":"2026-03-18T10:24:33Z","session_id":"<session_id>","sequence":43,"version":"1.0","payload":{"stream_id":"stream_001","reason":"interrupted","final_chunk_index":8}}
```
