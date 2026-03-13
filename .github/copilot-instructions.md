# Stackchan Rebuild Project: Copilot Instructions

## 1. このプロジェクトの目的

このリポジトリは、Stack-chan をゼロベースで再構築するためのプロジェクトです。  
機能追加よりも先に、長期運用しやすい構成、明確な責務分離、段階的に拡張できる設計を重視してください。

この指示書は、GitHub Copilot がコード提案やリファクタリングを行う際の判断基準です。

## 2. 現時点の想定アーキテクチャ

### 2.1 デバイス側（Stackchan 本体）

- M5Stack CoreS3
- タッチスクリーン
- マイクロSDカード
- マイク
- スピーカー
- 内蔵カメラ
- M5Go Bottom3
- NECO MIMI（NeoPixel）
- サーボモーター（X軸, Y軸）

### 2.2 バックエンド側

- 起動方式: docker-compose
- メイン言語: Go
- Web フレームワーク: Gin
- データベース: PostgreSQL（docker-compose で同梱）
- AI 関連コンポーネント
	- LLM: OpenAI API
	- STT: OpenAI API
	- TTS: Voicevox

### 2.3 バックエンド WebUI

- WebUI は Svelte を採用し、ビルド成果物は静的ファイルとして配信する
- 配信は Go の Web サービスが担当し、API/WebSocket と同一プロセスで提供する
- WebUI の目的
  - 各種設定の変更（音声、接続先、デバイス動作、デバッグ設定）
  - 疎通確認と機能テスト（STT/LLM/TTS 単体テスト、音声入出力テスト）
  - ランタイム状態の可視化（接続状態、キュー滞留、遅延、エラー）

## 3. 通信方式の基本方針

### 3.1 デバイス - バックエンド間通信

- 通信方式は WebSocket を第一候補として設計する
- 低遅延と双方向性を優先し、以下を満たすこと
	- 音声ストリームの継続送受信
	- 制御イベント（サーボ、表情、LED、状態通知）の即時反映
	- 再接続とセッション復帰

### 3.2 メッセージ設計指針

- バイナリとテキストを用途で分離する
	- バイナリ: 音声フレーム（Opus）
	- JSON: 制御コマンド、メタデータ、状態通知
- すべてのメッセージに最小限の共通フィールドを持たせる
	- type
	- timestamp
	- session_id
	- sequence
- 後方互換性のため version フィールドを検討する

## 4. 音声パイプライン方針

### 4.1 品質重視の音声戦略

- 音声入出力は Opus エンコード/デコードを導入し、高品質と低帯域を両立する
- 目標は「自然な会話体験」であり、以下を継続的に最適化する
	- レイテンシ（録音開始から再生開始まで）
	- 音質（ノイズ、破綻、クリッピングの抑制）
	- 発話の途切れ/詰まりの抑制

### 4.2 推奨データフロー（初期案）

1. デバイスで収音
2. Opus でフレーム化して WebSocket 送信
3. バックエンドでデコードまたは STT 入力向け変換
4. STT（OpenAI API）
5. LLM（OpenAI API）
6. TTS（Voicevox）
7. デバイスへ音声データとリップシンク用情報を送信
8. デバイスで再生と口パク同期

## 5. アバター表示とリップシンク

- 画面描画は m5stack-avatar をベースに実装する
- 「話している感じ」を出すため、最低限以下を同期する
	- 音声再生状態
	- 口形状の変化（音量または簡易音素ベース）
	- まばたき、視線、表情の状態管理
- 将来拡張を見越し、リップシンク信号は抽象化レイヤで扱う

## 6. 参照実装の扱い

- 参考リポジトリ
	- stackchan-atama
	- m5stack-avatar
	- AI_StackChan_Ex
- 方針
	- 設計意図を学ぶために参照する
	- 既存コードをそのまま複製しない
	- このプロジェクトの責務分離と拡張性を優先する

### 6.1 AI_StackChan_Ex から取り入れる観点

- 低遅延化の観点として、将来は Realtime API 相当の音声入出力直結モードを検討する
- 機能を段階導入できるよう、ハードウェア依存機能（カメラ、顔検出、ウェイクワード等）は feature flag で制御する
- 運用時の切り分けを容易にするため、状態可視化（ステータスモニタ）を WebUI の標準機能として重視する
- ユーザー拡張をしやすくするため、将来のアプリ追加を見据えたモジュール境界を維持する

## 7. Go バックエンド実装ガイド

### 7.1 設計原則

- クリーンアーキテクチャを過度に厳密化しすぎず、責務分離を明確にする
- HTTP/API サーバー実装は Gin を前提とする
- I/O 境界を明示する
	- WebSocket ハンドラ
	- 音声処理（Opus）
	- STT/LLM/TTS クライアント
	- セッション/状態管理
- 依存注入を行い、外部 API をモック可能にする

### 7.2 エラーハンドリング

- ネットワーク断、API 制限、タイムアウトを前提に実装する
- リトライは指数バックオフを基本とし、無限リトライを避ける
- 利用者に影響する障害はイベントとしてデバイスへ通知する

### 7.3 ログと観測性

- 構造化ログを採用する（JSON 形式を推奨）
- 相関 ID（session_id, request_id）を全レイヤで引き回す
- 最低限の観測項目
	- STT/LLM/TTS レイテンシ
	- WebSocket 切断回数
	- 音声キュー滞留時間

## 8. プロジェクト構造（レイヤー＋ランタイム分離）

このプロジェクトは単一アプリではなく、複数ランタイムで構成されるプラットフォームです。  
一般的な `src/` 中心構成ではなく、ランタイム単位 + 契約単位で分離する。

- 推奨トップレベル構造

