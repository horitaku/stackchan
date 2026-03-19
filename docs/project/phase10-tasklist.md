# フェーズ 10 タスクリスト（識別とメモリー運用基盤）

## 1. このドキュメントの目的

フェーズ 10 を、複数 Stackchan が 1 つのサーバーに同時接続する前提で実行可能なタスクへ分解します。
本フェーズは「識別の一貫性」と「セッション外記憶の安全運用」を主眼とし、設計だけでなく検証・運用まで含めて完了条件を定義します。

フェーズ 10 の主眼は以下です：
- 識別キーの責務分離を固定化する（session_id / device_id / request_id）
- 5 種類の記憶タイプを設計し、段階的に構築する
- Memory Orchestrator を中核に、記憶の取得・更新・要約フローを実装する
- 記憶品質と漏えいリスクを評価できる検証導線を整備する

## 2. 識別・メモリー方針（フェーズ 10 で固定する前提）

### 2.1 識別子の責務

| 識別子 | ライフタイム | 主な用途 |
| --- | --- | --- |
| session_id | WebSocket 接続単位（再接続で再発行） | ライブ接続管理、短期記憶スコープ |
| device_id | firmware 個体（半永久） | 再接続追跡、長期記憶の紐づけ主軸 |
| request_id | 1 会話ターン | STT/LLM/TTS/metrics の相関突合 |
| user_id | 運用単位（device_id と 1:1 または N:1 にできる） | 記憶スコープの論理単位 |
| persona_id | ペルソナ設定ごと | 記憶の混線防止スコープ |

- 現フェーズでは user_id = device_id として 1:1 運用から始める。
- 家族複数運用を将来想定する場合は user_id を独立させる。

### 2.2 記憶タイプ（5 種類）

記憶はすべて 1 つのテーブルに押し込まず、タイプ別に責務を分ける。

| タイプ | 用途 | 保存先 | ライフタイム |
| --- | --- | --- | --- |
| Session Memory（短期） | 直近会話の文脈維持、代名詞解決 | インメモリ または DB の直近 N 件 | セッション内のみ |
| Episodic Memory（出来事） | 「いつ・誰が・何をしたか」の時系列記録 | memories テーブル（type=episode） | 長期、TTL 設定可 |
| Semantic Memory（事実） | 安定した事実・好み・設定（key-value） | memory_facts テーブル | 長期、upsert で更新 |
| Profile Memory（人格） | AI の応答方針・ペルソナ・口調スタイル | profiles テーブル | 半永久（明示変更のみ更新） |
| Reflection Memory（要約） | 会話の圧縮ノート・傾向の抽象化 | memories テーブル（type=reflection） | 長期、要約のたびに更新 |

### 2.3 メモリスコープキー

```
memory_scope_key = user_id:persona_id
```

将来は `user_id` に `device_id` や家族スコープを追加して拡張できる形にする。

### 2.4 取得・更新ポリシー（cache-aside）

- 取得パス: cache hit → そのまま返す / cache miss → DB 取得後、cache に格納して返す
- 更新パス: DB 更新 → cache invalidate（この順を破らない）
- 機密データは cache 不可、または暗号化 + 短 TTL

### 2.5 安全性・整合性

- 受信識別子（device_id 等）を無条件に信用しない（server-side mapping で検証）
- immutable な内部 ID を優先し、可変属性（表示名等）を主キーにしない
- 同一 device_id の二重接続時ポリシーを明示する（reject または旧接続切断）

## 3. 実装アーキテクチャ設計

### 3.1 ディレクトリ構成（追加・整理分）

```
server/internal/
  memory/
    orchestrator.go      ← 中核：context 構築 + post-process を担う
    extractor.go         ← 記憶候補の抽出（ルールベース + LLM 判定）
    retriever.go         ← スコアリングによる記憶取得
    summarizer.go        ← セッション要約・reflection 生成
    scorer.go            ← importance / confidence / recency の重み計算
  prompt/
    builder.go           ← ContextBundle → LLM プロンプト変換
    templates.go         ← プロンプトテンプレート管理
  session/
    （既存）conversation_context.go   ← 短期記憶（Session Memory）
    memory_repository.go              ← memories テーブルアクセス
    fact_repository.go                ← memory_facts テーブルアクセス
    profile_repository.go             ← profiles テーブルアクセス
```

### 3.2 Memory Orchestrator インターフェース

