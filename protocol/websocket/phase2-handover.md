# Phase 2 Handover Notes

## 1. Ready Artifacts

- events contract: protocol/websocket/events.md
- schemas: protocol/websocket/schemas/
- examples: protocol/examples/
- versioning rules: protocol/versioning.md
- validation checklist: protocol/websocket/validation-checklist.md

## 2. Server Skeleton Prerequisites

- WebSocket 受信時に envelope 検証を実施すること
- direction ごとの sequence 管理を行うこと
- error イベント返却を共通化すること
- session.hello 受信後に session.welcome を返却すること

## 3. Logging Requirements

- session_id を全ログに含める
- request_type と request_sequence を error ログに含める
- invalid_sequence 検知時に warning を出す

## 4. Open Questions

- audio.chunk を将来バイナリ転送へ拡張する際の分離方式
- session.welcome の heartbeat_interval_ms 既定値
- error code の詳細体系（provider 由来エラーの命名規則）
