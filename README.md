# Stackchan Rebuild

ちいさなボディに、でっかい未来。
このリポジトリは、Stackchan をゼロベースで再構築していくためのホームです。

かわいく、しゃべって、うなずいて、つながっていく。
そんな「一緒に暮らしたくなるロボット体験」を、長く育てられる設計で作っていきます。

## このプロジェクトで大事にしていること

- Protocol First
  - 先に通信契約を決めて、firmware と server の手戻りを減らします。
- Thin Vertical Slices
  - 小さく動く縦スライスを積み重ねて、着実に前進します。
- Interface-First Provider Design
  - STT、LLM、TTS は差し替え可能にして、将来の選択肢を残します。
- Observability from the Start
  - 最初からログ・メトリクス・相関 ID を扱って、運用で困らない土台にします。

## いま目指している体験

- 音声で自然に会話できる
- 表情やリップシンクで「話している感じ」が出る
- 接続が切れても、しなやかに再接続できる
- WebUI で設定・診断・疎通確認ができる

## ディレクトリ構成

- `firmware`: M5Stack 実行時環境とデバイス I/O
- `server`: Go + Gin の API/WebSocket サーバー
- `protocol`: WebSocket プロトコル契約とスキーマ
- `providers`: STT/LLM/TTS アダプタ実装
- `infra`: Docker と運用構成
- `tools`: 開発支援ツール
- `examples`: 最小構成のサンプル
- `docs`: 設計、計画、運用ドキュメント

## クイックスタート

1. リポジトリ構成を確認する
   - `Get-ChildItem -Name`
2. server 用設定を作る
   - `Copy-Item .env.example .env`
3. firmware 用設定を作る
   - `Copy-Item firmware/platformio.ini.local.example firmware/platformio.ini.local`
   - `Copy-Item firmware/include/secrets.example.h firmware/include/secrets.h`
4. 機密情報の扱いを確認する
   - `Get-Content docs/project/secrets-operations.md`

## mise タスク（任意）

長い PlatformIO コマンドを覚えなくても実行できるように、`mise` タスクを用意しています。
`mise` コマンドはリポジトリルート（`stackchan/`）で実行してください。

- タスク一覧
  - `mise tasks`
- 開発サーバー起動
  - `mise run server:run`
- サーバーヘルスチェック
  - `mise run server:healthz`
- firmware ビルド
  - `mise run fw:build`
- firmware 書き込み（自動検出ポート）
  - `mise run fw:upload`
- シリアルモニター（自動検出ポート）
  - `mise run fw:monitor`
- 書き込み + モニター（自動検出ポート）
  - `mise run fw:upmon`

## 進捗の見かた

- 実装計画: `Get-Content docs/project/implementation-plan.md`
- フェーズ 0 タスク: `Get-Content docs/project/phase00-tasklist.md`

## ロードマップの雰囲気

現在は基盤づくりのフェーズを進めながら、次を段階的に育てていく予定です。

- protocol v0 の定義と互換性ルール整備
- hello/welcome を起点にしたセッション確立
- audio.chunk / audio.end を扱う最小音声パス
- provider 境界の整備とモック駆動の実装
- WebUI での可視化と診断導線の強化

## 参加してくれる方へ

このプロジェクトは、かわいさと堅牢さを両立するチャレンジです。
小さな改善、ドキュメント修正、アイデア提案、どれも大歓迎です。

まずは `docs/project` を読むところから、一緒にはじめてもらえるとうれしいです。

## 謝辞

このプロジェクトは、先人の素晴らしい取り組みに強く影響を受けています。

- 本家 Stackchan を生み出し、コミュニティを牽引してくださっている ししかわさん
  - GitHub: [stack-chan/stack-chan](https://github.com/stack-chan/stack-chan)
  - X: [@stack_chan](https://x.com/stack_chan)
  - 記事: [Stack-chan: JavaScript driven super kawaii robot](https://hackaday.io/project/181344-stack-chan-javascript-driven-super-kawaii-robot)
  - 感謝のことば: Stackchan の原点となる発想と実装、そしてコミュニティへの継続的な発信が、このプロジェクトの出発点になっています。
- Stackchan の土台となるデバイス/エコシステムを提供してくださっている M5Stack社
  - 公式サイト: [m5stack.com](https://m5stack.com/)
  - GitHub: [m5stack](https://github.com/m5stack)
  - 感謝のことば: M5Stack シリーズと周辺エコシステムが、Stackchan の開発と検証を進める実装基盤として大きく貢献しています。

あわせて、以下の関連リポジトリとメンテナの皆さまにも深く感謝します。

- stackchan-atama
  - リポジトリ: [karaage0703/stackchan-atama](https://github.com/karaage0703/stackchan-atama)
  - メンテナ: karaage0703さん
  - 感謝のことば: Arduino フレームワークでの実装資産が、派生開発や比較検証の土台として大きな助けになっています。
- m5stack-avatar
  - リポジトリ: [stack-chan/m5stack-avatar](https://github.com/stack-chan/m5stack-avatar)
  - メンテナ: mongonta0716さんをはじめとするメンテナの皆さま
  - 感謝のことば: アバター描画ライブラリとして、表情表現や顔表示まわりの設計・実装を進める上で重要な基盤になっています。
- AI_StackChan_Ex
  - リポジトリ: [ronron-gh/AI_StackChan_Ex](https://github.com/ronron-gh/AI_StackChan_Ex)
  - メンテナ: robo8080さん（原点となる実装と知見共有）、ronron-ghさん（機能拡張と継続メンテナンス）
  - 感謝のことば: AI サービス連携、YAML ベース設定、Realtime API 活用など、実運用に近い知見が本プロジェクトの設計検討に大きく寄与しています。

あらためて、心から感謝します。