```go
// Orchestrator は会話の文脈構築と記憶更新の中核を担います。
type Orchestrator interface {
    // BuildContext は LLM 呼び出し前に必要な記憶・プロファイル・会話履歴を集約します。
    BuildContext(ctx context.Context, userID, sessionID, input string) (*ContextBundle, error)
    // PostProcess は LLM 応答後に記憶候補を抽出・保存し、必要に応じてセッションを要約します。
    PostProcess(ctx context.Context, userID, sessionID, userInput, assistantOutput string) error
}

// ContextBundle は Prompt Builder へ渡す構築済み文脈です。
type ContextBundle struct {
    Profile          *domain.Profile
    RecentMessages   []providers.LLMMessage   // Session Memory
    RelevantMemories []domain.Memory           // Episodic / Reflection
    Facts            []domain.MemoryFact       // Semantic Memory
    SessionSummary   string
}
```

### 3.3 Prompt 構造（LLM へ渡す整形済み形式）

```
[system]
あなたは Stack-chan として会話する家庭向け AI です。

[profile]
- 表示名: xxx
- 技術レベル: advanced
- 子ども向け説明: enabled

[stable facts]
- 好きな食べ物 = ラーメン
- ...

[relevant memories]
- 前回、音声チャンクの途切れ対策を重視していた

[session summary]
- 今日は Memory サーバーの設計について話している

[recent messages]
user: ...
assistant: ...

[user input]
今日もよろしく
```

### 3.4 記憶スコアリング（Retriever）

```
total_score =
  0.50 * keyword_match     ← content / summary との一致度
  0.20 * recency_score     ← 直近 7 日は加点
  0.20 * importance        ← 書き込み時に付与した重要度（0-1）
  0.10 * confidence        ← 書き込み時に付与した確信度（0-1）
```

将来は `0.50 * vector_similarity` に置き換える（Qdrant 導入時）。

### 3.5 記憶候補の判定（Extractor）

保存する記憶：
- 長期的に意味がある事実（好み・家族情報・プロジェクト状況）
- 繰り返し出るトピック
- 「覚えて」「好き」「嫌い」「いつも」「作っている」などのシグナルワード

保存しない記憶：
- 単なるあいづち、一時的な指示、使い捨て雑談

```go
// MemoryCandidate は PostProcess で抽出された記憶保存候補です。
type MemoryCandidate struct {
    Type       string  // "semantic_fact" | "episode" | "reflection"
    Category   string  // "preference" | "family" | "project" | "schedule"
    Content    string
    Summary    string
    Importance float64
    Confidence float64
    ShouldSave bool
}
```

### 3.6 セッション要約戦略

要約トリガー：
- 20 メッセージを超えたとき
- 一定トークンを超えたとき
- セッション終了時

要約内容：
- 何について話したか
- 決まったこと・続きで必要なこと
- 長期記憶化候補

保存先：
- sessions の last_summary（検索用）
- memories テーブルの type=reflection（長期圧縮）

## 4. データモデル設計

### 4.1 ドメインモデル

```go
// Memory は記憶本体を表します。episode / reflection 両方に使います。
type Memory struct {
    ID          string
    UserID      string
    SessionID   *string    // episodic は紐付け有り、reflection は任意
    Type        MemoryType // episode | semantic | profile | reflection
    Scope       string     // user | family | device | global
    Category    string     // preference | project | family | schedule | personality
    Content     string
    Summary     string
    Importance  float64
    Confidence  float64
    Source      string     // conversation | imported | system
    LastUsedAt  *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// MemoryFact は Semantic Memory を key-value で構造化したテーブルです。
type MemoryFact struct {
    ID         string
    UserID     string
    Key        string     // 例: favorite_food / child_1_interest / project_current
    Value      string
    Scope      string     // user | family | child:1 | device:xxx
    Confidence float64
    Source     string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// Profile は AI の応答方針・ペルソナを管理します。
type Profile struct {
    UserID            string
    DisplayName       string
    DefaultLanguage   string
    TechnicalLevel    string // beginner | intermediate | advanced
    ChildFriendlyMode bool
    ResponseVerbosity string // short | normal | verbose
    PersonaPrompt     string // system prompt ベース
    UpdatedAt         time.Time
}
```

### 4.2 DB スキーマ追加（PostgreSQL migration）

