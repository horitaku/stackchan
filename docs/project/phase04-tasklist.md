# フェーズ 4 タスクリスト（最小音声パス）

## 1. このドキュメントの目的

フェーズ 4（最小音声パス）を実行しやすくするために、作業を具体タスクへ分解して管理します。
本ドキュメントは「日次で更新する実行用リスト」です。

## 2. 運用ルール

- ステータスは `Planned`、`In Progress`、`Blocked`、`Done` を使用します。
- 作業開始時に `開始日` を記録し、完了時に `完了日` を記録します。
- `Blocked` になった場合は、必ず `ブロック理由` と `解除条件` を記載します。
- 1 タスクは原則 0.5 日から 2 日で終わる粒度に保ちます。

## 3. フェーズ 4 完了条件

次のすべてを満たしたらフェーズ 4 完了とします。

- audio.chunk（JSON/base64）をWebSocket経由で受信し、セッションに蓄積できる（フェーズ 3 から継続）。
- audio.end 受信後にオーケストレーション（STT → LLM → TTS）が起動し、結果イベント（stt.final / tts.end）がクライアントへ送信される。
- request_id が会話パイプライン全体（ws_handler → orchestrator → logs）に伝搬されている。
- バイナリ WebSocket フレームの受信と AudioChunk への変換が実装されている。
- 音声ストリームの開始から TTS 完了までのキュー滞留・各 provider レイテンシが構造化ログで計測されている。
- 空ストリーム・バッファ上限超過・オーケストレーション失敗に対して適切な error イベントが返される。
- テスト入力（mock provider）でフル音声ライフサイクルが成立する自動テストが存在する。
- フェーズ 5（Firmware 接続性）への引き継ぎ事項が記録されている。

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 依存 | 優先度 | 担当 | 見積 | ステータス | 開始日 | 完了日 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P4-01 | オーケストレーション結果イベント送信を実装する | ws_handler に sendEvent ヘルパーを追加、stt.final / tts.end の送信 | - | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-02 | プロトコルイベントを拡張する（stt.final / tts.end） | protocol/websocket/events.md 追記、JSON Schema 追加、ペイロード例追加 | P4-01 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-03 | request_id を会話パイプラインへ導入する | Orchestrator の入出力型に request_id 追加、ws_handler と全ログへ伝搬 | P4-01 | 高 | Copilot | 1.0 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-04 | バイナリ WebSocket フレーム受信を実装する | ws_handler の BinaryMessage 対応、binary フレームから AudioChunk への変換 | - | 高 | Copilot | 1.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-05 | 音声ストリームのエラー処理を強化する | 空ストリーム検知、バッファ上限超過の error 返却、ストリーム中断時のバッファクリーンアップ | P4-01 | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-06 | キュー滞留時間と provider レイテンシの計測を実装する | audio.end 受信〜各 provider 完了のレイテンシを構造化ログへ記録 | P4-01、P4-03 | 高 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-07 | 音声パスのエンドツーエンド自動テストを追加する | audio.chunk × N → audio.end → stt.final → tts.end の正常系、エラー系テスト | P4-01 から P4-06 | 高 | Copilot | 1.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-08 | フェーズ 5 引き継ぎ事項を整理する | Firmware 接続性に向けた前提・未決事項メモ | P4-01 から P4-07 | 中 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |
| P4-09 | フェーズ 4 完了レビューを実施する | レビュー記録、未解決課題リスト | P4-01 から P4-08 | 中 | Copilot | 0.5 日 | Done | 2026-03-14 | 2026-03-14 |

## 5. タスク詳細（実行手順）

### P4-01 オーケストレーション結果イベント送信を実装する

- 作業内容
  - ws_handler に `sendEvent` ヘルパーを追加し、構造化イベントを WebSocket に送信できるようにします。
  - `audio.end` 受信後の `orchestrator.ProcessAudioStream` 呼び出し結果に基づき、`stt.final` と `tts.end` をクライアントへ送信します。
  - `server -> firmware` 方向の sequence を `stt.final` / `tts.end` 送信のたびにインクリメントします。
  - 送信失敗時はログに記録し、セッションをクローズします。
- 完了条件
  - WebSocket クライアントが `audio.end` を送信した後に `stt.final` と `tts.end` を受信できます。
