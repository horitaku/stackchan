# フェーズ 9 タスクリスト（LLM 実装と会話コンテキスト統合）

## 1. このドキュメントの目的

フェーズ 9（LLM 実装）を実行しやすくするために、初期バックログを整理します。
本ドキュメントはフェーズ 8 の引き継ぎ事項（mock LLM から OpenAI への置き換え）を具体タスクへ展開した着手版です。

フェーズ 9 の主眼は以下です：
- OpenAI Chat Completions API による実 LLM 統合
- Session 内の会話履歴（utterances）を context window として管理
- UI から Persona/System Prompt を動的に変更可能にする
- WebUI テスト導線で LLM の健全性を先に検証する

## 2. 実行タスクリスト（初期バックログ）

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P9-01 | OpenAI LLM Provider 実装 | `server/internal/providers/openai/llm.go`、Chat Completions クライアント、parameter 設計、error handling、test | 高 | mock-llm から実装への初期置き換え必須であり、後続タスクの前提 | Done |
| P9-02 | Conversation Context 管理（会話履歴）の実装 | `server/internal/session/conversation_context.go`、直近 N turns 取得、token window 計算、DB 永続化連携 | 高 | LLM が context なしに毎回新規開始するのを改善し、自然な会話流れを実現 | Done |
| P9-03 | LLM Persona / System Prompt UI 設定機能 | `server/internal/web/settings_store.go` 拡張、GET/POST `/api/settings/llm`、WebUI 設定パネル、値の永続化 | 高 | 運用者が persona を動的に切り替え可能にし、デバイス体験を柔軟化 | Done |
| P9-04 | LLM レイテンシ・トークン計測と可視化 | runtime_metrics に token 関連項目追加、`GET /api/runtime/overview` 拡張、WebUI パネル | 中 | 会話品質とコスト（token 消費）を定量監視し、最適化判定を支援 | Done |
| P9-05 | WebUI から OpenAI LLM を使った UI 単体テスト導線 | `POST /api/tests/llm/ui`、テキスト入力、persona 指定、応答テキスト表示、error 表示、手順書 | 高 | 実デバイス非接続でも LLM の健全性を先に切り分け可能に | Done |
| P9-06 | WebUI から Stackchan 連携した LLM テスト導線 | `POST /api/tests/llm/stackchan`、STT → LLM → TTS のフルパイプ、結果確認、手順書 | 高 | 実デバイス連携時の会話フロー全体を早期に検証 | Done |
| P9-07 | LLM 関連 ランブック追加 | `docs/runbooks/llm-config.md`（OpenAI設定、key、quota、error、token limit）、既存ランブック更新 | 中 | LLM 故障時の MTTR 短縮と運用ナレッジの定着 | Done |
| P9-08 | protocol への stt.final payload 拡張（confidence / alternatives） | `protocol/websocket/schemas/stt.final.schema.json` 更新、context hint 用備忘フィールド | 低 | 将来的な LLM disambiguation support への下準備 | Done |

## 2.1 実行方針（フェーズ 9）

### 優先順

1. **P9-01 → P9-02 → P9-03** (順序必須)
   - OpenAI client なしに context 管理は実装できないため、P9-01 を完遂後に P9-02 へ
   - Persona UI（P9-03）は P9-02 の context 機能とセットで検証

2. **P9-05 & P9-06** (並行可)
   - WebUI テスト導線は P9-02 完了後に開始
   - P9-05 と P9-06 は独立実装可

3. **P9-04** (P9-02 完了後)
   - token 計測は LLM context 管理で初めて有効になるため

4. **P9-07 & P9-08** (並行、低優先)

### 設計決定事項

#### 会話 Context 管理（P9-02）

- **履歴保持期間**: Session 内で直近 N turns（5 ターンを初期値）を保持
- **Token 上限**: max_tokens = 2000（content + history + system prompt）
- **構造**:
  ```json
  {
    "system_prompt": "あなたは ...",
    "history": [
      { "role": "user", "content": "..." },
      { "role": "assistant", "content": "..." }
    ]
  }
  ```
