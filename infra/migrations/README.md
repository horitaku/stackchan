# migrations

PostgreSQL スキーマ変更は SQL マイグレーションで管理します。

## 方針

- すべての変更は `up` / `down` をセットで追加します。
- 破壊的変更は「追加 -> データ移行 -> 削除」の段階移行で実施します。
- 本番適用前にステージング環境で `up` / `down` を検証します。

## ファイル命名

- `NNNNNN_description.up.sql`
- `NNNNNN_description.down.sql`

## ローカル実行例（golang-migrate）

`DATABASE_URL` 例:

- `postgres://stackchan:change-me@localhost:5432/stackchan?sslmode=disable`

適用:

```bash
migrate -path infra/migrations -database "$DATABASE_URL" up
```

巻き戻し:

```bash
migrate -path infra/migrations -database "$DATABASE_URL" down 1
```

状態確認:

```bash
migrate -path infra/migrations -database "$DATABASE_URL" version
```
