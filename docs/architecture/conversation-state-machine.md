# Conversation State Machine

## 1. 目的

会話体験の一貫性を保つため、firmware と server の状態遷移を明示します。
特に発話中の見た目（口パク/表情/モーション）と割り込み時の停止動作を固定します。

## 2. 主状態

- `idle`
  - 待機中。表情は neutral。
- `listening`
  - 収音中。必要に応じて listening 表情へ遷移。
- `thinking`
  - STT/LLM/TTS 処理中。通信待ち表情へ遷移。
- `speaking`
  - TTS 再生中。口パクと発話表情を有効化。
- `interrupted`
  - 割り込み停止処理中。
- `error`
  - 障害状態。エラー表情と復旧導線を表示。

## 3. 基本遷移

- `idle -> listening`
  - トリガ: ユーザー入力開始（ボタン/音声開始）
- `listening -> thinking`
  - トリガ: `audio.end` 送信完了
- `thinking -> speaking`
  - トリガ: `tts.end` 受信
- `speaking -> idle`
  - トリガ: 再生完了
- `* -> interrupted`
  - トリガ: 割り込み要求（cancel/stop）
- `interrupted -> idle`
  - トリガ: 停止処理完了
- `* -> error`
  - トリガ: 復帰不能エラー
- `error -> idle`
  - トリガ: 再接続または手動リカバリ成功

## 4. 体験ルール

- speaking 開始前に表情を先に反映し、再生開始遅延が見えないようにする。
- speaking 中は口パクを継続し、再生終了で必ず neutral へ戻す。
- thinking が長引く場合は待機用モーションを再生する。
- error では最小の通知を表示し、再接続試行中であることを明示する。

## 5. 割り込み（先行仕様）

- `conversation.cancel`
  - 会話リクエストを破棄し、以降の生成結果を返さない。
- `tts.stop`
  - 再生中 TTS を停止し、口パクを即時停止する。
- `audio.stream_abort`
  - 収音中ストリームを破棄し、次回入力を受け付ける。