```sql
-- memories テーブル（episodic / reflection / semantic）
CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    session_id TEXT,
    type TEXT NOT NULL CHECK (type IN ('episode', 'semantic', 'profile', 'reflection')),
    scope TEXT NOT NULL DEFAULT 'user',
    category TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    importance DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    source TEXT NOT NULL DEFAULT 'conversation',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_memories_user_type ON memories (user_id, type);
CREATE INDEX idx_memories_user_category ON memories (user_id, category);
CREATE INDEX idx_memories_importance ON memories (user_id, importance DESC);
CREATE INDEX idx_memories_created_at ON memories (user_id, created_at DESC);

-- memory_facts テーブル（Semantic Memory の key-value 構造）
CREATE TABLE memory_facts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'user',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    source TEXT NOT NULL DEFAULT 'conversation',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_memory_facts_unique ON memory_facts (user_id, key, scope)
    WHERE deleted_at IS NULL;

-- profiles テーブル（Profile Memory / ペルソナ設定）
CREATE TABLE profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    default_language TEXT NOT NULL DEFAULT 'ja',
    technical_level TEXT NOT NULL DEFAULT 'intermediate',
    child_friendly_mode BOOLEAN NOT NULL DEFAULT FALSE,
    response_verbosity TEXT NOT NULL DEFAULT 'normal',
    persona_prompt TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- sessions テーブルへの device_id 追加（既存 migration への追記）
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS device_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_summary TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_sessions_device_id ON sessions (device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);
```

## 5. 実行タスクリスト（初期バックログ）

段階的な実装順序を守り、初期から全量実装しない。

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P10-01 | 識別キー責務の仕様固定 | protocol/events.md 追記、server 設計メモ、運用ルール | 高 | 複数台接続時の混線防止の最上位要件 | Todo |
| P10-02 | sessions に device_id / user_id を追加 | migration 追加、handshake 更新、既存保存処理改修 | 高 | 再接続をまたぐ個体追跡に必須 | Todo |
| P10-03 | Profile Memory スキーマと API | profiles テーブル、GET/PUT /api/memory/profile、UI | 高 | ペルソナの一貫性はユーザー体験の基盤 | Todo |
| P10-04 | Semantic Memory（memory_facts）スキーマと初期抽出 | memory_facts テーブル、FactRepository、Extractor ルールベース | 高 | 「覚えてる感」を最も効率よく実現できる | Todo |
| P10-05 | Memory Orchestrator の骨組み | orchestrator.go、ContextBundle、Prompt Builder | 高 | 記憶取得・LLM 呼び出しの中核統合 | Todo |
| P10-06 | Session 要約機能 | Summarizer、sessions.last_summary 更新、トリガー設計 | 中 | 長い会話でのコンテキスト圧縮に必要 | Todo |
| P10-07 | Episodic Memory スキーマと保存 | memories テーブル（type=episode）、PostProcess 連携 | 中 | 出来事記憶で会話の継続性が大きく向上 | Todo |
| P10-08 | 記憶 Retriever の実装 | retriever.go、scorer.go、スコアリングロジック | 中 | Orchestrator の BuildContext 品質を決める | Todo |
| P10-09 | 同時接続制御と認可検証 | 同一 device_id 接続ポリシー（reject/kick）、エラーイベント設計 | 高 | なりすましと競合接続対策 | Todo |
| P10-10 | Reflection Memory と LLM 要約連携 | Summarizer の LLM 連携、memories type=reflection 保存 | 低 | 長期傾向の抽象化（Phase 2 以降でも可） | Todo |
| P10-11 | 記憶ガバナンス実装 | TTL 設定、削除 API、論理削除、監査ログ | 高 | 漏えい・肥大化リスクの抑制 | Todo |
| P10-12 | WebUI 記憶管理導線 | 記憶参照・削除 UI、facts 編集 UI、テスト API | 中 | 運用者が安全に扱える導線が必要 | Todo |
| P10-13 | 記憶品質評価と回帰テスト | key fact の再現テスト、誤想起テスト、CI 組込 | 中 | 記憶機能の品質保証を自動化 | Todo |
| P10-14 | ランブックと監査導線の整備 | memory 運用 runbook、障害切り分け手順、アクセスログ確認手順 | 中 | MTTR 短縮と運用標準化 | Todo |

## 6. 実装方針（ベストプラクティス反映）

### 6.1 キー設計

