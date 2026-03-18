# Protocol Validation Checklist (Phase 1)

## Envelope

- type, timestamp, session_id, sequence, version, payload が存在する
- timestamp が RFC3339 形式である
- sequence が 1 以上である

## Event-specific

- session.hello の device_id/client_type が存在する
- session.welcome の accepted/server_time が存在する
- error の code/message/retryable が存在する
- audio.chunk の stream_id/chunk_index/codec/data_base64 が存在する
- audio.end の stream_id/final_chunk_index が存在する
- tts.chunk(v1.1) の stream_id/chunk_index/frame_duration_ms/samples_per_chunk/audio_base64 が存在する
- tts.chunk(v1.1) は sent_at または playout_ts の少なくとも一方を持つ
- tts.chunk(v1.1) の codec が存在する場合は `pcm` または `opus` である
- conversation.cancel の reason/source が存在する
- tts.stop の reason が存在する
- audio.stream_abort の stream_id/reason が存在する

## Sequence and Ordering

- direction ごとに sequence が単調増加する
- 重複 sequence 時に再処理されない
- 順序逆転時に warning ログが出る

## Error Semantics

- invalid_message/unsupported_version/invalid_sequence/invalid_payload を返せる
- error.payload に request_type/request_sequence を関連付け可能

## Compatibility

- 追加フィールドは optional になっている
- 既存フィールドの型/意味変更がない
- versioning.md の更新が行われている
