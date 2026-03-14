# フェーズ 7 引き継ぎメモ（WebUI と可観測性）

## 1. 目的

この文書は、フェーズ 6（再生とアバター同期）で得られた結果をフェーズ 7（WebUI と可観測性）へ引き継ぐための実行メモです。

## 2. フェーズ 6 の完了サマリ

- firmware は `tts.end` の `audio_base64`（PCM）をデコードして再生できる。
- 音声再生中に振幅ベースの簡易リップシンク値を更新できる。
- `avatar.expression` / `motion.play` を受信し、最小反映（表示更新・安全な通知動作）が可能。
- 再生状態は `Idle / Buffering / Playing / Stopping / Error` で管理される。
- `websocket close 1009 (message too big)` は mock TTS 音声サイズを縮小して解消済み。

## 3. フェーズ 7 で可視化する監視項目

### 3.1 接続とセッション

- WebSocket 接続状態（Connected / Reconnecting / Disconnected）
- 現在の session_id
- 再接続回数、最終再接続時刻
- heartbeat 送信間隔と最終送信時刻

### 3.2 音声再生

- playback state（Idle / Buffering / Playing / Stopping / Error）
- request_id
- playback_start_latency_ms
- playback_duration_ms
- decode_error_count
- output_error_count

### 3.3 会話パイプライン

- stream_id / request_id
- queue_wait_ms
- stt_latency_ms
- llm_latency_ms
- tts_latency_ms
- total_latency_ms

### 3.4 アバター同期

- expression（neutral / happy / sad / surprised）
- motion（idle / nod / shake）
- lip_sync_level（0.0 - 1.0）
- lip_sync_update_interval_ms

## 4. フェーズ 7 で API 化する候補

- 再生音量設定
- 表情プリセット設定
- リップシンク係数（感度・減衰）
- モーション有効化フラグ（安全運用用）
- 診断情報の取得 API（最新 N 件ログ、集計メトリクス）

## 5. 未解決事項（フェーズ 7 へ持ち越し）

- Opus 再生未対応（現状は PCM 優先）
- `tts.chunk` の分割配信未対応（現状は `tts.end` 一括）
- `m5stack-avatar` への本統合未完了（現状は簡易オーバーレイ）
- モーションの実サーボ制御未完了（現状は最小動作）
- firmware 側の構造化ログ出力未整備（シリアルログ中心）

## 6. フェーズ 7 着手順序（推奨）

1. 可視化対象メトリクスのサーバー API を定義
2. WebUI の最小ダッシュボードを作成（接続・再生・遅延）
3. 設定更新 API を追加（音量・表情・同期係数）
4. WebUI からの疎通テスト実行導線を追加
5. 主要指標に対する閾値アラート表示を追加

## 7. 確認済み実機挙動（フェーズ 6）

- タッチ送信から `stt.final` 受信まで継続動作
- `tts.end` 受信後の再生開始・完了を確認
- 再生中に切断せず heartbeat が継続

以上をフェーズ 7 の初期要件として扱います。