- 確認観点
  - `stt.final` の payload に transcript が含まれること。
  - `tts.end` の payload に audio_base64 / duration_ms / sample_rate_hz が含まれること。
  - sequence が `session.welcome` からの連番で正しく付与されること。

### P4-02 プロトコルイベントを拡張する（stt.final / tts.end）

- 作業内容
  - `protocol/websocket/events.md` に `stt.final` と `tts.end` のイベント定義を追記します。
  - `protocol/websocket/schemas/` に `stt.final.schema.json` と `tts.end.schema.json` を作成します。
  - `protocol/examples/` に正常系ペイロード例を追加します。
  - 将来の `stt.partial`（部分認識結果）と `tts.chunk`（音声ストリーミング）のための予備的な方針をコメントとして残します。
- 完了条件
  - P4-01 の実装に対して schema が一致しています。
- 確認観点
  - `direction` が `server -> firmware` であること。
  - `request_id` が必須フィールドとして含まれること。

### P4-03 request_id を会話パイプラインへ導入する

- 作業内容
  - `audio.end` の `stream_id` を `request_id` として ws_handler 内で確定します。
  - `Orchestrator.ProcessAudioStream` の引数に `requestID string` を追加します。
  - `conversation.Result` に `RequestID string` フィールドを追加します。
  - `stt.final` / `tts.end` の送信コードで `request_id` をペイロードに含めます。
  - 全ログエントリ（STT / LLM / TTS 完了ログ）に `request_id` を付与します。
- 完了条件
  - 1 件の `audio.end` → `stt.final` の往復ログで `request_id` が一貫して確認できます。
- 確認観点
  - `session_id` と `request_id` の両方が全関連ログに含まれること。
  - `request_id` が `stt.final` と `tts.end` のペイロードに含まれること。

### P4-04 バイナリ WebSocket フレーム受信を実装する

- 作業内容
  - `ws_handler.go` の `readLoop` を拡張し、`websocket.BinaryMessage` を処理します。
  - バイナリフレームのフォーマットを決定します（先頭バイトにフレームタイプ識別子を置くか、stream_id を別途 JSON で確立するかなど）。フェーズ 4 では `stream_id` のみ JSON で事前送信し、後続フレームをバイナリで受信する方式を最小実装とします。
  - バイナリフレームを `providers.AudioChunk` に変換し `Session.AddAudioChunk` を呼びます。
  - バイナリ受信中のタイムアウト（`WS_READ_TIMEOUT`）を JSON メッセージと同一の設定で適用します。
  - フォーマット不正なバイナリフレームに対して `error` を返します。
- 完了条件
  - バイナリフレームとして送信された音声データが AudioChunk としてバッファに蓄積されます。
- 確認観点
  - JSON メッセージとバイナリフレームが同一ループで処理されること。
  - バイナリフレームに対して JSON エンベロープ検証をスキップすること。

### P4-05 音声ストリームのエラー処理を強化する

- 作業内容
  - `audio.end` 受信時にバッファが空（チャンク 0 件）の場合、`error`（code: `invalid_payload`、message: 空ストリーム）を返します。
  - セッションごとのバッファ上限（デフォルト: 500 チャンク）を設定し、超過時に `error` を返してバッファをクリアします。
  - オーケストレーション失敗後もセッションが継続して次のストリームを受け付けられるよう状態を回復します。
  - ストリーム中断（切断など）が発生した場合、残留バッファを自動クリーンアップします。
- 完了条件
  - 空ストリームに対して `error` が返り、セッションが継続します。
  - バッファ上限超過で `error` が返り、バッファがリセットされます。
- 確認観点
  - エラー後の次の音声ストリームが正常に処理されること。

### P4-06 キュー滞留時間と provider レイテンシの計測を実装する

- 作業内容
  - `audio.chunk` 初回受信時刻をセッションに記録し、`audio.end` 受信時との差分を「キュー待機時間」として構造化ログに記録します。
  - `audio.end` → STT 呼び出し開始・完了、LLM 呼び出し開始・完了、TTS 呼び出し開始・完了のタイムスタンプを計測します。
  - Orchestrator のログ出力に `stt_latency_ms`、`llm_latency_ms`、`tts_latency_ms`、`total_latency_ms` フィールドを追加します。
  - `session_id` と `request_id` を全計測ログに付与します。
