# 障害復旧ランブック: 接続断

## 概要

デバイス（M5Stack CoreS3）とバックエンドサーバー間の WebSocket 接続が切断された場合の診断・復旧手順を記載します。

---

## 1. 症状の分類

| 症状 | 原因候補 | 対応セクション |
|---|---|---|
| デバイスが Wi-Fi に繋がらない | SSID/パスワード設定ミス、AP 障害 | [§2 Wi-Fi 接続断](#2-wi-fi-接続断) |
| WebSocket が `upgrade` で失敗する | サーバー未起動、URL 設定ミス、CORS | [§3 WebSocket 接続失敗](#3-websocket-接続失敗) |
| `session.hello` 後に接続が切れる | ハンドシェイク不備、device_id 未設定 | [§4 ハンドシェイク失敗](#4-ハンドシェイク失敗) |
| 会話途中で突然切れる | タイムアウト（heartbeat 未送信）、AP 干渉 | [§5 セッション中の切断](#5-セッション中の切断) |
| サーバー再起動後に復帰しない | セッション未クリア、firmware の再接続ロジック | [§6 サーバー再起動後の復旧](#6-サーバー再起動後の復旧) |

---

## 2. Wi-Fi 接続断

### 診断

```bash
# デバイスのシリアルログを確認（USB 接続時）
# pio device monitor --baud 115200

# 出力例（接続失敗）
# [WARN] WiFi connect failed: SSID=MyHomeWiFi retries=3
# [ERROR] WiFi not connected, abort WS connection
```

### 確認事項

1. `firmware/include/secrets.h` の `FW_WIFI_SSID` / `FW_WIFI_PASSWORD` が正しいか確認する。
2. AP の 2.4GHz 帯が有効か、および 5GHz のみ設定になっていないか確認する。
3. AP の MACアドレスフィルタリングが有効な場合、デバイスの MAC を許可リストに追加する。

```cpp
// firmware/include/secrets.h の注意点
// FW_WIFI_SSID には SSID の文字列をダブルクォートで指定
// FW_WIFI_PASSWORD には パスワードをダブルクォートで指定
// 日本語・マルチバイト文字を含む SSID は未対応
```

### 復旧手順

```bash
# 1. secrets.h を修正してリビルド
#    （platformio.ini.local から参照される secrets.h を更新）
# 2. firmware を再書き込み
mise run fw:upload

# 3. シリアルモニターで接続確認
mise run fw:monitor
# 期待ログ: [INFO] WiFi connected ip=192.168.x.x
```

---

## 3. WebSocket 接続失敗

### 診断

```bash
# サーバーのヘルスチェックで起動確認
curl -fsS http://localhost:8080/healthz
# 期待値: {"status":"ok"}

# WebSocket エンドポイントの疎通確認（wscat または websocat）
wscat -c ws://localhost:8080/ws
# 正常時: Connected (press CTRL+C to quit)
```

### サーバー側ログ確認

```bash
# Docker compose 環境の場合
mise run infra:logs
# または
docker compose -f infra/docker/docker-compose.yml logs stackchan-server

# 確認すべきログパターン
# [INFO] starting stackchan server addr=:8080   → 起動成功
# [ERROR] failed to bind address ...            → ポート競合
```

### 確認事項

| 確認項目 | コマンド / 設定 |
|---|---|
| サーバーが起動しているか | `mise run infra:ps` または `docker ps` |
| ポート 8080 が LISTEN しているか | `ss -tlnp | grep 8080` |
| `FW_WS_URL` が正しいか | `firmware/include/secrets.h` の `FW_WS_URL` |
| サーバーの `CORS_ALLOWED_ORIGINS` | `.env` の設定値 |

### 復旧手順

```bash
# サーバーが停止している場合
mise run infra:up
# 起動を待ってから再確認
sleep 5 && curl -fsS http://localhost:8080/healthz

# ポート競合がある場合（8080 を別プロセスが使用）
lsof -i :8080
# 競合プロセスを終了するか SERVER_ADDR をずらす
```

---

## 4. ハンドシェイク失敗

`session.hello` 送信後に即座に切断される場合、サーバー側でハンドシェイクバリデーションエラーが発生しています。

### 診断

```bash
# サーバーログで Fatal メッセージを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "handshake\|fatal\|hello"

# 期待されるエラーログの例
# [ERROR] handshake validation failed error="device_id is required"
# [ERROR] handshake validation failed error="unsupported client_type: unknown"
```

### 確認事項（ハンドシェイク必須フィールド）

| フィールド | 期待値 | バリデーション |
|---|---|---|
| `device_id` | 空でない文字列（例: `stackchan-cores3-01`） | 必須 |
| `client_type` | `firmware` または `test_harness` | 列挙型チェック |
| `protocol_version` | `1` | バージョンチェック |

```cpp
// firmware 側の確認ポイント（firmware/app/stackchan/session.cpp 付近）
// session.hello の payload に device_id を含んでいるか確認する
```

### 復旧手順

```bash
# firmware の secrets.h を確認
grep "FW_DEVICE_ID" firmware/include/secrets.h
# 値が空になっていないか確認（例: "stackchan-cores3-01"）

# firmware をリビルドして再書き込み
mise run fw:upload
```

---

## 5. セッション中の切断

### heartbeat タイムアウトによる切断

サーバーは `WS_READ_TIMEOUT`（デフォルト 45 秒）以内に受信がない場合、接続を切断します。デバイスは `session.welcome` で受け取る `heartbeat_interval_ms`（15,000ms）に従って定期送信する必要があります。

```bash
# サーバーログで unexpected close を確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep "unexpected\|timeout"
# 出力例:
# [WARN] unexpected WebSocket close session_id=xxxx error="read tcp: i/o timeout"
```

### AP 干渉・電波品質による切断

```bash
# デバイス側のシリアルログで RSSI や再接続回数を確認
mise run fw:monitor
# 期待ログ（再接続成功）:
# [INFO] WebSocket reconnected attempt=2 delay_ms=1000

# runtime overview で再接続カウントを確認
curl -fsS http://localhost:8080/api/runtime/overview | jq .
```

### 確認事項

| 確認項目 | 参照先 |
|---|---|
| heartbeat が定期送信されているか | firmware のシリアルログ |
| `reconnect_count` が急増していないか | `GET /api/runtime/overview` |
| サーバーとデバイスが同一ネットワーク内か | ルーター設定 |
| AP の送信電力設定が低すぎないか | AP の管理画面 |

### 復旧手順

```bash
# 1. サーバー側の状態を確認
curl -fsS http://localhost:8080/api/runtime/overview | jq '{
  connected: .session.connected,
  reconnect_count: .connection.reconnect_count,
  ws_read_timeout: .config.ws_read_timeout_sec
}'

# 2. タイムアウト値を延長して様子を見る（応急処置）
#    .env の WS_READ_TIMEOUT を 60 → 90 に変更
#    サーバーを再起動
mise run infra:down && mise run infra:up

# 3. firmware の heartbeat 送信間隔を確認（15,000ms 以内か）
```

---

## 6. サーバー再起動後の復旧

サーバーを再起動した場合、既存のセッションはすべてクリアされます。デバイス側は自動的に再接続して `session.hello` を再送する必要があります。

### 診断

```bash
# デバイスが再接続しているか確認
mise run fw:monitor
# 期待ログ:
# [INFO] WebSocket reconnected attempt=1 delay_ms=500
# [INFO] session.hello sent device_id=stackchan-cores3-01

# サーバーが新規セッションを受け付けているか確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep "connection established"
```

### 復旧手順

```bash
# 1. サーバーを正常起動
mise run infra:up

# 2. DB ヘルスチェックで postgres が ready か確認
docker compose -f infra/docker/docker-compose.yml ps db
# STATUS が healthy になっていること

# 3. サーバーヘルスチェック
curl -fsS http://localhost:8080/healthz

# 4. デバイスが自動再接続しない場合はデバイスを再起動
#    （firmware の再接続ロジックが指数バックオフで最大 FW_RECONNECT_MAX_MS まで待機する）
```

---

## 7. エスカレーション基準

以下の条件を満たす場合は、コードやインフラ設定の変更が必要です。

| 状況 | 対応 |
|---|---|
| `reconnect_count` が 1 時間で 10 回以上 | AP 環境または firmware の再接続ロジックを調査 |
| heartbeat タイムアウトが毎回発生 | firmware の heartbeat 実装を確認 |
| `session.hello` の `device_id` が空になる | `secrets.h` のビルド設定を確認 |
| サーバー再起動後もデバイスが復帰しない | firmware の再接続ループを確認 |
