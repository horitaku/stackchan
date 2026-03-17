# WebSocket Event Contracts (Protocol v0)

## 1. Scope

この文書は Stackchan のプロトコル v0 における WebSocket イベント契約を定義します。
対象は次のイベントセットです（フェーズ 5 で拡張）。

- session.hello
- session.welcome
- error
- audio.chunk
- audio.end
- audio.stream_open （フェーズ 4 追加）
- stt.final （フェーズ 4 追加）
- tts.end （フェーズ 4 追加）
- heartbeat （フェーズ 5 追加）
- avatar.expression （フェーズ 6 追加）
- motion.play （フェーズ 6 追加）
- conversation.cancel （フェーズ 8 追加）
- tts.stop （フェーズ 8 追加）
- audio.stream_abort （フェーズ 8 追加）

## 2. Common Envelope

すべての JSON メッセージは次の共通エンベロープを持ちます。

- type: string
  - イベント種別。固定文字列。
- timestamp: string
  - RFC3339 形式 UTC 時刻。
- session_id: string
  - セッション識別子。`session.hello` では空文字を許容。
- sequence: integer
  - 送信元ごとの単調増加シーケンス番号。1 以上。
- version: string
  - プロトコルバージョン。v0 では `1.0` を使用。
- payload: object
  - イベント固有の内容。

### Sequence Rules

- sequence は direction（firmware->server / server->firmware）ごとに単調増加。
- 同一 direction で sequence 重複が発生した場合は重複メッセージとして扱い、再処理しない。
- 欠番は許容するが、順序逆転を検知した場合は warning ログを残す。

## 3. Event Definitions

### 3.1 session.hello

- Direction: firmware -> server
- Purpose: セッション開始要求とデバイス情報通知
- Payload fields:
  - device_id: string (required)
  - client_type: string (required, enum: firmware, test_harness)
  - protocol_capabilities: object (optional)
    - audio_chunk: boolean
    - audio_end: boolean

### 3.2 session.welcome

- Direction: server -> firmware
- Purpose: セッション確立通知
- Payload fields:
  - accepted: boolean (required)
  - server_time: string (required, RFC3339)
  - heartbeat_interval_ms: integer (optional, minimum: 1000)
  - message: string (optional)

### 3.3 error

- Direction: bidirectional
- Purpose: エラー通知
- Payload fields:
  - code: string (required)
  - message: string (required)
  - retryable: boolean (required)
  - request_type: string (optional)
  - request_sequence: integer (optional, minimum: 1)

### 3.4 audio.chunk

- Direction: firmware -> server
- Purpose: 音声フレームメタデータ通知（v0 は JSON のみ定義）
- Payload fields:
  - stream_id: string (required)
  - chunk_index: integer (required, minimum: 0)
  - codec: string (required, enum: opus)
  - sample_rate_hz: integer (required, enum: 16000, 24000, 48000)
  - frame_duration_ms: integer (required, enum: 10, 20, 40, 60)
  - channel_count: integer (required, enum: 1, 2)
  - data_base64: string (required)

### 3.5 audio.end

- Direction: firmware -> server
- Purpose: 音声ストリーム終端通知
- Payload fields:
  - stream_id: string (required)
  - final_chunk_index: integer (required, minimum: 0)
  - reason: string (optional, enum: normal, cancel, timeout, error)

## 4. Error Handling

- 必須フィールド不足時は `error` イベントを返す。
- code の最小セット:
  - invalid_message
  - unsupported_version
  - invalid_sequence
  - invalid_payload
  - provider_unavailable
  - provider_timeout
  - provider_failed

### 4.1 audio stream error codes (フェーズ 4)

- audio.end 受信時にバッファが空の場合: `invalid_payload`（message: "audio stream is empty"）
- バッファ上限超過（500 チャンク）時: `invalid_payload`（message: "audio stream buffer overflow"）
- binary frame のフォーマット不正時: `invalid_payload`
- audio.stream_open なしでバイナリフレーム受信時: `invalid_payload`

### 4.2 interrupt 系 error codes (フェーズ 8)

