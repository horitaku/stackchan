# LLM 設定・障害対応ランブック

OpenAI LLM 統合（Phase 9）に関する設定、疎通確認、障害切り分け、復旧手順をまとめます。

## 1. 事前確認

- `OPENAI_API_KEY` が設定済みであること
- `OPENAI_MODEL_CHAT` が有効モデルであること（例: `gpt-4o-mini`）
- `OPENAI_HTTP_TIMEOUT_SEC` が 30〜60 秒程度であること
- `DATABASE_URL` が設定され、`system_settings` / `utterances` へ書き込み可能であること

確認コマンド:

```bash
# サーバー環境変数（キー値本体は表示しない）
docker compose -f infra/docker/docker-compose.yml exec stackchan-server \
  sh -lc 'env | grep -E "OPENAI_MODEL_CHAT|OPENAI_HTTP_TIMEOUT_SEC|DATABASE_URL|OPENAI_API_KEY" | sed "s/OPENAI_API_KEY=.*/OPENAI_API_KEY=***MASKED***/"'

# LLM 設定 API
curl -fsS http://localhost:8080/api/settings/llm | jq .
```

## 2. LLM 単体疎通（WebUI なし）

```bash
curl -fsS -X POST http://localhost:8080/api/tests/llm/ui \
  -H 'Content-Type: application/json' \
  -d '{"text":"こんにちは、自己紹介して"}' | jq .
```

成功時の確認ポイント:

- `reply_text` が空でない
- `llm_latency_ms` が記録される
- `llm_input_token_count` / `llm_output_token_count` / `llm_total_token_count` が返る

## 3. Stackchan 連携疎通

```bash
curl -fsS -X POST http://localhost:8080/api/tests/llm/stackchan \
  -H 'Content-Type: application/json' \
  -d '{
    "text":"今日の気分をひとことで",
    "speaker":1,
    "expression":"happy",
    "motion":"nod",
    "chunk_version":"1.1"
  }' | jq .
```

成功時の確認ポイント:

- `active_stackchan_session_id` が返る
- LLM 応答テキストが `reply_text` に入る
- firmware 側で音声再生される

## 4. Persona / System Prompt 更新

```bash
# 現在値
curl -fsS http://localhost:8080/api/settings/llm | jq .

# 更新
curl -fsS -X POST http://localhost:8080/api/settings/llm \
  -H 'Content-Type: application/json' \
  -d '{"system_prompt":"あなたは丁寧で短く答えるアシスタントです。"}' | jq .
```

注意点:

- 反映は次回会話ターンから
- DB 利用時は `system_settings(category=llm,key=system_prompt)` に永続化

## 5. 代表的な障害と復旧

### 5.1 401 / 403（認証失敗）

症状:

- `/api/tests/llm/ui` が 502 で失敗
- ログに `openai authentication failed`

対処:

1. `OPENAI_API_KEY` の期限・権限を確認
2. キーのローテーション後にサーバー再起動
3. 再度 LLM 単体疎通を実行

### 5.2 429（レート制限）

症状:

- 応答遅延増大
- `openai rate limited` ログ

対処:

1. `OPENAI_MODEL_CHAT` を軽量モデルへ切り替え
2. 同時リクエストを抑制
3. クォータ上限・課金状態を確認

### 5.3 5xx（上流不安定）

症状:

- `openai upstream error` ログ
- 応答がフォールバック文言になる

対処:

1. OpenAI ステータス確認
2. 10〜30 秒待って再試行
3. 長時間継続時は運用モードを mock LLM へ一時退避

### 5.4 token 上限超過

症状:

- 長会話で応答失敗または遅延増大

対処:

1. `LLM_CONTEXT_MAX_TOKENS` を調整（既定 2000）
2. `LLM_CONTEXT_MAX_TURNS` を縮小（既定 5）
3. Persona を短く保つ

## 6. 観測項目

`GET /api/runtime/overview` の `pipeline` で以下を監視:

- `llm_latency_ms`
- `llm_input_token_count`
- `llm_output_token_count`
- `llm_total_token_count`
- `llm_effective_turns_in_context`

履歴確認:

```bash
curl -fsS "http://localhost:8080/api/runtime/metrics?metric_name=llm_total_token_count&limit=20" | jq .
```

## 7. 既定値

- system prompt: `Stack-chan です。話しかけてくれてありがとう。`
- model: `gpt-4o-mini`
- context turns: `5`
- context max tokens: `2000`