- 完了条件
  - 1 件の音声ライフサイクル完了後に各ステップのレイテンシがログで確認できます。
- 確認観点
  - `total_latency_ms` が STT + LLM + TTS の合計以上であること（オーバーヘッドを含む）。
  - キュー待機時間が 0 ms 以上の値として記録されること。

### P4-07 音声パスのエンドツーエンド自動テストを追加する

- 作業内容
  - `ws_handler_test.go` に音声ライフサイクルの正常系テストを追加します。
    - `session.hello` → `session.welcome` → `audio.chunk × 3` → `audio.end` → `stt.final` 受信 → `tts.end` 受信 の一連フロー。
  - `request_id` が `stt.final` と `tts.end` のペイロードに含まれることを検証します。
  - エラー系テストを追加します。
    - 空ストリームの `audio.end` で `error` が返ることを確認します。
    - Orchestrator 失敗時（mock でエラーを注入）に `error` が返ることを確認します。
  - バイナリ WebSocket フレームの単体テスト（P4-04 の変換ロジック）を追加します。
  - `go test ./... -v -timeout 120s` が通過することを確認します。
- 完了条件
  - 正常系・空ストリーム・オーケストレーション失敗の 3 シナリオが自動テストで検証されています。
- 確認コマンド例
  - `cd server ; go test ./... -v -timeout 120s`

### P4-08 フェーズ 5 引き継ぎ事項を整理する

- 作業内容
  - フェーズ 5（Firmware 接続性）に向けた前提条件と未決事項をまとめます。
  - フェーズ 4 で採用したバイナリフレームフォーマットを firmware 実装者向けに記録します。
  - `heartbeat_interval_ms` の運用値と再接続ポリシーの現状の假定を記録します。
  - フェーズ 4 で計測したレイテンシの初期値を記録し、フェーズ 5 以降の最適化基準とします。
- 完了条件
  - フェーズ 5 着手時のブロッカーが明確です。
- 確認観点
  - プロトコル、server、providers の責務境界が維持されていること。
  - Firmware 側が受信すべきイベント（`stt.final`、`tts.end`）の仕様が引き継がれること。

### P4-09 フェーズ 4 完了レビューを実施する

- 作業内容
  - フェーズ 4 完了条件の満たし込み確認を行います。
  - 未解決事項をフェーズ 5 へ引き継ぎます。
- 完了条件
  - 合意済みのレビュー結果が記録されています。
- 確認観点
  - フェーズ 5（Firmware 接続性）の作業開始判断が可能であること。

## 6. 前提と設計メモ

### 現状の実装状況（フェーズ 3 からの引き継ぎ）

| 項目 | 状態 | 備考 |
| --- | --- | --- |
| audio.chunk JSON 受信・バッファへの蓄積 | 実装済み | ws_handler.go、session/audio_stream.go |
| audio.end 受信・Orchestrator 起動 | 実装済み | ws_handler.go |
| STT → LLM → TTS パイプライン | 実装済み（mock） | conversation/orchestrator.go |
| 結果イベントのクライアントへの送信 | **未実装** | P4-01 で対応 |
| binary WebSocket フレーム受信 | **未実装** | P4-04 で対応 |
| request_id の全レイヤー伝搬 | **未実装** | P4-03 で対応 |
| キュー・レイテンシ計測 | **未実装** | P4-06 で対応 |

### 新規プロトコルイベントの概要（P4-02 で詳細化）

```text
stt.final（server -> firmware）
  payload:
    request_id: string (required)
    transcript: string (required)
    confidence: number (optional, 0-1)

tts.end（server -> firmware）
  payload:
    request_id: string (required)
    audio_base64: string (required)  // Opus/PCM 音声データ
    duration_ms: integer (required)
    sample_rate_hz: integer (required)
    codec: string (required)

将来拡張（フェーズ 5 以降の候補）:
  stt.partial（途中認識結果のストリーミング）
  tts.chunk（音声ストリーミング配信）
```

### バイナリフレームフォーマット方針（P4-04 設計候補）

フェーズ 4 では以下の 2 段階方式を最小実装とします。

1. JSON テキストメッセージ（`audio.stream_open` イベント）で `stream_id`、コーデック情報、サンプルレートを事前通知する。
2. 後続のバイナリメッセージでは、先頭 36 バイトに `stream_id`（UUID 文字列）を固定長で埋め込み、残りを Opus フレームのバイト列とする。

