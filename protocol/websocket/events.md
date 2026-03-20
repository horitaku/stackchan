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
- tts.buffer.watermark （フェーズ 8/P8-19 追加）
- device.servo.move （フェーズ 11/P11-05 追加）
- device.servo.calibration.get （フェーズ 11/P11-05 追加）
- device.servo.calibration.set （フェーズ 11/P11-05 追加）
- device.servo.calibration.response （フェーズ 11/P11-05 追加）
- device.led.set （フェーズ 11/P11-06 追加）
- device.ears.set （フェーズ 11/P11-06 追加、NECO MIMI オプション）
- device.audio.test.play （フェーズ 11/P11-07 追加）
- device.mic.test.start （フェーズ 11/P11-07 追加）
- device.camera.capture （フェーズ 11/P11-07 追加）
- device.state.report （フェーズ 11/P11-07 追加）

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

### 4.3 device.servo 系 error codes (フェーズ 11)

- device.servo.move で axis=x なのに angle_x_deg が存在しない場合: `invalid_payload`（message: "angle_x_deg required when axis=x"）
- device.servo.move で axis=y なのに angle_y_deg が存在しない場合: `invalid_payload`（message: "angle_y_deg required when axis=y"）
- device.servo.move で axis=both なのに angle_x_deg/angle_y_deg の一方が存在しない場合: `invalid_payload`（message: "both angle_x_deg and angle_y_deg required when axis=both"）
- device.servo.calibration.set で min_deg >= max_deg の場合: `invalid_payload`（message: "min_deg must be less than max_deg"）
- calibration の不揮発保存に失敗した場合: `device_error`（message: "servo calibration save failed", retryable: true）
- 未接続のセッションへ servo コマンドが届いた場合: `session_not_found`（message: "no active stackchan session found"）

### 4.4 device.led / device.ears 系 error codes（フェーズ 11）

- device.led.set で mode=solid/blink/breathe なのに color が省略されている場合: `invalid_payload`（message: "color required for mode solid/blink/breathe"）
- device.ears.set で mode=solid/blink/breathe なのに color が省略されている場合: `invalid_payload`（message: "color required for mode solid/blink/breathe"）
- NECO MIMI が未接続の場合（device.ears.set): firmware は warning ログを出力し、エラーは返さない（silent ignore ポリシー）

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
  - alternatives: array (optional) — 候補テキスト一覧。各要素は `transcript`（required）+ `confidence`（optional）
  - context_hint: string (optional) — LLM 側の曖昧性解消に使う補助情報
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
  - codec: string (optional, v1.1, enum: opus, pcm) — 未指定時は pcm 扱い
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

### 5.11 tts.buffer.watermark（P8-19）

- Direction: firmware -> server
- Purpose: TTS 再生バッファの watermark 状態変化を server へ通知し、ネットワーク揺らぎの定量分析と runtime_metrics 記録を可能にする
- JSON Schema: `protocol/websocket/schemas/tts.buffer.watermark.schema.json`
- Example: `protocol/examples/tts.buffer.watermark.example.json`
- Payload fields:
  - request_id: string (required)
  - stream_id: string (required)
  - status: string (required, enum: normal, low_water, high_water)
  - buffered_ms: integer (required, minimum: 0) — 現在のバッファ深さ（ms）
  - threshold_ms: integer (required, minimum: 0) — 発火した watermark 閾値（ms）
  - frames_in_queue: integer (required, minimum: 0) — 現在のキュー内フレーム数
- 送信タイミング: watermark 状態が変化した時点のみ（low_water 持続中の連続送信は禁止）
- 送信レート制限: 同一状態での再送は500ms 以上間隔を空ける

## 5.12 device.servo.move（P11-05）