- conversation.cancel 受信時に active conversation が存在しない場合: `invalid_payload`（message: "no active conversation to cancel"）
- tts.stop 対象の再生が存在しない場合: `invalid_payload`（message: "no active tts playback to stop"）
- audio.stream_abort の stream_id が未登録の場合: `invalid_payload`（message: "audio stream not found"）

## 5. フェーズ 4 追加イベント

### 5.1 audio.stream_open

- Direction: firmware -> server
- Purpose: バイナリフレーム送信を開始する前に、ストリームのコーデック・フォーマット情報を登録する
- JSON Schema: `protocol/websocket/schemas/audio.stream_open.schema.json`
- Example: `protocol/examples/audio.stream_open.example.json`
- Payload fields:
  - stream_id: string (required, UUID 推奨)
  - codec: string (required, enum: opus, pcm)
  - sample_rate_hz: integer (required, enum: 8000, 16000, 22050, 24000, 44100, 48000)
  - frame_duration_ms: integer (required, enum: 10, 20, 40, 60)
  - channel_count: integer (required, enum: 1, 2)
- Note: この後続のバイナリ WebSocket フレームはフォーマット § 6.1 を参照
- Normative note: 正式運用ルートは `codec=opus` とする。`codec=pcm` は開発互換の fallback としてのみ許容する。

### 5.2 stt.final

- Direction: server -> firmware
- Purpose: STT 処理が完了した認識テキストを通知する
- JSON Schema: protocol/websocket/schemas/stt.final.schema.json
- Payload fields:
  - request_id: string (required) — フェーズ 4 では stream_id と同値
  - transcript: string (required)
  - confidence: number (optional, 0–1)
- 将来候補: `stt.partial`（部分認識結果のストリーミング）

### 5.3 tts.chunk

- Direction: server -> firmware
- Purpose: TTS 音声データ本体を音声フレーム単位で通知する
- JSON Schema: protocol/websocket/schemas/tts.chunk.schema.json
- Payload fields:
  - request_id: string (required)
  - stream_id: string (required, v1.1)
  - chunk_index: integer (required, minimum: 0)
  - frame_duration_ms: integer (required, v1.1, enum: 10, 20, 40, 60)
  - samples_per_chunk: integer (required, v1.1, minimum: 1)
  - sent_at: string (optional, v1.1, RFC3339)
  - playout_ts: string (optional, v1.1, RFC3339)
  - audio_base64: string (required) — Base64 エンコード済みの音声フレーム
  - total_chunks: integer (optional, minimum: 1) — v1.0 互換。v1.1 では非推奨
- Normative note: v1.1 では `sent_at` または `playout_ts` の少なくとも一方を必須とする
- Compatibility note: schema は `version=1.0`（旧 payload）と `version=1.1`（新 payload）を併存サポートする

### 5.4 tts.end

- Direction: server -> firmware
- Purpose: TTS 合成が完了した再生メタデータを通知する
- JSON Schema: protocol/websocket/schemas/tts.end.schema.json
- Payload fields:
  - request_id: string (required) — stt.final の request_id と一致
  - duration_ms: integer (required, minimum: 1)
  - sample_rate_hz: integer (required, enum: 8000, 16000, 22050, 24000, 44100, 48000)
  - codec: string (required, enum: opus, pcm)
  - total_chunks: integer (optional, minimum: 1)
- Backward compatibility: `audio_base64` は fallback 用に optional で残すが、標準経路では `tts.chunk` で音声本体を送る
- フェーズ 8 以降の標準経路では `codec=opus` を優先し、`codec=pcm` は開発互換で維持する

### 5.5 heartbeat

- Direction: firmware -> server
- Purpose: 接続維持のためのキープアライブ通知。`session.welcome` の `heartbeat_interval_ms` 間隔で送信する
- Payload fields:
  - uptime_ms: integer (required) — firmware 起動からの経過時間（ms）
  - rssi: integer (optional) — Wi-Fi 信号強度（dBm）
- サーバーは heartbeat 受信時に接続タイムアウトをリセットする
- `WS_READ_TIMEOUT` は `heartbeat_interval_ms × 3` 以上に設定すること（デフォルト 45s = 15s × 3）

