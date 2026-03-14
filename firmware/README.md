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
; このファイルは git 管理外です。platformio.ini.example をコピーして作成してください。
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

### 3.0 PlatformIO 実行方式（Windows）

- このリポジトリでは、Windows 環境の標準コマンドを `py -m platformio` とします。
- 理由: `pip install --user platformio` で導入した場合、`pio.exe` の配置先が PATH に含まれず、`pio` コマンドが未認識になることがあるためです。
- `pio` を使いたい場合は、以下をユーザー PATH へ追加して新しいシェルを開いてください。
	- `C:\Users\<your-user>\AppData\Roaming\Python\Python313\Scripts`

> 補足: 再起動だけでは PATH は増えません。PATH を追加した場合のみ、新しいシェルで `pio` が使えるようになります。

```bash
# ビルドのみ
py -m platformio run -e stackchan_cores3

# ビルド + フラッシュ（ポートは自動検出、または -e m5stack-cores3 で明示）
py -m platformio run -e stackchan_cores3 --target upload

# シリアルモニター（ボーレート 115200）
py -m platformio device monitor --baud 115200
```

## 4. 起動後の疎通確認

### 4.1 シリアルモニターで確認すること

正常起動時のログ出力順序:

```
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

```
binary stream registered stream_id=... codec=pcm sample_rate_hz=16000
audio stream consumed, starting orchestration
```

## 5. ディレクトリ構成

```
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
|------|---------|
| Wi-Fi 接続が失敗する | `FW_WIFI_SSID` / `FW_WIFI_PASSWORD` が正しいか確認。2.4GHz のみ対応 |
| WebSocket 接続が失敗する | `FW_WS_URL` のアドレスがサーバーの IP と一致しているか確認。サーバーが起動しているか確認 |
| `session.welcome` が返ってこない | サーバーのログを確認。`session.hello` が到達しているか確認 |
| タッチしても音声送信ログが出ない | シリアルモニターで `State != Active` が出ていないか確認 |
| ビルドエラー: `secrets.h not found` | `include/secrets.h` を作成したか確認（`secrets.example.h` をコピー） |
| `platformio.ini.local not found` の警告 | 警告は無視可能。ただし Wi-Fi/WS の設定値が反映されないため要確認 |
| `self-signed certificate in certificate chain` / `CERTIFICATE_VERIFY_FAILED` | 企業プロキシ配下の TLS 検証エラー。対処手順は [docs/project/secrets-operations.md](docs/project/secrets-operations.md) の「企業プロキシ環境での TLS エラー対応（PlatformIO）」を参照 |
| `pio` が見つからない（`command not found`） | Windows では `py -m platformio` を使う。`pio` を使う場合は Python Scripts の PATH 追加が必要 |

## 7. Phase 6 に向けた注意事項

- 現フェーズ（Phase 5）の音声は**ダミー PCM（無音）**です。実マイク収音は Phase 6 で実装します
- `firmware/runtime/audio/mic_reader.cpp` の `readFrame()` に `M5.Mic.record()` を追加する予定です
- Opus エンコードは Phase 6 で導入します。フレームフォーマットは変わりません
