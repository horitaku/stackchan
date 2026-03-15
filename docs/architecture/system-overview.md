# System Overview

## 1. 目的

Stackchan 再構築のランタイム境界と責務を 1 枚で把握できるようにします。
この文書は設計の「基準点」とし、実装詳細は各コンポーネント README に委譲します。

## 2. ランタイム境界

- firmware
  - デバイス I/O（mic, speaker, display, servo, touch）
  - WebSocket 接続管理（接続/再接続/heartbeat）
  - 最小状態管理（再生状態、表情状態）
- server
  - セッション管理
  - 会話オーケストレーション（STT -> LLM -> TTS）
  - API / WebSocket 提供、設定保存、可観測性集約
- protocol
  - WebSocket 契約（envelope、イベント、バージョニング）
  - サンプルと JSON Schema
- providers
  - provider 仕様、契約、将来の実装分離単位
- infra
  - Docker / Compose / migration / CI など運用再現性

## 3. データフロー（現行方針）

1. firmware が WebSocket 接続し、`session.hello` を送信する。
2. server が `session.welcome` でセッションを確立する。
3. firmware が `audio.stream_open` を送信し、続いてバイナリ音声フレームを送信する。
4. server が音声入力を STT へ渡し、LLM で応答文を生成し、TTS で音声化する。
5. server が `stt.final`、`tts.end`、`avatar.expression`、`motion.play` を返す。
6. firmware は再生とアバター同期を行い、必要時に `audio.end` を送信する。

## 4. 重要原則

- Protocol First: 実装前に契約を先に確定する。
- Thin Vertical Slices: 1 スライスずつ E2E で閉じる。
- Firmware Thin: firmware は賢くしすぎず、I/O と接続制御に集中する。
- Server Orchestration: 会話ロジックと provider 制御は server に集約する。

## 5. 次の固定事項

- 音声輸送の正規ルートは `binary + opus` とする。
- `pcm` は開発互換の fallback として維持する。
- 割り込み系イベント（conversation.cancel / tts.stop / audio.stream_abort）を protocol で先行定義する。
