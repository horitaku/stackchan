# 障害復旧ランブック — インデックス

Stackchan サーバーおよびファームウェアで発生する主要な障害シナリオの診断・復旧手順をまとめたランブック群です。  
運用時の MTTR（平均復旧時間）を短縮するために活用してください。

---

## ランブック一覧

| ランブック | 主なシナリオ |
|---|---|
| [接続断](connection-failure.md) | Wi-Fi 断、WebSocket 接続失敗、ハンドシェイク失敗、タイムアウト切断、サーバー再起動後の復旧 |
| [Provider 遅延・タイムアウト](provider-latency.md) | STT/LLM/TTS のタイムアウト、Voicevox 停止、レート制限、リトライ設定調整 |
| [LLM 設定・障害対応](llm-config.md) | OpenAI 設定、persona/system prompt、token 監視、quota/5xx/429 切り分け |
| [設定不整合](configuration-mismatch.md) | 環境変数ミス、DB 接続エラー、API キー不正、firmware 設定ミス、CORS エラー、metrics 永続化無効 |
| [Hardware Overview 診断](hardware-overview.md) | device.state.report の可視化、最終更新時刻、未接続時の切り分け、hardware dispatch ログ確認 |

---

## 最初に確認すること（クイックチェック）

```bash
# 1. 全サービスの起動状態
mise run infra:ps
# または
docker compose -f infra/docker/docker-compose.yml ps

# 2. サーバーのヘルスチェック
curl -fsS http://localhost:8080/healthz
# 期待値: {"status":"ok"}

# 3. Voicevox の疎通確認
curl -fsS http://localhost:50021/version
# 期待値: "latest"

# 4. ランタイム状態の概要
curl -fsS http://localhost:8080/api/runtime/overview | jq .

# 5. デフォルト値が .env に残っていないか
grep -n "replace-with\|change-me" .env && echo "⚠ 要変更あり" || echo "✓ OK"
```

---

## ログの確認方法

```bash
# 全サービスのログを表示
docker compose -f infra/docker/docker-compose.yml logs

# サービスを指定してリアルタイムで確認
docker compose -f infra/docker/docker-compose.yml logs -f stackchan-server
docker compose -f infra/docker/docker-compose.yml logs -f db
docker compose -f infra/docker/docker-compose.yml logs -f voicevox

# firmware のシリアルログ（USB 接続時）
mise run fw:monitor
```

---

## 主要なエンドポイット早見表

| エンドポイント | 用途 |
|---|---|
| `GET /healthz` | サーバー生死確認 |
| `GET /api/runtime/overview` | 接続状態・最新メトリクスのスナップショット |
| `GET /api/runtime/metrics` | メトリクス履歴（`?metric_name=&limit=&from=&to=`） |
| `GET /api/settings` | 現在の設定値確認 |
| `PUT /api/settings` | 設定値の動的更新 |
| `GET /api/settings/llm` | LLM system prompt の取得 |
| `POST /api/settings/llm` | LLM system prompt の更新 |
| `POST /api/tests/pipeline` | STT/LLM/TTS パイプライン疎通テスト |
| `POST /api/tests/llm/ui` | OpenAI LLM 単体テスト |
| `POST /api/tests/llm/stackchan` | LLM + Voicevox + Stackchan 連携テスト |
| `POST /api/tests/voicevox/ui` | Voicevox 単体 TTS テスト |
| `POST /api/tests/voicevox/stackchan` | Voicevox + Stackchan 連携テスト |
| `GET /api/tests/hardware/state` | device.state.report 要求送信（Hardware Overview 更新トリガー） |

---

## 関連ドキュメント

- [システム概要](../architecture/system-overview.md)
- [会話状態マシン](../architecture/conversation-state-machine.md)
- [音声トランスポート](../architecture/audio-transport.md)
- [secrets 運用](../project/secrets-operations.md)
- [インフラ/マイグレーション手順](../../infra/migrations/README.md)
