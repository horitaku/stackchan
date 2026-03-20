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
- device.servo.move の axis が存在する
- device.servo.move で axis=x のとき angle_x_deg が存在する
- device.servo.move で axis=y のとき angle_y_deg が存在する
- device.servo.move で axis=both のとき angle_x_deg と angle_y_deg の両方が存在する
- device.servo.calibration.get の request_id が存在する
- device.servo.calibration.set の request_id と axis が存在する
- device.servo.calibration.set で min_deg < max_deg である（設定された場合）
- device.servo.calibration.response の request_id / x / y が存在する
- device.servo.calibration.response の x/y に required フィールド（center_offset_deg / min_deg / max_deg / invert / speed_limit_deg_per_sec / soft_start / home_deg）がすべて存在する
- device.led.set の mode が存在し、enum: off, solid, blink, breathe のいずれかである
- device.led.set で mode=solid/blink/breathe のとき color が存在する
- device.led.set の color が存在する場合 #RRGGBB 形式である
- device.led.set の brightness が存在する場合 0〜255 の整数である
- device.led.set の blink_interval_ms が存在する場合 50〜5000 の整数である
- device.led.set の breathe_period_ms が存在する場合 200〜10000 の整数である
- device.ears.set の mode が存在し、enum: off, solid, blink, breathe, rainbow のいずれかである
- device.ears.set で mode=solid/blink/breathe のとき color が存在する
- device.ears.set の color が存在する場合 #RRGGBB 形式である
- device.ears.set の brightness が存在する場合 0〜255 の整数である
- device.ears.set の blink_interval_ms が存在する場合 50〜5000 の整数である
- device.ears.set の breathe_period_ms が存在する場合 200〜10000 の整数である
- device.ears.set の rainbow_period_ms が存在する場合 200〜30000 の整数である
- NECO MIMI 未接続時、device.ears.set を受信した firmware は warning ログのみ出力しエラーを返さない

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