- **DB 連携**: utterances テーブルを読み直近 N 件を memory へ再構築
- **スコアリング**: Token 数をシミュレートし、context window 内に収まるまで古いターンを削除

#### Persona / System Prompt 設定（P9-03）

- **保存先**: `system_settings` テーブルの `system_prompt` column
- **デフォルト**: 「Stack-chan です。話しかけてくれてありがとう。」
- **UI**: WebUI の "設定" → "LLM" タブで long text 入力可
- **適用**: 次の会話から反映（既存セッションは影響なし）
- **キャッシング**: ランタイムメモリ（settings_store）でキャッシュ、5 分 TTL

#### LLM テスト導線（P9-05 & P9-06）

- **P9-05 (UI 単体)**:
  - 入力: テキスト + persona override（optional）
  - 出力: LLM 応答テキスト、使用 token、レイテンシ
- **P9-06 (連携)**:
  - 入力: テキスト + persona / speaker / expression
  - 流れ: text → LLM → TTS → Stackchan 送信 → 再生確認
  - 出力: JSON 結果、実際の会話を再現

#### メトリクス（P9-04）

- `llm_input_token_count`: request token 数
- `llm_output_token_count`: response token 数  
- `llm_total_token_count`: request + response
- `llm_latency_ms`: API 呼び出し時間
- `llm_effective_turns_in_context`: 実際に context に含まれた turns

### 互換性・Fallback 方針

- OpenAI API が 5xx を返す場合 → default generic response を返す（「申し訳ありません。今は返答できません」）
- Token limit 超過 → 古いターンを削除して retry（最大 2 回）
- Persona 未設定 → デフォルト prompt を使用

## 2.2 P9-01 との引き継ぎ条件

- `OPENAI_API_KEY` が正しく設定されていることを確認
- OpenAI model name（`OPENAI_MODEL_CHAT`）が有効であることをテストで検証
- Provider 境界インターフェース（P3 設計）に従い、実装すること

## 2.3 P9-02 との引き継ぎ条件

- P9-01 の OpenAI LLMProvider が正常に動作していること
- `utterances` テーブルが存在し、read/write 可能であること（DB migration 既存）
- Token counting library（例：`github.com/tiktoken-go/tokenizer`）を go.mod に追加可能であること

## 2.4 P9-03 との引き継ぎ条件

- P9-02 の context 管理が実装済みであること
- `system_settings` テーブルが存在し、`system_prompt` column が作成済みであること（必要に応じて migration 追加）
- WebUI の Settings 画面スケルトンが存在すること

## 2.5 P9-05 & P9-06 との引き継ぎ条件

- P8-12/P8-13 同様のテスト API パターンが確立していること
- WebUI の "テスト" パネルから `/api/tests/llm/*` エンドポイントへアクセス可能であること

## 3. フェーズ 8 からの引き継ぎ

- OpenAI STT/TTS provider は既に実装済み
- mock LLM は現状で動作するため、段階的に置き換え可
- WebSocket protocol、session state machine、avatar sync は既に安定
- runtime_metrics ストア、WebUI テスト導線の構造も確立済み

## 4. 受け入れ方針（フェーズ 9）

- OpenAI LLM が最低 3 回連続で応答成功すること（API 疎通確認）
- Conversation context が 2 turn 以上正しく保持されることを確認
- Persona UI で入力した prompt が実際に LLM へ送信されることを verifyテストで確認
- WebUI テスト導線から LLM 応答が表示されることを手動確認
- Stackchan 連携テストで「STT 入力 → LLM 応答 → TTS 出力 → firmware 再生」まで動作することを確認

## 5. 今後の拡張候補（Phase 10+）

- Function calling: LLM が motion / expression を指示
- Memory system: Session 外の長期記憶（e.g. user name、preference）
- Fine-tuning: 特定 persona への適応化
- Streaming: ChatGPT streaming で低遅延化
- Vision: デバイスカメラ入力を LLM へ渡す
- Multi-turn interruption: 会話中割り込みでの context 更新

---

本ドキュメントは初版です。実装が進んだら、実際のコード構成・API 設計に合わせて更新してください。
