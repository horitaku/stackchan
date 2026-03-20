# フェーズ 11 タスクリスト（Firmware ハードウェア制御と診断導線）

## 1. このドキュメントの目的

フェーズ 11 を、既存の Protocol First / Thin Vertical Slices 方針を維持したまま実行可能なタスクへ分解します。
本フェーズは「firmware の責務分離」「制御イベントの契約化」「WebUI 診断導線の拡張」を主眼とし、ゼロベース実装ではなく既存の session / protocol / WebUI テスト導線をハードウェア制御へ広げます。

フェーズ 11 の主眼は以下です：
- firmware 内のハードウェア責務をサービスへ分離し、StackchanSession の肥大化を防ぐ
- hardware control と avatar behavior を分離した protocol / runtime 境界を固定する
- WebUI を Hardware Test 中心の診断コンソールとして拡張する
- server 経由の test API で、UI -> server -> Stackchan の既存導線を再利用する
- サーボ校正・状態レポート・診断ログまで含めて、安全に運用できる土台を作る

## 2. フェーズ 11 で固定する設計方針

### 2.1 3 層の責務分離

1. デバイス抽象化
   - マイク、スピーカー、カメラ、タッチ、LED、サーボ、耳 NeoPixel を session.cpp から直接操作しない
   - runtime / app 配下のサービスへ分離し、個別に初期化・制御・状態取得を担わせる
2. 制御プロトコル
   - WebUI から送るハードウェア操作は既存の WebSocket イベント体系に沿って追加する
   - 一時操作と永続校正を別イベントとして定義し、後方互換と拡張性を維持する
3. WebUI テスト導線
   - 本番設定画面へ一気に寄せず、まずは「押したらその場で試せる」診断コンソールを育てる
   - 既存の Voicevox テスト導線と同じく、server の API 経由で接続中セッションへ中継する

### 2.2 low-level 制御と high-level 演出の分離

- device.servo.move は低レベル制御として扱い、診断・校正・安全制限の対象にする
- motion.play は高レベル動作として扱い、うなずき・見上げるなどの演出に限定する
- low-level と high-level を混ぜず、WebUI では診断しやすく、会話中は演出しやすい構成を維持する

### 2.3 サーボ制御の安全モデル

サーボは raw angle 直指定を前提にせず、次の校正情報を firmware 側に保持する。

- center_offset_deg
- min_deg
- max_deg
- invert
- speed_limit_deg_per_sec
- soft_start
- home_x_deg
- home_y_deg

- WebUI は論理角度を送信し、firmware が校正値を適用して実角度へ変換する
- 一時的な手動移動と、不揮発に保持する校正値保存をイベントレベルで分ける
- 初期フェーズでは「壊さない」「戻せる」「保存できる」を優先し、自律動作は後段へ回す

## 3. 実装スコープ

### 3.1 firmware の責務分解

初手では StackchanSession にハードウェア制御を足し込まず、次の単位でサービス化する。

```txt
firmware/runtime/
  actuators/
    servo_controller.*
  lighting/
    base_led_controller.*
    ear_neopixel_controller.*
  input/
    touch_service.*
  audio/
    mic_reader.*      # 既存活用
    tts_player.*      # 既存活用
  vision/
    camera_service.*
```

StackchanSession の責務は以下へ寄せる。

- 受信イベントのルーティング
- service 初期化と依存の束ね込み
- 状態レポート送信
- heartbeat / session lifecycle の維持

### 3.2 追加する最小制御イベント

最初に追加するイベントは以下の最小セットとする。

- device.led.set
- device.ears.set
- device.servo.move
- device.servo.calibration.get
- device.servo.calibration.set
- device.audio.test.play
- device.mic.test.start
- device.camera.capture
- device.state.report

補足方針：

- 校正系イベントは move 系と分離し、値の保存責務を明確化する
- camera は最初からストリーミングを狙わず、静止画取得を最小縦スライスにする
- state.report は heartbeat と競合させず、診断向け payload を独立させる

### 3.3 server と WebUI の拡張方針

WebUI から firmware へ直接 WebSocket 接続は行わず、server に次のテスト API を追加する。

- POST /api/tests/hardware/servo
- POST /api/tests/hardware/led
- POST /api/tests/hardware/ears
- POST /api/tests/hardware/audio/play
- POST /api/tests/hardware/mic/start
- POST /api/tests/hardware/camera/capture
- GET /api/tests/hardware/state

server の責務：

- 接続中 Stackchan セッションの選択
- protocol event 生成と送信
- 応答結果・タイムアウト・未接続時エラーの標準化
- WebUI で扱いやすい診断結果 JSON への変換

### 3.4 Hardware Test 画面の初期パネル

WebUI には少なくとも次の 4 パネルを追加する。

1. Touch / Mic / Speaker テスト
   - タッチ状態表示
   - マイク入力レベル表示
   - テストトーン再生
2. Servo テスト
   - X/Y スライダー
   - center offset 調整
   - home へ戻す
   - 保存 / 読み出し
3. LED テスト
   - M5GO Bottom3 LED の ON/OFF・明るさ・色
   - 耳 NeoPixel の色・輝度・パターン
4. Camera テスト
   - 静止画取得
   - 解像度切替
   - 最終撮影時刻表示

## 4. 実行タスクリスト

