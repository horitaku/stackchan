# infra

Docker やデプロイ設定を管理します。
ローカル・検証・本番の再現性を高めるため、ランタイム設定を文書化します。

## ディレクトリ

- `docker/`
  - `Dockerfile`: WebUI build -> Go build -> 最小 runtime の 3 ステージ定義
  - `docker-compose.yml`: server + PostgreSQL のローカル統合起動定義
- `migrations/`
  - SQL マイグレーション定義と運用手順

## ローカル統合起動

`infra/docker` で次を実行します。

```bash
docker compose up --build
```

主要エンドポイント:

- `http://localhost:8080/healthz`
- `http://localhost:8080/ui/`

停止:

```bash
docker compose down
```

DB データも削除する場合:

```bash
docker compose down -v
```
