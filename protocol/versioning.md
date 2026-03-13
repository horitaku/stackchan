# Protocol Versioning Policy

## 1. Version Field

- すべてのイベントに `version` を含める。
- v0 系は `1.x` とし、初期値は `1.0`。

## 2. Compatibility Rules

### Additive Changes (Backward Compatible)

次は後方互換として許可します。

- 新しい optional フィールドの追加
- 新しいイベント type の追加
- enum 値の追加（既存受信側が unknown を許容できる場合）

### Breaking Changes (Not Backward Compatible)

次は破壊的変更とみなします。

- required フィールドの追加
- 既存フィールドの削除
- 既存フィールドの型変更
- 既存フィールドの意味変更

## 3. Deployment Order for Breaking Changes

1. Reader 側で新旧フォーマットの dual-read を実装
2. Writer 側で新フォーマットの dual-write を実装
3. 全体切替後に旧フォーマットを削除

## 4. Deprecation Policy

- deprecated 対象は events.md に明記する。
- 最低 1 フェーズは互換期間を設ける。
- 廃止時は移行先イベントと例を示す。

## 5. Validation Requirements

- すべての example JSON が schema 検証に通ること。
- 追加変更時は互換性ノートを更新すること。