| ID | タスク | 成果物 | 優先度 | 理由 | ステータス |
| --- | --- | --- | --- | --- | --- |
| P11-01 | firmware ハードウェア責務の棚卸しと境界定義 | docs 追記、責務一覧、既存 StackchanSession 依存マップ | 高 | いきなり実装分割すると責務漏れや循環参照が起きやすいため | 完了 |
| P11-02 | ServoController サービスを追加 | firmware/runtime/actuators/servo_controller.*、インターフェース、初期化方針 | 高 | サーボ校正と安全制御が以後の診断導線の中核になるため | 完了 |
| P11-03 | LED / Ear NeoPixel サービスを追加 | firmware/runtime/lighting/base_led_controller.*、ear_neopixel_controller.* | 高 | 視覚的フィードバックが早く、Hardware Test の価値をすぐ出せるため | 完了 |
| P11-04 | Touch / Camera サービス境界を追加 | firmware/runtime/input/touch_service.*、vision/camera_service.* | 中 | タッチ反映と静止画取得の入口を session から分離するため | 未着手 |
| P11-05 | device.servo 系 protocol を追加 | schema、examples、events.md、validation checklist 更新 | 高 | move と calibration を先に契約化しないと UI / server / firmware がずれるため | 完了 |
| P11-06 | device.led.set と device.ears.set を追加 | schema、examples、server/firmware 受信テスト | 高 | LED 系は安全かつ即効性の高い最初の制御対象であるため | 完了 |
| P11-07 | audio / mic / camera / state report イベントを追加 | schema、examples、互換性メモ | 中 | 診断導線を広げる前に最小イベント集合を揃える必要があるため | 完了 |
| P11-08 | StackchanSession に hardware command router を追加 | 受信ディスパッチ整理、各サービスへの委譲、error handling | 高 | session の肥大化を抑えつつ、既存接続フローへ安全に統合するため | 完了 |
| P11-09 | サーボ校正ストアを実装 | calibration モデル、不揮発保存、read/write API | 高 | 個体差調整を後回しにすると以後の servo UI が危険になるため | 完了 |
| P11-10 | device.state.report を firmware から送信 | RSSI / free heap / current angle / calibration / mic level / speaker busy / camera available / firmware version | 高 | WebUI の診断精度と運用時の切り分けを大きく改善するため | 完了 |
| P11-11 | server に hardware test API を追加 | /api/tests/hardware/*、session bridge、timeout handling、test | 高 | WebUI から firmware へ直接つながず既存の運用導線を流用するため | 完了 |
| P11-12 | WebUI Hardware Test 画面を追加 | Tests セクション拡張、Servo/LED/Audio/Camera パネル | 高 | 「押したら試せる」導線がこのフェーズのユーザー価値そのもののため | 完了 |
| P11-13 | Hardware Overview を追加 | device.state.report 可視化、最終更新時刻、未接続表示 | 中 | 状態確認と操作画面を分けることで運用時の見通しを良くするため | 完了 |
| P11-14 | 診断ログとランブックを追加 | runbook、操作手順、失敗時の見方、構造化ログ項目 | 中 | 実機差分や接続失敗時の MTTR を短縮するため | 完了 |

## 5. 優先順位と薄い縦スライス

### 5.1 フェーズ A: すぐ価値が出るもの

最優先は以下とする。

- サーボ X/Y 手動制御
- サーボ校正値の読み出し・保存
- M5GO Bottom3 LED 制御
- 耳 NeoPixel 制御
- WebUI Hardware Test 画面の最小導線

狙い：

- 動作確認の手応えが強い
- 校正基盤が以後の motion / expression 拡張の土台になる
- UI から hardware を動かす縦スライスを早期に閉じられる

### 5.2 フェーズ B: 音と入力

- speaker test 再生
- mic level meter
- touch event の WebUI 反映
- ローカルテストアクションとの接続

狙い：

- 既存の音声パイプラインと自然に接続する
- 「入出力が生きているか」を素早く切り分けできるようにする

### 5.3 フェーズ C: カメラ

- 静止画取得
- 解像度・品質設定
- WebUI プレビュー
- 将来の顔追跡用インターフェース定義

狙い：

- 最初は「撮れる」「見える」に限定して複雑さを抑える
- 顔追跡や視線制御は後続フェーズへ安全に接続する

## 6. 週次の実装イメージ

### 第 1 週

- ServoController 作成
- device.servo.move
- device.servo.calibration.get / set
- WebUI のサーボスライダー画面

### 第 2 週

- Bottom LED / 耳 NeoPixel 制御
- device.led.set
- device.ears.set
- 明るさ・色・パターン UI

### 第 3 週

- device.state.report
- Hardware Overview 画面
- 診断ログ追加

### 第 4 週

- speaker test / mic level meter
- device.audio.test.play
- device.mic.test.start

### 第 5 週

- camera capture
- プレビュー UI
- 顔追跡用インターフェース定義

## 7. 受け入れ方針（フェーズ 11）

- WebUI から接続中 Stackchan を選び、サーボ X/Y を安全制限付きで手動操作できること
- サーボ校正値を取得・更新・保存し、再起動後も復元できること
- LED と耳 NeoPixel を WebUI から色・輝度つきで制御できること
- firmware が device.state.report を送信し、WebUI に現在値が反映されること
- 未接続時やタイムアウト時に、server の hardware test API が一貫したエラー形式を返すこと
- low-level hardware control と high-level motion.play が protocol / 実装上で分離されていること

## 8. 後続フェーズへの引き継ぎ

- Thinking / Speaking と連動した LED 演出
- タッチ開始 / 長押しキャンセルの会話操作
- マイクレベル連動の口パク・反応演出
- 顔検知や人検知とサーボ追従の統合
- 自己診断モードによる一括ヘルスチェック
- 機体ごとの校正値・LED プリセット管理

---

本ドキュメントは初版です。実装が進んだら、実際の firmware 構成、protocol event 名、WebUI 画面構成、運用手順に合わせて更新してください。