- Direction: server -> firmware
- Purpose: X/Y 軸のサーボを指定した論理角度へ移動する。firmware が校正値（center_offset_deg / min_deg / max_deg / invert / speed_limit_deg_per_sec）を適用して実角度へ変換する
- JSON Schema: `protocol/websocket/schemas/device.servo.move.schema.json`
- Example: `protocol/examples/device.servo.move.example.json`
- Payload fields:
  - request_id: string (optional) — 診断・ログ用相関 ID
  - axis: string (required, enum: x, y, both) — 移動軸。x=水平（左右）、y=垂直（上下）、both=両軸同時
  - angle_x_deg: number (optional, -90–90) — X 軸目標論理角度（度）。axis=x または both の場合は必須
  - angle_y_deg: number (optional, -90–90) — Y 軸目標論理角度（度）。axis=y または both の場合は必須
  - speed: number (optional, 0.1–3.0) — 速度倍率。校正値の speed_limit_deg_per_sec が絶対上限
- Safety rule: firmware は angle を [min_deg, max_deg] でクランプしてから校正値を適用する。クランプが発生した場合は warning ログを出力する
- Event role: control command（即時反映、ack なし。エラー時のみ error イベント）

## 5.13 device.servo.calibration.get（P11-05）

- Direction: server -> firmware
- Purpose: firmware が保持している現在のサーボ校正値を request/response パターンで取得する
- JSON Schema: `protocol/websocket/schemas/device.servo.calibration.get.schema.json`
- Example: `protocol/examples/device.servo.calibration.get.example.json`
- Payload fields:
  - request_id: string (required) — device.servo.calibration.response で mirror して返す相関 ID
- Response: `device.servo.calibration.response`（同一 session_id で firmware から送信）
- Event role: request/response。firmware は受信後なるべく速やかに response を返す

## 5.14 device.servo.calibration.set（P11-05）

- Direction: server -> firmware
- Purpose: 校正値を差分更新し、不揮発ストレージ（Preferences / SPIFFS 等）へ保存する
- JSON Schema: `protocol/websocket/schemas/device.servo.calibration.set.schema.json`
- Example: `protocol/examples/device.servo.calibration.set.example.json`
- Payload fields:
  - request_id: string (required) — 診断・ログ用相関 ID
  - axis: string (required, enum: x, y) — 更新対象の軸
  - center_offset_deg: number (optional, -45–45) — 機体中央のズレ補正値（度）
  - min_deg: number (optional, -90–0) — 論理角度の最小値（度）
  - max_deg: number (optional, 0–90) — 論理角度の最大値（度）
  - invert: boolean (optional) — サーボ回転方向の反転フラグ
  - speed_limit_deg_per_sec: number (optional, 1–360) — 角速度の上限（度/秒）
  - soft_start: boolean (optional) — スムーズな加減速（ソフトスタート）フラグ
  - home_deg: number (optional, -90–90) — この軸のホーム位置（論理角度）
- Constraint: min_deg < max_deg を firmware 側でも検証し、違反時は error イベントを返す
- Semantics: 差分更新。省略フィールドは現在値を保持する。保存後に設定を即時反映する
- Event role: control command（保存成功は ack なし、保存失敗時は error イベント）

## 5.15 device.servo.calibration.response（P11-05）

- Direction: firmware -> server
- Purpose: device.servo.calibration.get への応答。現在の校正値と現在角度を返す
- JSON Schema: `protocol/websocket/schemas/device.servo.calibration.response.schema.json`
- Example: `protocol/examples/device.servo.calibration.response.example.json`
- Payload fields:
  - request_id: string (required) — device.servo.calibration.get の request_id をそのまま返す
  - x: object (required) — X 軸の校正値（ServoAxisCalibration）
  - y: object (required) — Y 軸の校正値（ServoAxisCalibration）
  - current_angle_x_deg: number (optional, -90–90) — 現在の X 軸論理角度（診断用）
  - current_angle_y_deg: number (optional, -90–90) — 現在の Y 軸論理角度（診断用）
- ServoAxisCalibration 共通フィールド（x/y 共通）:
  - center_offset_deg: number (required, -45–45)
  - min_deg: number (required, -90–0)
  - max_deg: number (required, 0–90)
  - invert: boolean (required)
  - speed_limit_deg_per_sec: number (required, 1–360)
  - soft_start: boolean (required)
  - home_deg: number (required, -90–90)

## 5.16 device.led.set（P11-06）

