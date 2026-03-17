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

## 6. Phase 8 Interrupt Additions

- `conversation.cancel` / `tts.stop` / `audio.stream_abort` は新規イベント追加のため、v0（1.x）の additive change として扱う。
- 受信側は未知イベントを許容し、warning を残して無視できる実装を維持する。
- 導入順序は server 側受理 -> firmware 側送受信対応 -> strict validation 有効化とする。

## 7. Phase 8 tts.chunk Frame Redesign (P8-14)

- `tts.chunk` は v1.1 でフレーム単位 payload を導入する。
	- required: `request_id`, `stream_id`, `chunk_index`, `frame_duration_ms`, `samples_per_chunk`, `audio_base64`
	- timing: `sent_at` または `playout_ts` の少なくとも一方を必須
- `version=1.0`（旧 payload: `total_chunks` 前提）は互換期間中に限り受理する。
- 互換分類: 旧 writer に対しては additive（dual-read 前提）。strict に v1.1 必須化する段階では breaking。

### 7.1 Rollout Order

1. Reader（firmware/server）で `version=1.0` と `1.1` の dual-read を有効化する
2. Writer（server）で `version=1.1` を優先送信し、必要時のみ `1.0` fallback を許容する
3. 観測期間で `1.0` トラフィックが解消されたことを確認する
4. `1.0` fallback を削除し、`1.1` を strict 運用へ切り替える
