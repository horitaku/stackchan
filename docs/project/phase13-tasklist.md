# フェーズ 13 タスクリスト（WebUI 内部リファクタリング）

## 1. このドキュメントの目的

フェーズ 13 を、既存 WebUI の外部挙動を維持しながら実装可能な内部リファクタリングタスクへ分解します。
本フェーズは機能追加よりも、肥大化した `App.svelte` の責務整理、変更容易性の向上、テスト可能性の改善を主眼とします。

フェーズ 13 の主眼は以下です：
- `App.svelte` に混在した責務（状態管理 / API 呼び出し / 画面レンダリング）を分離する
- セクション単位で Svelte コンポーネントを分割し、差分レビューを容易にする
- API 呼び出しを共通化し、エラーハンドリングの一貫性を高める
- 既存 UI 仕様を維持したまま、将来機能（メモリー運用導線など）を追加しやすい構造へ整える

## 2. フェーズ 13 で固定する前提

### 2.1 このフェーズで守ること

- WebUI の外部挙動と API 契約は原則維持する
- 既存エンドポイントの URL、HTTP メソッド、payload 形式を変更しない
- 画面の主要導線（Overview / Settings / Tests / Hardware Test）を壊さない
- 変更は段階的に行い、まず file split、その後に state/API の整理を行う

### 2.2 このフェーズで無理にやらないこと

- 大規模な UI デザイン刷新
- server 側 API の仕様変更
- protocol イベント定義の変更
- 本質的に別フェーズとなる新規機能追加（例: 新しい診断カテゴリ）

### 2.3 現在の App.svelte の主要責務

現状の `App.svelte` は少なくとも次の責務を持っています。

- runtime overview の定期更新
- settings / llm settings の読み書き
- pipeline / voicevox / llm テスト実行
- hardware test（speaker / mic / servo / led / ears / camera / state）
- camera 結果レンダリング（成功 / 失敗 / メタデータ）
- alert 判定と表示

本フェーズでは、これらを一度に書き換えず、壊れにくい順に切り分けます。

## 3. リファクタリング方針

### 3.1 第 1 段階: 画面コンポーネント分割

最初は API と状態の意味を変えず、表示責務をコンポーネントへ分離します。

想定例：

```txt
server/webui/src/
  App.svelte
  components/
    overview/
      RuntimeOverviewCards.svelte
      AlertList.svelte
    settings/
      RuntimeSettingsPanel.svelte
      LLMSettingsPanel.svelte
    tests/
      PipelineTestPanel.svelte
      VoicevoxUiTestPanel.svelte
      VoicevoxStackchanTestPanel.svelte
      LlmUiTestPanel.svelte
      LlmStackchanTestPanel.svelte
    hardware/
      HardwareAudioPanel.svelte
      HardwareServoPanel.svelte
      HardwareLightingPanel.svelte
      HardwareCameraPanel.svelte
```

この段階の目的：

- `App.svelte` のテンプレート肥大化を解消する
- UI 差分の影響範囲を局所化する
- テスト対象をパネル単位で分割できる下地を作る

### 3.2 第 2 段階: API 呼び出し層の分離

`fetchJSON` / `postHardware` と各テスト実行ロジックを `lib/api` に集約します。

想定例：

```txt
server/webui/src/lib/api/
  client.js
  runtime.js
  settings.js
  tests.pipeline.js
  tests.voicevox.js
  tests.llm.js
  tests.hardware.js
```

この段階の目的：

- 通信エラー整形の重複を削減する
- API 単位の責務境界を固定する
- 将来のモック化・自動テスト導入を容易にする

### 3.3 第 3 段階: 状態管理の整理

`App.svelte` 直下の大量な `let` をドメイン単位で分け、必要に応じて store 化します。

優先順は以下を推奨します。

1. runtime / alerts（参照頻度が高く副作用が少ない）
2. settings / llm settings（保存導線の責務が明確）
3. tests / hardware tests（状態数が多く、分離効果が大きい）

