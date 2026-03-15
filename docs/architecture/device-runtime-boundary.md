# Device Runtime Boundary

## 1. 目的

firmware が担う責務と、server に委譲する責務を明確に分離します。

## 2. firmware が持つ責務

- デバイス I/O
  - mic / speaker / display / servo / touch
- 接続制御
  - Wi-Fi 接続
  - WebSocket 再接続
  - heartbeat 送信
- 最小ランタイム状態
  - 再生状態
  - 表情状態
  - 安全停止
- protocol 適合
  - envelope 構築
  - sequence 管理
  - 必須イベント送受信

## 3. firmware に持ち込まない責務

- STT/LLM/TTS の選定・切替
- 会話フロー分岐（ツール選択、memory 参照、persona 決定）
- provider リトライ戦略の高度化
- ユーザー設定の永続化ロジック

## 4. server が持つ責務

- 会話オーケストレーション
- provider 境界管理
- API/WebUI 提供
- ランタイム可観測性と障害通知

## 5. 実装配置ルール

- 実行コード:
  - 当面は `server/internal/providers` に集約する。
- 共有資産:
  - トップレベル `providers/` は provider 仕様、契約、fixture の置き場とする。

この分離により、将来 provider を別プロセス化する場合でも、現行 server 実装を壊さずに移行できる。
