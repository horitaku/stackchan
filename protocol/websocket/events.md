# WebSocket Event Contracts (Protocol v0)

## 1. Scope

この文書は Stackchan のプロトコル v0 における WebSocket イベント契約を定義します。
対象は次の最小イベントセットです。

- session.hello
- session.welcome
- error
- audio.chunk
- audio.end

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

## 5. Compatibility Note

- v0 では additive change のみ許可する。
- 新規フィールドは optional として追加し、既存フィールドの意味変更は行わない。
- breaking change は `version` を上げ、移行手順を versioning.md に記載する。