### 5.6 avatar.expression

- Direction: server -> firmware
- Purpose: 発話内容に応じてアバター表情を更新する
- JSON Schema: `protocol/websocket/schemas/avatar.expression.schema.json`
- Example: `protocol/examples/avatar.expression.example.json`
- Payload fields:
  - request_id: string (required)
  - expression: string (required, enum: neutral, happy, sad, surprised)
  - intensity: number (optional, 0.0–1.0)

### 5.7 motion.play

- Direction: server -> firmware
- Purpose: サーボ等の最小モーション再生を指示する
- JSON Schema: `protocol/websocket/schemas/motion.play.schema.json`
- Example: `protocol/examples/motion.play.example.json`
- Payload fields:
  - request_id: string (required)
  - motion: string (required, enum: idle, nod, shake)
  - speed: number (optional, 0.1–3.0)

### 5.8 conversation.cancel

- Direction: firmware -> server
- Purpose: 会話処理中にユーザー割り込みを通知し、現在の conversation ターンを中断する
- JSON Schema: `protocol/websocket/schemas/conversation.cancel.schema.json`
- Example: `protocol/examples/conversation.cancel.example.json`
- Payload fields:
  - request_id: string (optional) — 中断対象の request_id（未指定時は active request を対象）
  - reason: string (required, enum: user_interrupt, barge_in, timeout, provider_error, session_end)
  - source: string (required, enum: touch, button, voice, system)

### 5.9 tts.stop

- Direction: server -> firmware
- Purpose: firmware 側で再生中の TTS を即時停止し、再生キューをクリアする
- JSON Schema: `protocol/websocket/schemas/tts.stop.schema.json`
- Example: `protocol/examples/tts.stop.example.json`
- Payload fields:
  - request_id: string (optional) — 停止対象の request_id（未指定時は active playback を対象）
  - reason: string (required, enum: interrupted, superseded, timeout, error, session_end)
  - clear_queue: boolean (optional, default: true)

### 5.10 audio.stream_abort

- Direction: firmware -> server
- Purpose: 収音ストリーム送信を途中中断したことを通知し、server 側の入力バッファを破棄する
- JSON Schema: `protocol/websocket/schemas/audio.stream_abort.schema.json`
- Example: `protocol/examples/audio.stream_abort.example.json`
- Payload fields:
  - stream_id: string (required)
  - reason: string (required, enum: interrupted, user_cancel, timeout, transport_error, device_error)
  - final_chunk_index: integer (optional, minimum: 0)

## 6. バイナリフレームフォーマット（フェーズ 4）

### 6.1 バイナリ WebSocket フレームの構造

audio.stream_open 後に送信するバイナリ WebSocket フレームの構造:

```text
[ bytes 0–35 ]  stream_id（UUID 文字列、ASCII 36 バイト固定長）
[ bytes 36–  ]  音声データ（Opus フレームまたは raw PCM）
```

- 先頭 36 バイトは stream_id を UTF-8 文字列として固定長で埋め込む（UUID v4 は常に 36 文字）
- 37 バイト未満のフレームは `invalid_payload` エラーとして扱う
- stream_id に対応する audio.stream_open が事前に送信されていない場合は `invalid_payload` エラー

## 7. Compatibility Note

- v0 では additive change のみ許可する。
- 新規フィールドは optional として追加し、既存フィールドの意味変更は行わない。
- breaking change は `version` を上げ、移行手順を versioning.md に記載する。

### 7.1 フェーズ 8 互換性メモ（interrupt 系）

- `conversation.cancel` / `tts.stop` / `audio.stream_abort` は新規イベント追加であり、既存イベントの required フィールドは変更しない。
- 受信側が未知イベントを受けた場合は、warning ログを残して無視してよい（v0 運用の後方互換ポリシー）。
- interrupt 導入前の実装と共存するため、`request_id` は optional とし active request 解決を許容する。
- rollout 順序は server 先行（受理のみ） -> firmware 送受信対応 -> strict validation 有効化とする。

## 8. Deferred Candidates

interrupt 系 3 イベントはフェーズ 8 で正式化済み。次候補は別途 backlog で管理する。