### 3.4 壊れやすい箇所の扱い

以下は特に注意します。

- `onMount` のタイマー管理（リーク防止）
- camera preview の結果状態（`cameraCaptureRecent` と表示条件）
- 既存 JSON 結果表示の互換性
- 例外時のメッセージ文言と表示位置

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P13-01 | App.svelte 責務マップを作成 | 責務一覧、状態一覧、イベントハンドラ一覧 | 高 | 先に責務の見える化をしないと分割時の漏れが発生しやすいため | 未着手 |
| P13-02 | UI 分割境界を定義 | コンポーネント一覧、props / events 設計 | 高 | 表示分割の基準を固定しないと再結合時に責務が逆流するため | 未着手 |
| P13-03 | Overview / Alerts を分離 | overview 系コンポーネント群 | 高 | 依存が比較的少なく、分離効果が早く得られるため | 未着手 |
| P13-04 | Settings パネルを分離 | Runtime / LLM 設定コンポーネント | 中 | 保存導線を独立させ、設定関連の変更を局所化するため | 未着手 |
| P13-05 | Pipeline / Voicevox / LLM テストパネルを分離 | tests 配下のコンポーネント群 | 高 | UI 行数の多くを占め、可読性への寄与が大きいため | 未着手 |
| P13-06 | Hardware パネルを分離 | audio / servo / lighting / camera コンポーネント群 | 高 | 現在最も状態量が多く、差分衝突を起こしやすいため | 未着手 |
| P13-07 | API クライアント層を追加 | `lib/api/*`、共通 error mapping | 高 | fetch ロジックの重複を削減し、保守性を向上するため | 未着手 |
| P13-08 | 状態管理をドメイン単位へ整理 | store または state helper | 中 | グローバルな `let` 群を整理し、副作用の追跡を容易にするため | 未着手 |
| P13-09 | 回帰確認チェックリストを整備 | 画面導線ごとの確認手順 | 高 | リファクタ単独フェーズでは回帰防止が成果そのものであるため | 未着手 |
| P13-10 | 実装結果の引き継ぎを docs へ反映 | 完了サマリ、設計境界、既知制約 | 中 | 後続フェーズで同じ前提で改修を進めるため | 未着手 |

## 5. 実施順の推奨

### スライス A: 表示責務の分離

- P13-01
- P13-02
- P13-03
- P13-04

成功条件：

- `App.svelte` が「ページ組み立て + 最小 orchestration」に縮小される
- 表示コンポーネントが props ベースで再利用できる

### スライス B: テスト導線の分離

- P13-05
- P13-06

成功条件：

- test / hardware パネルが独立コンポーネントとして分割される
- カメラ結果表示を含む既存 UX が維持される

### スライス C: API と状態の整理

- P13-07
- P13-08

成功条件：

- API 呼び出しの共通層が導入される
- 主要状態がドメイン境界で追跡しやすくなる

### スライス D: 回帰確認と引き継ぎ

- P13-09
- P13-10

成功条件：

- 回帰確認手順が文書化される
- 後続フェーズ向けの境界ルールが明文化される

## 6. 受け入れ方針（フェーズ 13）

- `App.svelte` の行数と責務が縮小され、ページの見通しが改善していること
- 既存 API 契約と主要導線が互換性を維持していること
- テストパネルと hardware パネルの変更が独立差分として扱えること
- 主要ハンドラで例外時メッセージが維持されていること
- リファクタ後の回帰確認手順が運用可能な形で残っていること

## 7. 後続フェーズへの引き継ぎ

- メモリー運用 UI（Phase 10 系）の追加は、分割済みコンポーネント境界に沿って導入する
- Hardware 診断の拡張は `hardware/*` 配下で完結させ、`App.svelte` へ逆流させない
- API 増加時は `lib/api` に追加し、コンポーネント側で `fetch` を直接呼ばない

---

本ドキュメントは初版です。実装が進んだら、実際のファイル分割結果、props / events 仕様、状態管理方針、回帰確認手順に合わせて更新してください。