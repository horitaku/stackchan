# firmware ハードウェア責務マップ（P11-01）

## 1. このドキュメントの目的

P11-01 の成果物として、フェーズ 11 開始時点における firmware の責務現状を整理します。  
「いきなり実装分割すると責務漏れや循環参照が起きやすい」という理由から、実装前に境界を文書化します。

- 既存 StackchanSession の依存マップを可視化する
- ハードウェア別の責務を列挙し、現状の問題点を明示する
- フェーズ 11 で行う分離の境界線を確定する

---

## 2. 現状のファイル構成

```txt
firmware/
├ main.cpp                          # エントリーポイント
├ app/stackchan/
│  ├ session.h                      # StackchanSession クラス定義（神クラス化の兆候あり）
│  ├ session.cpp                    # begin() / loop() / Wi-Fi 再接続
│  ├ session_connection.cpp         # WebSocket コールバック（onWSConnected 等）
│  ├ session_protocol.cpp           # 送信ヘルパー（sendHello / sendHeartbeat 等）
│  ├ session_avatar.cpp             # Avatar 表情・Motion 演出・会話状態管理
│  ├ session_audio_upload.cpp       # 音声 uplink（audio.stream_open → binary → audio.end）
│  └ session_tts_stream.cpp         # TTS 受信・キュー・Opus デコード・再生
├ runtime/
│  ├ audio/
│  │  ├ mic_reader.h / .cpp         # マイク収音サービス（既存）
│  │  └ tts_player.h / .cpp        # TTS PCM 再生サービス（既存）
│  └ network/
│     ├ wifi.h / .cpp               # Wi-Fi 接続ヘルパー
│     └ ws_client.h / .cpp         # WebSocket クライアント
├ protocol/
│  ├ events.h                       # イベントタイプ定数
│  ├ envelope.h / .cpp              # メッセージエンベロープ生成
└ boards/cores3/
   └ board_config.h / .cpp          # M5Stack CoreS3 固有の初期化
```

---

## 3. StackchanSession 依存マップ（現状）

```
                        ┌─────────────────────────────────────────┐
                        │         StackchanSession                │
                        │                                         │
  Network::WsClient ────┤ _ws         (WebSocket 送受信)          │
  Audio::MicReader ─────┤ _mic        (マイク収音)                │
  Audio::TTSPlayer ─────┤ _ttsPlayer  (TTS PCM 再生)             │
  Protocol::OutboundSeq ┤ _seq        (シーケンス番号管理)         │
  m5avatar::Avatar ─────┤ _avatar     (顔描画 ← M5Stack-Avatar)  │◄── Avatar は
                        │                                         │    ハードウェア
  TTSStreamContext ──────┤ _tts        (フレームキュー・Opus・     │    ではなく
                        │              concealment を集約)         │    演出層
                        │                                         │
                        │  ── ハードウェアで直接触っているもの ──  │
                        │  M5.Touch.getCount()  → main.cpp から  │
                        │  (タッチ検出は session を経由せず main  │
                        │   で直接検出し sendAudioStream() を呼ぶ)│
                        └─────────────────────────────────────────┘
```

### 3.1 依存ファイル一覧

| 依存先 | セッションのどこで使っているか | ハードウェア直接操作か |
|--------|-------------------------------|----------------------|
| `Network::WsClient` | セッション接続・送受信全般 | No（抽象化済み） |
| `Audio::MicReader` | `sendAudioStream()` でフレーム収音 | Yes（I2S マイク） |
| `Audio::TTSPlayer` | `processTTSPlaybackQueue()` で PCM 再生 | Yes（I2S スピーカー） |
| `m5avatar::Avatar` | `begin()`, `updateAvatarFace()` で顔描画 | Yes（LCD） |
| `M5.Touch` | `main.cpp` で直接検出 | Yes（タッチパネル） |
| LED（M5GO Bottom3） | **未実装**（session に存在しない） | — |
| 耳 NeoPixel（NECO MIMI） | **未実装** | — |
| サーボ（X / Y 軸） | **未実装** | — |
| カメラ | **未実装** | — |