この方式は将来の変更を想定し、バイナリヘッダにバージョンバイト（1 バイト）を含めることを推奨します。

> 注: Opus エンコード/デコードの導入（`gopus` などのバインディング）は firmware 接続性確認後（フェーズ 5 以降）を推奨します。フェーズ 4 では raw PCM をバイナリ転送することを許容します。

### ディレクトリ構成（フェーズ 4 変更分の想定）

```text
server/
├── internal/
│   ├── web/
│   │   ├── ws_handler.go        // sendEvent 追加、binary 受信対応
│   │   └── ws_handler_test.go   // エンドツーエンドテスト追加
│   ├── conversation/
│   │   ├── orchestrator.go      // request_id 引数追加、レイテンシ計測強化
│   │   └── orchestrator_test.go // 既存テスト維持
│   ├── session/
│   │   ├── audio_stream.go      // バッファ上限・キュー開始時刻を追加
│   │   └── manager.go           // 既存を維持
│   └── protocol/
│       ├── events.go            // stt.final / tts.end の型定義を追加
│       └── ...                  // 既存を維持
protocol/
├── websocket/
│   ├── events.md                // stt.final / tts.end / 将来候補を追記
│   └── schemas/
│       ├── stt.final.schema.json    // 新規追加
│       └── tts.end.schema.json      // 新規追加
└── examples/
    ├── stt.final.example.json       // 新規追加
    └── tts.end.example.json         // 新規追加
```

### レイテンシ計測の観点

```text
T1: audio.chunk 初回受信時刻（stream_id ごと）
T2: audio.end 受信時刻  → キュー待機時間 = T2 - T1
T3: STT 呼び出し完了時刻 → STT レイテンシ = T3 - T2
T4: LLM 呼び出し完了時刻 → LLM レイテンシ = T4 - T3
T5: TTS 呼び出し完了時刻 → TTS レイテンシ = T5 - T4
合計遅延 = T5 - T2
```

## 7. ブロッカー管理

| 日付 | タスク ID | ブロック理由 | 解除条件 | オーナー | 状態 |
| --- | --- | --- | --- | --- | --- |
| - | - | - | - | - | - |

## 7.1 フェーズ 4 完了レビュー記録

- レビュー日: 2026-03-14
- レビュー結果: フェーズ 4 の完了条件をすべて満たす
- 確認事項:
  - audio.chunk（JSON）受信・バッファ蓄積は継続して動作確認済み
  - audio.end 受信後に stt.final / tts.end をクライアントへ送信することを確認済み
  - request_id（= stream_id）が orchestrator / 全ログ / stt.final / tts.end ペイロードに伝搬済み
  - バイナリ WebSocket フレーム（先頭 36 バイト = stream_id）の受信と AudioChunk 変換を実装済み
  - audio.stream_open イベントでバイナリストリームのメタ登録を実装済み
  - 空ストリーム検知・バッファ上限（500 チャンク）超過で invalid_payload を返却済み
  - STT / LLM / TTS レイテンシ（ms）とキュー待機時間を構造化ログに記録済み
  - E2E テスト（TestAudioFullLifecycle / TestAudioEnd_EmptyStream / TestAudioEnd_OrchestratorFailure / TestBinaryStreamOpenAndFrames）を追加し `go test ./... -v -timeout 120s` 全通過を確認済み
- 未解決課題:
  - Opus エンコード/デコード（gopus 等）の導入はフェーズ 5 以降（現在は raw PCM をバイナリ転送）
  - heartbeat_interval_ms の運用値はフェーズ 5 以降で確定
  - audio.stream_open の JSON Schema 作成はフェーズ 5 で実施

## 8. フェーズ 4 実績メモ

- 開始日: 2026-03-14
- 目標完了日: 2026-03-14
- 完了日: 2026-03-14
- 主な学び: ConsumeAudioStream の返り値変更（→ firstChunkAt 追加）と AddAudioChunk のエラー返却化により、キュー計測・バッファ管理をクリーンに実装できた。binary フレームのロジックを Session メソッドへ閉じ込めたことで ws_handler の関心を薄くに保てた。
- 次フェーズへの持ち越し: Furniture での実接続試験、Opus コーデック導入、heartbeat_interval_ms 運用値の確定
