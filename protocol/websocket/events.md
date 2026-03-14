# WebSocket Event Contracts (Protocol v0)

## 1. Scope

この文書は Stackchan のプロトコル v0 における WebSocket イベント契約を定義します。
対象は次のイベントセットです（フェーズ 4 で拡張）。

- session.hello
- session.welcome
- error
- audio.chunk
- audio.end
- audio.stream_open （フェーズ 4 追加）
- stt.final （フェーズ 4 追加）
- tts.end （フェーズ 4 追加）

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

## 5. フェーズ 4 追加イベント

### 5.1 audio.stream_open

- Direction: firmware -> server
- Purpose: バイナリフレーム送信を開始する前に、ストリームのコーデック・フォーマット情報を登録する
- JSON Schema: protocol/websocket/schemas/ 未作成（フェーズ 5 で追加予定）
- Payload fields:
  - stream_id: string (required, UUID 推奨)
  - codec: string (required, enum: opus, pcm)
  - sample_rate_hz: integer (required, enum: 8000, 16000, 22050, 24000, 44100, 48000)
  - frame_duration_ms: integer (required, enum: 10, 20, 40, 60)
  - channel_count: integer (required, enum: 1, 2)
- Note: この後続のバイナリ WebSocket フレームはフォーマット § 6.1 を参照

### 5.2 stt.final

- Direction: server -> firmware
- Purpose: STT 処理が完了した認識テキストを通知する
- JSON Schema: protocol/websocket/schemas/stt.final.schema.json
- Payload fields:
  - request_id: string (required) — フェーズ 4 では stream_id と同値
  - transcript: string (required)
  - confidence: number (optional, 0–1)
- 将来候補: `stt.partial`（部分認識結果のストリーミング）

### 5.3 tts.end

- Direction: server -> firmware
- Purpose: TTS 合成が完了した音声データと再生メタデータを通知する
- JSON Schema: protocol/websocket/schemas/tts.end.schema.json
- Payload fields:
  - request_id: string (required) — stt.final の request_id と一致
  - audio_base64: string (required) — Base64 エンコードされた音声データ
  - duration_ms: integer (required, minimum: 1)
  - sample_rate_hz: integer (required, enum: 8000, 16000, 22050, 24000, 44100, 48000)
  - codec: string (required, enum: opus, pcm)
- 将来候補: `tts.chunk`（音声ストリーミング配信）

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