```txt
repo/
├ firmware/                # M5Stack / Stack-chan runtime
├ server/                  # Go + Gin AI server
├ protocol/                # WebSocket protocol contract
├ providers/               # STT / LLM / TTS adapter
├ tools/                   # 開発支援ツール
├ infra/                   # docker / deploy
├ docs/
├ examples/
└ .github/                 # Copilot / CI / automation
```

- 設計原則
  - ランタイム境界（firmware/server）と契約境界（protocol）を分離する
  - provider 実装を分離してベンダーロックを避ける
  - 拡張時に既存ランタイムを壊さない構造を優先する

### 8.1 firmware

```txt
firmware/
├ runtime/
│  ├ audio/
│  ├ display/
│  ├ motion/
│  ├ input/
│  └ network/
├ boards/
│  ├ core2/
│  └ cores3/
├ protocol/
├ app/
│  └ stackchan/
└ main.cpp
```

- 役割
  - デバイス制御（マイク、スピーカー、LCD、サーボ、タッチ）
  - WebSocket クライアント
- 重要ルール
  - firmware に AI オーケストレーションロジックを持ち込まない

### 8.2 server

```txt
server/
├ cmd/
│  └ stackchan-server/
├ internal/
│  ├ session/
│  ├ conversation/
│  ├ audio/
│  ├ providers/
│  ├ tools/
│  ├ memory/
│  ├ persona/
│  ├ protocol/
│  └ web/
└ pkg/
```

- 役割
  - AI orchestration（session / STT / LLM / TTS / tools / memory）
  - API/WebSocket 提供
  - Svelte の static build 配信

### 8.3 protocol（重要）

```txt
protocol/
├ websocket/
│  ├ events.md
│  └ schemas/
├ versioning.md
└ examples/
```

- WebSocket protocol を API 契約として先に定義する
- イベント例
  - session.hello
  - audio.chunk
  - audio.end
  - stt.partial
  - stt.final
  - llm.started
  - tts.chunk
  - tts.end
  - avatar.expression
  - motion.play
  - conversation.cancel

### 8.4 providers

```txt
providers/
├ stt/
│  ├ openai/
│  └ whisper/
├ llm/
│  ├ openai/
│  ├ gemini/
│  └ ollama/
└ tts/
   ├ openai/
   ├ voicevox/
   └ elevenlabs/
```

- メリット
  - provider の追加が容易
  - vendor lock の回避
  - モックと差し替えテストが容易

### 8.5 tools

```txt
tools/
├ protocol-validator/
├ session-replay/
├ log-viewer/
└ event-generator/
```

- 例
  - `stackchan replay session.json` のような会話再現ツールを提供する

### 8.6 infra

```txt
infra/
├ docker/
├ dev/
├ production/
└ terraform/
```

- docker-compose、開発環境、デプロイ定義を配置する
- PostgreSQL は docker-compose で Go サービスと同梱して起動する
- DB 接続情報は環境変数で管理し、リポジトリにパスワード等を含めない
- マイグレーション管理ツール（例: golang-migrate）を採用し、スキーマ変更を追跡する

### 8.7 docs / examples / .github

```txt
docs/
├ architecture/
├ decisions/
├ project/
└ protocol/

examples/
├ minimal-client/
├ simple-stackchan/
└ test-bot/
```

- `.github/` には Copilot 指示、CI、自動化ルールを配置する

### 8.8 WebUI 運用ルール

- 開発時は Svelte 開発サーバーと Go バックエンドを分離起動してよい
- 本番相当では Svelte を static build し、Go が `webui/dist` を配信する
- 設定値は API 経由で保存・更新し、UI から直接ファイル書き換えしない
- UI からテスト実行した結果（成功/失敗、レイテンシ、エラー）は構造化して保持する

### 8.9 開発順序（Protocol First）

破綻しにくい実装順序は次のとおり。

1. protocol 設計
2. server 実装
3. firmware 実装

## 9. テスト戦略

- 重点
	- WebSocket プロトコル整合性
	- 音声フレーム連結処理の健全性
	- 外部 API 異常時のフォールバック
- テストレベル
	- 単体テスト: 変換ロジック、メッセージ検証、状態遷移
	- 結合テスト: WebSocket と音声パイプライン
	- UI 結合テスト: WebUI から設定更新と疎通テストを実行できること
	- 手動確認: 実機でのリップシンク、発話遅延、モーター連動

## 10. Copilot への具体的な指示

コード提案時は、以下を優先してください。

1. まずは動く最小構成を実装し、その後に拡張しやすい形へ整理する
2. WebSocket メッセージは型安全とバージョニングを考慮して定義する
3. Opus を使う音声経路では、サンプルレート/チャネル数/フレーム長を明示する
4. OpenAI/Voicevox クライアントはインターフェース化し、モック可能にする
5. タイムアウト、キャンセル、再接続を必ず設計に含める
6. 提案コードには、処理の意図が分かる簡潔なコメントを付ける
7. 破壊的変更を提案する場合は、移行手順を必ず示す
8. WebUI は「Svelte static build を Go で配信」の構成を前提に提案する
9. WebUI の設定操作とテスト実行は API 化し、画面と処理責務を分離する

## 11. 今後の拡張候補（メモ）

- VAD（Voice Activity Detection）導入
- ノイズ抑制とエコーキャンセル
- リップシンクを音素ベースへ高精度化
- カメラ入力を使った視線・表情インタラクション
- オフライン時の限定機能モード
- Realtime API 相当の低遅延会話モード
- ローカル推論モード（クラウド API 非依存）の検証

---

このファイルは初版です。実装が進んだら、実際のディレクトリ構成、プロトコル定義、運用ポリシーに合わせて更新してください。