---

## 4. ハードウェア別 責務現状

### 4.1 マイク（I2S マイク）

| 項目 | 現状 |
|------|------|
| 実装場所 | `runtime/audio/mic_reader.*` |
| 呼び出し元 | `session_audio_upload.cpp` |
| 状態 | サービス化済み ✅ |
| 問題点 | なし（MicReader は session から分離されている） |

### 4.2 スピーカー（I2S スピーカー）

| 項目 | 現状 |
|------|------|
| 実装場所 | `runtime/audio/tts_player.*` |
| 呼び出し元 | `session_tts_stream.cpp` |
| 状態 | サービス化済み ✅ |
| 問題点 | なし（TTSPlayer は session から分離されている） |

### 4.3 LCD ディスプレイ（アバター描画）

| 項目 | 現状 |
|------|------|
| 実装場所 | `m5avatar::Avatar` を `session.h` のメンバーとして直接保持 |
| 呼び出し元 | `session.cpp` (`begin` / `loop`) / `session_avatar.cpp` |
| 状態 | **session に混在** ⚠️ |
| 問題点 | アバターは演出層（high-level）だが session が直接所有している。分離すると将来の表情制御が整理しやすくなる |

### 4.4 タッチパネル

| 項目 | 現状 |
|------|------|
| 実装場所 | `main.cpp` で `M5.Touch.getCount()` を直接呼び出し |
| 呼び出し元 | `main.cpp` |
| 状態 | **main.cpp に直書き** ⚠️ |
| 問題点 | 「タッチされたら sendAudioStream」のロジックが main.cpp に散らかっている。WebUI 連携や長押し判定を追加するとさらに複雑化する |

### 4.5 LED（M5GO Bottom3）

| 項目 | 現状 |
|------|------|
| 実装場所 | **未実装** |
| 呼び出し元 | — |
| 状態 | **未対応** ❌ |
| 問題点 | 会話状態（Thinking / Speaking）に連動した LED 演出がない。Protocol イベントも未定義 |

### 4.6 耳 NeoPixel（NECO MIMI）

| 項目 | 現状 |
|------|------|
| 実装場所 | **未実装** |
| 呼び出し元 | — |
| 状態 | **未対応** ❌ |
| 問題点 | NeoPixel 制御コードが存在しない |

### 4.7 サーボ（X / Y 軸）

| 項目 | 現状 |
|------|------|
| 実装場所 | **未実装** |
| 呼び出し元 | — |
| 状態 | **未対応** ❌ |
| 問題点 | サーボ制御・校正値保存・安全制限が一切ない。フェーズ 11 で最優先で実装する対象 |

### 4.8 カメラ

| 項目 | 現状 |
|------|------|
| 実装場所 | **未実装** |
| 呼び出し元 | — |
| 状態 | **未対応** ❌ |
| 問題点 | 静止画取得・ストリーミングとも未実装 |

---

## 5. 現状の問題点まとめ

```
┌───────────────────────────────────────────────────────────┐
│  問題 1: session.h がデータ構造の神クラスになりつつある   │
│                                                           │
│  　_ws / _mic / _ttsPlayer / _avatar / _tts（巨大構造体）│
│  　を全部保持しており、責務が 1 ファイルに集まりすぎている│
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│  問題 2: ハードウェア 4 種が未実装で WebUI から制御不能   │
│                                                           │
│  　LED / NeoPixel / サーボ / カメラ のコードが存在しない │
│  　これらを後付けすると session がさらに肥大化する       │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│  問題 3: タッチ検出が main.cpp に直書きで拡張困難        │
│                                                           │
│  　長押し・マルチタッチ・WebUI 反映を追加すると           │
│  　main.cpp が肥大化するか session に移植が必要になる     │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│  問題 4: protocol イベントが未定義                        │
│                                                           │
│  　device.servo.* / device.led.* / device.ears.* /       │
│  　device.state.report 等をどこで受け取るか決まっていない │
└───────────────────────────────────────────────────────────┘
```

---

