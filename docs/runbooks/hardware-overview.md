# Hardware Overview 診断ランブック

## 1. 目的

Hardware Overview の運用時に、接続状態・ハードウェア状態レポート・診断ログを使って
問題を素早く切り分けるための手順を定義します。

対象:
- WebUI Hardware Overview
- GET /api/tests/hardware/state
- firmware -> server の device.state.report
- server の hardware dispatch 構造化ログ

---

## 2. 事前条件

- stackchan-server が起動していること
- WebSocket で firmware が接続済みであること
- WebUI が参照する API エンドポイントへ到達可能であること

確認コマンド:

```bash
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:8080/api/runtime/overview | jq .connection
```

期待値:
- connection.status が connected
- connection.session_id が空でない

---

## 3. 基本操作手順

### 手順 1: state.report 要求を送る

```bash
curl -fsS "http://localhost:8080/api/tests/hardware/state" | jq .
```

期待値:
- status: sent
- event_type: device.state.report
- target_session_id が返る

### 手順 2: Overview 反映を確認する

```bash
curl -fsS http://localhost:8080/api/runtime/overview | jq .hardware
```

期待値:
- last_report_at が更新される
- request_id が反映される
- rssi, free_heap_bytes, current_angle_x_deg, current_angle_y_deg が表示される

### 手順 3: WebUI で確認する

- Hardware Overview の最終更新時刻を確認
- 未接続時メッセージが出る場合は、connection.status を先に確認
- speaker_busy や camera_available は false でも異常とは限らない

---

## 4. 失敗時の見方

### 4.1 state 要求 API が失敗する

症状:
- /api/tests/hardware/state が 409 stackchan_not_connected を返す

確認:

```bash
curl -fsS http://localhost:8080/api/runtime/overview | jq .connection
```

対処:
- firmware 側の Wi-Fi / WS 再接続状態を確認
- session.hello -> session.welcome が成立しているか確認

### 4.2 state 要求は成功するが Overview に反映されない

症状:
- dispatch は sent だが hardware.last_report_at が更新されない

確認:
- server ログに device.state.report received が出ているか
- firmware シリアルログに StateReport sent が出ているか

対処:
- protocol version 不一致の有無を確認
- firmware 側 event router で device.state.report を受理しているか確認

### 4.3 値が異常に見える

チェック観点:
- rssi: 極端に低い場合は通信品質問題の可能性
- free_heap_bytes: 短時間で単調減少する場合はメモリリーク疑い
- current_angle_x_deg / y_deg: 校正範囲外は clamp ログと合わせて確認
- mic_level: 現フェーズでは 0 固定実装の可能性があるため仕様を確認

---

## 5. 構造化ログ項目

### 5.1 hardware dispatch ログ

イベント:
- hardware control dispatched
- hardware control dispatch failed

主なフィールド:
- component=hardware_dispatch
- event_type
- request_id
- command
- session_id (成功時)
- dispatch_timeout_ms
- error_code (失敗時)
- error_message (失敗時)

### 5.2 state report 受信ログ

イベント:
- device.state.report received

主なフィールド:
- request_id
- rssi
- free_heap_bytes
- speaker_busy
- camera_available

---

## 6. ログ確認コマンド

```bash
# server ログ全体
docker compose -f infra/docker/docker-compose.yml logs -f stackchan-server

# dispatch 成功 / 失敗を絞り込み
 docker compose -f infra/docker/docker-compose.yml logs -f stackchan-server | grep -E "hardware control dispatched|hardware control dispatch failed"

# state.report 受信を絞り込み
docker compose -f infra/docker/docker-compose.yml logs -f stackchan-server | grep "device.state.report received"
```

---

## 7. エスカレーション基準

- 5 分以上連続して stackchan_not_connected が続く
- dispatch_timeout が断続的に発生し、再接続後も改善しない
- free_heap_bytes の継続的低下が観測され、再起動でのみ回復する

上記に該当する場合は、以下を添えて調査依頼すること:
- /api/runtime/overview の出力
- 直近 5 分の stackchan-server ログ
- firmware シリアルログ（接続前後）
