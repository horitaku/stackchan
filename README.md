# Stackchan Rebuild

Stackchan 再構築プロジェクトのルートリポジトリです。
本リポジトリは Protocol First と Thin Vertical Slices に基づいて、firmware と server を段階的に統合します。

## Top-level Directories

- `firmware`: M5Stack 実行時環境とデバイス I/O
- `server`: Go + Gin の API/WebSocket サーバー
- `protocol`: WebSocket プロトコル契約とスキーマ
- `providers`: STT/LLM/TTS アダプタ実装
- `infra`: Docker と運用構成
- `tools`: 開発支援ツール
- `examples`: 最小構成のサンプル
- `docs`: 設計、計画、運用ドキュメント

## Initial Setup

1. リポジトリの構成を確認します。
   - `Get-ChildItem -Name`
2. server 用設定を作成します。
   - `Copy-Item .env.example .env`
3. firmware 用設定を作成します。
   - `Copy-Item firmware/platformio.ini.example firmware/platformio.ini.local`
   - `Copy-Item firmware/include/secrets.example.h firmware/include/secrets.h`
4. 機密情報の扱いは `docs/project/secrets-operations.md` に従います。

## Verification

- タスク進捗確認: `Get-Content docs/project/phase00-tasklist.md`
- 計画確認: `Get-Content docs/project/implementation-plan.md`
