# Audio Transport

## 1. 目的

音声輸送の仕様を固定し、firmware/server/protocol の手戻りを防ぎます。

## 2. 正式ルート

- 伝送方式: WebSocket
- 制御フレーム: JSON envelope
- 音声フレーム: binary frame
- 正式 codec: `opus`

> 方針: production 相当の音声輸送は `binary + opus` を標準とし、`pcm` は開発用 fallback として扱う。

## 3. ストリーム開始

`audio.stream_open` を送信し、次の情報を明示する。

- `stream_id`
- `codec`
- `sample_rate_hz`
- `frame_duration_ms`
- `channel_count`

`audio.stream_open` が未送信の状態で binary frame を受信した場合、server は `invalid_payload` を返す。

## 4. binary frame 形式

- bytes 0-35: stream_id（UUID 文字列 36 バイト固定）
- bytes 36- : 音声ペイロード

37 バイト未満は不正フレームとして扱う。

## 5. 互換モード（PCM fallback）

- 主用途: 開発初期のデバッグ、デバイス差異吸収
- 制約:
  - latency / bitrate / 音質評価の基準には使わない
  - CI の正式性能判定は Opus のみで行う
- 終了条件:
  - firmware 側 Opus 実装が安定した時点で、PCM を「非推奨」に格上げする

## 6. 観測項目

- stream open から first frame までの時間
- frame cadence jitter
- end-to-end latency（録音開始から再生開始まで）
- reconnect 発生時の欠損フレーム数