- Direction: server -> firmware
- Purpose: M5GO Bottom3 の RGB LED の点灯パターンと色を制御する
- JSON Schema: `protocol/websocket/schemas/device.led.set.schema.json`
- Example: `protocol/examples/device.led.set.example.json`
- Payload fields:
  - request_id: string (optional) — 診断・ログ用相関 ID
  - mode: string (required, enum: off, solid, blink, breathe) — 点灯パターン
  - color: string (optional, pattern: #RRGGBB) — RGB カラーコード。mode=solid/blink/breathe の場合は必須
  - brightness: integer (optional, 0–255) — 輝度。省略時は firmware のデフォルト値
  - blink_interval_ms: integer (optional, 50–5000) — blink モード時の点滅間隔（ms）
  - breathe_period_ms: integer (optional, 200–10000) — breathe モード時の明暗 1 周期（ms）
- Event role: control command（即時反映、ack なし。バリデーションエラー時のみ error イベント）

## 5.17 device.ears.set（P11-06）

- Direction: server -> firmware
- Purpose: NECO MIMI（NeoPixel）の点灯パターンと色を制御する。NECO MIMI はオプションハードウェアのため、未接続時は firmware が warning ログを出力し、エラーは返さない
- JSON Schema: `protocol/websocket/schemas/device.ears.set.schema.json`
- Example: `protocol/examples/device.ears.set.example.json`
- Payload fields:
  - request_id: string (optional) — 診断・ログ用相関 ID
  - mode: string (required, enum: off, solid, blink, breathe, rainbow) — 点灯パターン。rainbow=レインボーサイクル
  - color: string (optional, pattern: #RRGGBB) — RGB カラーコード。mode=solid/blink/breathe の場合は必須
  - brightness: integer (optional, 0–255) — 輝度。省略時は firmware のデフォルト値
  - blink_interval_ms: integer (optional, 50–5000) — blink モード時の点滅間隔（ms）
  - breathe_period_ms: integer (optional, 200–10000) — breathe モード時の明暗 1 周期（ms）
  - rainbow_period_ms: integer (optional, 200–30000) — rainbow モード時の色相 1 周期（ms）
- Optional hardware policy: firmware は `#ifdef FW_NECO_MIMI_ENABLED` ガードでコンパイル時に有効/無効を切り替える。実行時に未接続の場合は warning ログのみ出力し、エラーは送信しない
- Event role: control command（即時反映、ack なし）

## 5.18 device.audio.test.play（P11-07）

- Direction: server -> firmware
- Purpose: スピーカーテストトーンの再生を指示する
- JSON Schema: `protocol/websocket/schemas/device.audio.test.play.schema.json`
- Example: `protocol/examples/device.audio.test.play.example.json`
- Payload fields:
  - request_id: string (required) — 診断・ログ用相関 ID
  - tone_hz: integer (optional, 100–3000) — テストトーン周波数（Hz）
  - duration_ms: integer (optional, 100–5000) — 再生時間（ms）
  - volume: number (optional, 0.0–1.0) — 再生音量
- Event role: control command（即時反映、ack なし）

## 5.19 device.mic.test.start（P11-07）

- Direction: server -> firmware
- Purpose: マイク入力テストを開始する
- JSON Schema: `protocol/websocket/schemas/device.mic.test.start.schema.json`
- Example: `protocol/examples/device.mic.test.start.example.json`
- Payload fields:
  - request_id: string (required) — 診断・ログ用相関 ID
  - duration_ms: integer (optional, 100–10000) — テスト収音時間（ms）
  - sample_rate_hz: integer (optional, enum: 8000, 16000, 22050, 24000, 44100, 48000) — 収音サンプルレート（Hz）
  - frame_duration_ms: integer (optional, enum: 10, 20, 40, 60) — フレーム長（ms）
- Event role: control command（即時反映、ack なし）

## 5.20 device.camera.capture（P11-07）

- Direction: server -> firmware
- Purpose: カメラ静止画取得を指示する（初期フェーズは capture 要求のみ）
- JSON Schema: `protocol/websocket/schemas/device.camera.capture.schema.json`
- Example: `protocol/examples/device.camera.capture.example.json`
- Payload fields:
  - request_id: string (required) — 診断・ログ用相関 ID
  - resolution: string (optional, enum: qqvga, qvga, vga) — 撮影解像度
  - quality: integer (optional, 1–63) — JPEG quality（1=高品質, 63=低品質）
- Event role: control command（即時反映、ack なし）

## 5.21 device.state.report（P11-07）

- Direction: bidirectional
- Purpose: 診断状態の要求（server -> firmware）と状態レポート通知（firmware -> server）
- JSON Schema: `protocol/websocket/schemas/device.state.report.schema.json`
- Examples:
  - request: `protocol/examples/device.state.report.request.example.json`
  - response: `protocol/examples/device.state.report.response.example.json`
- Request payload fields (server -> firmware):
  - request_id: string (required) — 診断要求の相関 ID
  - source: string (optional) — 要求元識別子（例: webui.hardware_test）
- Response payload fields (firmware -> server):
  - request_id: string (optional) — 要求由来の相関 ID
  - source: string (optional) — レポート生成元識別子
  - uptime_ms: integer (required, minimum: 0)
  - rssi: integer (required, -120–0)
  - free_heap_bytes: integer (required, minimum: 0)
  - current_angle_x_deg: number (required, -90–90)
  - current_angle_y_deg: number (required, -90–90)
  - calibration: object (required) — x/y 軸の校正値
  - mic_level: number (required, 0.0–1.0)
  - speaker_busy: boolean (required)
  - camera_available: boolean (required)
  - firmware_version: string (optional)
- Event role: telemetry + request/response

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

### 7.2 フェーズ 11 互換性メモ（device.servo 系）

- `device.servo.move` / `device.servo.calibration.get` / `device.servo.calibration.set` / `device.servo.calibration.response` はすべて新規イベント追加であり、既存イベントの required フィールドは変更しない。
- 旧 firmware が未知イベントを受信した場合は、warning ログを残して無視してよい（v0 の後方互換ポリシーを継承）。
- サーボ未実装の firmware と共存するため、`device.servo.calibration.get` に対して response が返らない場合は server 側でタイムアウト（推奨: 3 秒）してエラーとして扱う。
- rollout 順序は protocol 定義（P11-05） -> firmware ServoController 実装（P11-02） -> server hardware test API（P11-11） -> WebUI（P11-12）とする。
- `device.servo.calibration.set` の差分更新セマンティクス（省略フィールドは現在値保持）は v0 内の additive change として許容する。

### 7.3 フェーズ 11 互換性メモ（device.led / device.ears 系）

- `device.led.set` / `device.ears.set` はすべて新規イベント追加であり、既存イベントの required フィールドは変更しない。
- NECO MIMI 未接続機では `device.ears.set` を silent ignore するため、server 側は response を期待しない（fire-and-forget）。
- `FW_NECO_MIMI_ENABLED` コンパイルフラグで NECO MIMI 対応を有効/無効化できる。未定義時は無効扱いとし、ランタイム警告だけ出す実装でもよい。
- rollout 順序は protocol 定義（P11-06） -> firmware LedController / EarsController 実装（P11-03） -> server hardware test API（P11-11） -> WebUI（P11-12）とする。
- 既存 firmware がこれらのイベントを受け取っても warning ログを残して無視してよい（v0 の後方互換ポリシーを継承）。

### 7.4 フェーズ 11 互換性メモ（audio / mic / camera / state report 系）

- `device.audio.test.play` / `device.mic.test.start` / `device.camera.capture` / `device.state.report` は新規イベント追加であり、既存イベントの required フィールドは変更しない。
- `device.state.report` は同一 type を request（server -> firmware）と response（firmware -> server）で共用する。`payload` は oneOf で分岐し、既存 reader は unknown field tolerant を維持する。
- camera 未実装 firmware と共存するため、`device.camera.capture` は未対応機で warning ログを残して無視してよい（v0 後方互換ポリシー）。
- rollout 順序は protocol 定義（P11-07） -> firmware ハンドラー実装（P11-04/P11-10） -> server hardware test API（P11-11） -> WebUI（P11-12）とする。

## 8. Deferred Candidates

interrupt 系 3 イベントはフェーズ 8 で正式化済み。`tts.buffer.watermark` は P8-19 で追加。P11-07 の最小診断イベント追加も完了。次候補は別途 backlog で管理する。