## 6. フェーズ 11 での分離方針（境界定義）

### 6.1 分離対象と優先度

| ハードウェア | 分離先（予定パス） | 優先度 | 対応 Task |
|------------|-------------------|--------|-----------|
| サーボ（X/Y） | `runtime/actuators/servo_controller.*` | 高 | P11-02, P11-09 |
| LED（Bottom3） | `runtime/lighting/base_led_controller.*` | 高 | P11-03 |
| 耳 NeoPixel | `runtime/lighting/ear_neopixel_controller.*` | 高 | P11-03 |
| タッチパネル | `runtime/input/touch_service.*` | 中 | P11-04 |
| カメラ | `runtime/vision/camera_service.*` | 中 | P11-04 |
| Avatar（演出層） | `app/stackchan/avatar_service.*`（将来） | 低 | フェーズ 12 以降 |

### 6.2 分離後の StackchanSession の責務

分離後、StackchanSession の役割を以下に**限定**します。

```
StackchanSession（分離後の役割）
  ├ WebSocket ライフサイクル管理（接続・再接続・heartbeat）
  ├ 受信イベントのルーティング（どのサービスへ委譲するか決める）
  ├ 各サービスの初期化と依存の束ね込み
  ├ ConversationState の管理と遷移
  └ device.state.report の送信
```

**あくまで「ルーター兼コーディネーター」であり、個別 HW 制御は持たない。**

### 6.3 新設するサービスの責務

| サービス | 責務 |
|---------|------|
| `ServoController` | 論理角度 → 実角度変換・校正値適用・安全制限・不揮発保存 |
| `BaseLEDController` | M5GO Bottom3 の ON/OFF・色・輝度制御 |
| `EarNeoPixelController` | NECO MIMI NeoPixel の色・輝度・パターン制御 |
| `TouchService` | タッチ状態の検出・クリック/長押し判定・イベント通知 |
| `CameraService` | 静止画取得・解像度設定・JPEG エンコード |

### 6.4 protocol イベントの受信フロー（分離後）

```
WebSocket受信
    │
    ▼
StackchanSession::onTextMessage()
    │
    ├─ device.servo.move        → ServoController::move()
    ├─ device.servo.calibration.get → ServoController::getCalibration() → sendResponse
    ├─ device.servo.calibration.set → ServoController::setCalibration()
    ├─ device.led.set           → BaseLEDController::set()
    ├─ device.ears.set          → EarNeoPixelController::set()
    ├─ device.audio.test.play   → TTSPlayer::playTestTone()
    ├─ device.mic.test.start    → MicReader::startLevelMeter()
    ├─ device.camera.capture    → CameraService::capture() → sendResponse
    └─ device.state.report(GET) → 各サービスから状態収集 → sendStateReport()
```

---

## 7. 不変条件（フェーズ 11 中に壊してはいけないもの）

| 項目 | 理由 |
|------|------|
| `session.hello / welcome` フロー | 接続確立の核心。変更すると全テストが壊れる |
| `audio.stream_open → binary → audio.end` フロー | 音声 uplink の基盤 |
| TTS フレームキュー（Producer-Consumer） | P8-17 で整備した遅延最小化設計 |
| `ConversationState` の定義と遷移名 | server 側 protocol と対応している |
| `Protocol::EventType::*` 定数 | events.h を変更すると firmware と server がずれる |

---

## 8. 次のアクション

| 順序 | タスク ID | 作業内容 |
|------|-----------|---------|
| 1 | P11-05 | `device.servo.*` protocol を先に定義（contract first） |
| 2 | P11-06 | `device.led.set` / `device.ears.set` を定義 |
| 3 | P11-02 | `ServoController` 実装 |
| 4 | P11-03 | `BaseLEDController` / `EarNeoPixelController` 実装 |
| 5 | P11-08 | `StackchanSession` に hardware command router 追加 |
| 6 | P11-09 | 校正値の不揮発保存 実装 |

---

*このドキュメントは P11-01 の成果物です。実装が進んだら、実際のファイル構成・イベント名・サービス境界に合わせて更新してください。*