- 主キーは「意味を持たない immutable ID」を採用する
- user_id / device_id などは検索用インデックスで持ち、主キーにしない
- 記憶スコープキーは `user_id:persona_id` の複合文字列キーで管理する

### 6.2 記憶の保存判定ルール

```
保存する：
  長期的に意味がある事実 / 強い好み / 重要イベント / 繰り返し出る話題
  シグナルワード：「好き」「嫌い」「いつも」「家族」「作っている」「覚えて」

保存しない：
  単なるあいづち / 一時的な指示 / 使い捨て雑談 / 意味のない短文
```

### 6.3 矛盾・陳腐化の管理

- Semantic Memory（MemoryFact）は (user_id, key, scope) の UNIQUE 制約で upsert 管理する
- confidence / updated_at でどちらが新しいかを判定する
- 古い値を完全削除する前に、旧 value を履歴として保持する設計を検討する

### 6.4 プライバシー

- memories / memory_facts には平文の機密情報（パスワード等）を保存しない
- source フィールドでデータ出所を記録し、外部送信ログを最小化する
- 家族スコープ（scope=family）アクセス制御を将来の user 認証と連動させる

### 6.5 データライフサイクル

- 書き込み時に type / category / importance / confidence を必ず付与する
- 重要度（importance）が低いものは自動で論理削除対象にする
- 削除要求は論理削除（deleted_at）→ 非同期物理削除の 2 段階で行う
- 監査要件がある操作は request_id を付与してログに残す

### 6.6 段階的拡張（Phase 別）

| フェーズ | 追加する記憶機能 |
| --- | --- |
| Phase 1（まず動かす） | Profile Memory + Semantic Facts + Session Summary |
| Phase 2 | Episodic Memory + Retriever スコアリング |
| Phase 3 | Reflection Memory + Embedding 検索（Qdrant） |
| Phase 4 | 家族スコープ、感情タグ、時間帯ペルソナ、IoT 連携 |

## 7. 可観測性（追加メトリクス）

最低限の観測項目：

| メトリクス名 | 説明 |
| --- | --- |
| memory_lookup_latency_ms | 記憶取得レイテンシ |
| memory_hit_rate | cache-aside ヒット率 |
| memory_invalidate_count | cache 無効化回数 |
| memory_items_by_type | タイプ別記憶件数 |
| memory_delete_count | 削除操作回数 |
| identity_conflict_count | 同一 device_id 競合接続数 |
| extractor_candidate_count | PostProcess で抽出した記憶候補数 |
| extractor_save_rate | 候補のうち実際に保存した割合 |

## 8. 受け入れ条件（フェーズ 10）

- 同一サーバーに 3 台以上の Stackchan が接続しても識別が混線しない
- 再接続後に同一 device_id の Semantic Memory（facts）が再現される
- session_id が変化しても長期記憶の取得が破綻しない
- profile / facts が正しく LLM プロンプトへ反映されることをテストで確認できる
- 記憶削除 API が機能し、監査ログで追跡できる
- 記憶品質評価テストが CI で継続実行できる

## 9. 互換性・移行メモ

- 既存 utterances は session_id 基準のため、長期記憶へは段階移行とする
- 移行手順は追加型で実施する（破壊的変更なし）
  1. sessions に device_id / user_id を追加（nullable migration）
  2. handshake で device_id を sessions に保存するよう更新
  3. 新規書き込みを device_id 付きに切替
  4. 既存データを backfill（バッチまたは session 再接続契機）
  5. 長期記憶取得を memory_scope_key ベースへ切替

## 10. 調査参照（ベストプラクティス）

- Azure Architecture: request を tenant へ安全にマップし、受信識別子の検証を行う
  - https://learn.microsoft.com/en-us/azure/architecture/guide/multitenant/considerations/map-requests
- Azure Architecture: identity は immutable ID を基準に設計し、tenant 境界を明示する
  - https://learn.microsoft.com/en-us/azure/architecture/guide/multitenant/considerations/identity
- Azure Architecture: cache-aside の更新順序と失効戦略
  - https://learn.microsoft.com/en-us/azure/architecture/patterns/cache-aside
- Twelve-Factor App: config/secrets をコードから分離する
  - https://12factor.net/config
- OpenAI Developers: guardrails、state/memory、評価導線を運用設計に組み込む
  - https://developers.openai.com/tracks/building-agents/

---

本ドキュメントは初版です。実装と検証結果に応じて、実際の schema 名称、API 名称、運用手順へ更新してください。
