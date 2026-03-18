# 障害復旧ランブック: 設定不整合

## 概要

環境変数・secrets ファイル・Docker compose 設定の不整合が原因でサービスが正常に起動・動作しない場合の診断・復旧手順を記載します。

---

## 1. 症状の分類

| 症状 | 原因候補 | 対応セクション |
|---|---|---|
| サーバーが起動するが OpenAI API が機能しない | `OPENAI_API_KEY` 未設定・不正 | [§2 OpenAI 設定の確認](#2-openai-設定の確認) |
| DB 接続エラーでサーバーが起動しない | `DATABASE_URL` 不正、postgres 未起動 | [§3 データベース接続の確認](#3-データベース接続の確認) |
| Voicevox TTS が失敗する | `VOICEVOX_BASE_URL` 不正 | [§4 voicevox-設定の確認](#4-voicevox-設定の確認) |
| セッション認証エラーが頻発する | `SESSION_SECRET` 空・短すぎる | [§5 セッション設定の確認](#5-セッション設定の確認) |
| firmware が接続できない | `FW_WS_URL` 不正、SSID/パスワード誤設定 | [§6 firmware-設定の確認](#6-firmware-設定の確認) |
| CORS エラーで WebUI が機能しない | `CORS_ALLOWED_ORIGINS` 不整合 | [§7 cors-設定の確認](#7-cors-設定の確認) |
| runtime metrics が保存されない | `DATABASE_URL` 未設定 | [§8 metrics-永続化の確認](#8-metrics-永続化の確認) |

---

## 2. OpenAI 設定の確認

### 診断

```bash
# サーバー起動ログで API キー関連のメッセージを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "openai\|api.key"

# パイプラインテストで直接確認
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "テスト"}' | jq '{success: .success, error: .error}'
```

### 必須設定値の確認

```bash
# .env から OpenAI 設定を確認（実際のキー値は表示しない）
grep "OPENAI" .env | sed 's/=.*/=***MASKED***/'

# デフォルト値（要変更！）が残っていないか確認
grep "replace-with" .env
# この出力が空であること（デフォルト値が残っている場合は危険）
```

| 変数名 | 確認方法 | 注意点 |
|---|---|---|
| `OPENAI_API_KEY` | 先頭が `sk-` で始まること | `replace-with-openai-api-key` のままでは動作しない |
| `OPENAI_MODEL_CHAT` | `gpt-4o-mini` 等の有効なモデル名 | 廃止済みモデルは使用不可 |
| `OPENAI_MODEL_STT` | `gpt-4o-mini-transcribe` 等 | STT 対応モデルであること |

### 復旧手順

```bash
# 1. API キーを OpenAI ダッシュボードで確認・再発行
#    https://platform.openai.com/api-keys

# 2. .env を更新
#    OPENAI_API_KEY=sk-proj-xxxxxxxxxxxx（実際のキーに変更）

# 3. サーバーを再起動
mise run infra:down && mise run infra:up

# 4. パイプラインテストで確認
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "確認テスト"}' | jq .
```

---

## 3. データベース接続の確認

サーバーは DB ヘルスチェック後に起動します（`depends_on: db: condition: service_healthy`）。

### 診断

```bash
# DB コンテナのステータスを確認
docker compose -f infra/docker/docker-compose.yml ps db
# STATUS が "healthy" であること

# DB コンテナのログを確認
docker compose -f infra/docker/docker-compose.yml logs db --tail=30

# サーバーログで DB 接続エラーを確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "database\|postgres\|sql"
```

### DATABASE_URL の形式確認

```
postgres://{USER}:{PASSWORD}@{HOST}:{PORT}/{DBNAME}?sslmode=disable

# compose 内での設定例
DATABASE_URL=postgres://stackchan:change-me@db:5432/stackchan?sslmode=disable
#                        ↑ POSTGRES_USER  ↑ POSTGRES_PASSWORD  ↑ compose サービス名  ↑ POSTGRES_DB
```

### 確認事項

| 確認項目 | コマンド |
|---|---|
| `DATABASE_URL` の USER / PASSWORD が postgres 設定と一致しているか | `.env` の `DATABASE_URL` と `POSTGRES_USER` / `POSTGRES_PASSWORD` を比較 |
| HOST が compose サービス名（`db`）になっているか | `DATABASE_URL` のホスト部分を確認 |
| `POSTGRES_PASSWORD` が `change-me` のままでないか | 本番環境では必ず変更する |

### 復旧手順

```bash
# 1. DB コンテナのみ再起動
docker compose -f infra/docker/docker-compose.yml restart db

# 2. ヘルスチェックが通るまで待機
until docker compose -f infra/docker/docker-compose.yml ps db | grep -q "healthy"; do
  echo "Waiting for db to be healthy..."
  sleep 5
done
echo "DB is healthy!"

# 3. 接続確認
docker compose -f infra/docker/docker-compose.yml exec db \
  pg_isready -U stackchan -d stackchan

# 4. マイグレーション状態を確認
#    golang-migrate 使用時:
docker run --rm \
  -v "$(pwd)/infra/migrations:/migrations" \
  --network host \
  migrate/migrate:latest \
  -path=/migrations \
  -database "${DATABASE_URL}" \
  version

# 5. サーバーを再起動
mise run infra:up
```

### マイグレーション失敗時の対処

```bash
# マイグレーション状態の確認
docker run --rm \
  -v "$(pwd)/infra/migrations:/migrations" \
  --network host \
  migrate/migrate:latest \
  -path=/migrations \
  -database "${DATABASE_URL}" \
  version

# dirty フラグが立っている場合（強制リセット — データを保持する）
# 1. dirty フラグを手動でリセット（バージョン番号は現在の番号を指定）
docker run --rm \
  -v "$(pwd)/infra/migrations:/migrations" \
  --network host \
  migrate/migrate:latest \
  -path=/migrations \
  -database "${DATABASE_URL}" \
  force {version_number}

# 2. マイグレーションを再実行
docker run --rm \
  -v "$(pwd)/infra/migrations:/migrations" \
  --network host \
  migrate/migrate:latest \
  -path=/migrations \
  -database "${DATABASE_URL}" \
  up
```

---

## 4. Voicevox 設定の確認

### 診断

```bash
# VOICEVOX_BASE_URL の現在値を確認
curl -fsS http://localhost:8080/api/settings | jq '.voicevox_base_url'

# Voicevox への直接疎通確認
curl -fsS http://localhost:50021/version
# 期待値: "latest" またはバージョン文字列

# Voicevox 単体テストで確認
curl -fsS -X POST http://localhost:8080/api/tests/voicevox/ui \
  -H "Content-Type: application/json" \
  -d '{"text": "テストです", "speaker": 1}' | jq '{success: .success, error: .error}'
```

### 確認事項

| 設定 | compose 内サービス名 | ホストからのアクセス |
|---|---|---|
| コンテナ間の通信（server → voicevox） | `http://voicevox:50021` | — |
| ホストからの直接アクセス | — | `http://localhost:50021` |

```bash
# .env で VOICEVOX_BASE_URL を確認
grep "VOICEVOX_BASE_URL" .env
# compose 内では http://voicevox:50021 を指定すること
```

### 復旧手順

```bash
# 1. Voicevox コンテナを再起動
docker compose -f infra/docker/docker-compose.yml restart voicevox

# 2. 起動完了まで待機（CPU モデルは 30〜60 秒かかることがある）
sleep 30

# 3. 疎通確認
curl -fsS http://localhost:50021/version

# 4. サーバー側の設定 API で URL を更新（必要な場合）
curl -fsS -X PUT http://localhost:8080/api/settings \
  -H "Content-Type: application/json" \
  -d '{"voicevox_base_url": "http://voicevox:50021"}'
```

---

## 5. セッション設定の確認

`SESSION_SECRET` は WebSocket セッションの署名・識別に使用します。

### 診断

```bash
# SESSION_SECRET が設定されているか（値は表示しない）
grep "SESSION_SECRET" .env | sed 's/=.*/=***MASKED***/'

# デフォルト値が残っていないか
grep "replace-with-long-random-secret" .env
# この出力が空であること
```

### 復旧手順

```bash
# 1. 十分な長さのランダム文字列を生成（最低 32 文字以上推奨）
openssl rand -hex 32
# 出力例: a8f3b7e2c1d4...（64 文字の hex 文字列）

# 2. .env に設定
# SESSION_SECRET=a8f3b7e2c1d4...

# 3. サーバーを再起動
mise run infra:down && mise run infra:up
```

> **注意**: `SESSION_SECRET` を変更した場合、既存のすべてのセッションは無効化されます。  
> デバイスは再接続して `session.hello` を再送する必要があります。

---

## 6. Firmware 設定の確認

### 診断ポイント

firmware の設定は `firmware/include/secrets.h` で管理されています。  
シリアルモニターで接続状況を確認します。

```bash
# シリアルモニターで接続ログを確認
mise run fw:monitor

# 期待ログの例（正常時）:
# [INFO] WiFi connected ip=192.168.x.x
# [INFO] WebSocket connected url=ws://192.168.1.10:8080/ws
# [INFO] session.hello sent device_id=stackchan-cores3-01

# 失敗パターン:
# [ERROR] WiFi connect failed: SSID= retries=3         → SSID が空
# [ERROR] WebSocket connect failed: url=ws://... code=404 → URL が不正
```

### 必須設定値の確認

```cpp
// firmware/include/secrets.h で確認すべき設定

// Wi-Fi 設定
FW_WIFI_SSID      // 空でないこと
FW_WIFI_PASSWORD  // ログに出力してはいけない

// サーバー接続設定
FW_WS_URL         // ws:// または wss:// で始まること
                  // compose 環境では ws://{ホストPCのIP}:8080/ws
FW_DEVICE_ID      // 空でないこと（例: "stackchan-cores3-01"）
```

### よくある設定ミスと対処

| 症状 | ミスの内容 | 対処 |
|---|---|---|
| Wi-Fi に繋がらない | `FW_WIFI_SSID` のスペルミス | `secrets.h` を再確認 |
| `ws://localhost:8080/ws` に接続できない | Docker 内サーバーに `localhost` では届かない | ホスト PC の IP アドレスを指定（例: `ws://192.168.1.10:8080/ws`） |
| セッションが確立できない | `FW_DEVICE_ID` が空 | 一意の ID を設定する |
| 再接続ループが止まらない | `FW_WS_TOKEN` が設定されているがサーバー側未対応 | `FW_WS_TOKEN` を空にする |

### ホスト PC の IP アドレス確認方法

```bash
# Linux の場合
ip addr show | grep "inet " | grep -v "127.0.0.1"
# または
hostname -I

# 例: 192.168.1.10 を使う場合の FW_WS_URL
# FW_WS_URL = "ws://192.168.1.10:8080/ws"
```

### 復旧手順

```bash
# 1. secrets.h を修正
#    （エディタで firmware/include/secrets.h を開いて修正）

# 2. firmware をリビルドして再書き込み
mise run fw:upload

# 3. シリアルモニターで接続を確認
mise run fw:monitor
```

---

## 7. CORS 設定の確認

ブラウザから WebUI にアクセスした際に CORS エラーが発生する場合は、`CORS_ALLOWED_ORIGINS` の設定を確認します。

### 診断

```bash
# ブラウザの開発者ツール（F12）→ Console または Network タブで確認
# エラー例:
# Access to fetch at 'http://localhost:8080/api/...' from origin 'http://localhost:5173'
# has been blocked by CORS policy

# 現在の CORS 設定を確認
grep "CORS_ALLOWED_ORIGINS" .env
```

### 復旧手順

```bash
# 開発環境（Svelte dev server: 5173）と Go server（8080）を分離している場合
# CORS_ALLOWED_ORIGINS=http://localhost:5173

# 本番環境（Go サーバーが WebUI も静的配信している場合は CORS 不要）
# CORS_ALLOWED_ORIGINS=  ← 空でも同一オリジンなら問題なし

# 複数オリジンを許可する場合（カンマ区切り）
# CORS_ALLOWED_ORIGINS=http://localhost:5173,http://192.168.1.10:8080

# .env 変更後にサーバー再起動
mise run infra:down && mise run infra:up
```

---

## 8. Metrics 永続化の確認

`DATABASE_URL` が設定されていない場合、runtime metrics の永続化が無効になります（サーバー自体は起動します）。

### 診断

```bash
# 警告ログが出ていないか確認
docker compose -f infra/docker/docker-compose.yml logs stackchan-server | grep -i "metrics\|disabled"
# 出力例（永続化無効時）:
# [WARN] runtime metrics store is disabled: DATABASE_URL not set

# GET /api/runtime/metrics を叩いて確認
curl -fsS "http://localhost:8080/api/runtime/metrics?limit=5" | jq .
```

### 復旧手順

```bash
# 1. DATABASE_URL が .env に設定されているか確認
grep "DATABASE_URL" .env

# 2. DB コンテナが起動しているか確認
docker compose -f infra/docker/docker-compose.yml ps db

# 3. 接続文字列を確認
#    DATABASE_URL=postgres://stackchan:change-me@db:5432/stackchan?sslmode=disable

# 4. サーバーを再起動
mise run infra:down && mise run infra:up

# 5. metrics が保存されるかパイプラインテストで確認
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "テスト"}' | jq .

# 6. metrics が保存されているか確認
curl -fsS "http://localhost:8080/api/runtime/metrics?limit=5" | jq '.metrics | length'
# 0 より大きい値が返れば永続化されている
```

---

## 9. 設定チェックリスト（起動前確認）

サービスを初回起動する前、または設定変更後に確認するリストです。

```bash
# 以下を順番に確認する

echo "=== .env の必須項目確認 ==="
# デフォルト値（replace-with-...）が残っていないか
grep -n "replace-with\|change-me" .env && echo "⚠ 要変更項目あり" || echo "✓ デフォルト値なし"

echo ""
echo "=== サービス起動確認 ==="
mise run infra:ps 2>/dev/null || docker compose -f infra/docker/docker-compose.yml ps

echo ""
echo "=== ヘルスチェック ==="
curl -fsS http://localhost:8080/healthz && echo "" || echo "⚠ サーバー未起動"
curl -fsS http://localhost:50021/version && echo "" || echo "⚠ Voicevox 未起動"

echo ""
echo "=== パイプラインテスト ==="
curl -fsS -X POST http://localhost:8080/api/tests/pipeline \
  -H "Content-Type: application/json" \
  -d '{"text": "確認テスト"}' | jq '{success: .success, error: .error}'
```

---

## 10. エスカレーション基準

| 状況 | 対応 |
|---|---|
| `OPENAI_API_KEY` が正しいのに認証エラーが続く | OpenAI サポートへ問い合わせ |
| DB が healthy にならない | Docker のリソース制限（メモリ/ディスク）を確認 |
| マイグレーションの `dirty` フラグが解消しない | `infra/migrations/` のファイル整合性を確認 |
| firmware の `FW_WS_URL` を正しく設定しても接続できない | ファイアウォール・ルーターのポート設定を確認 |
| 設定変更後もサーバーに反映されない | コンテナのボリュームキャッシュを確認（`docker compose down -v` で強制クリア） |
