/**
 * @file events.h
 * @brief WebSocket イベントタイプ定数定義
 * 
 * protocol/websocket/events.md で定義されたイベント種別文字列を集約します。
 * 文字列リテラルをコード内で直書きせず、このファイルの定数を使ってください。
 */
#pragma once

namespace Protocol {

namespace EventType {

// ── Firmware → Server ─────────────────────────────────────────────────
/// セッション開始要求とデバイス情報通知
constexpr const char* SESSION_HELLO      = "session.hello";
/// キープアライブ送信（heartbeat_interval_ms 毎）
constexpr const char* HEARTBEAT          = "heartbeat";
/// バイナリフレームのコーデック・フォーマットを事前登録する
constexpr const char* AUDIO_STREAM_OPEN  = "audio.stream_open";
/// 音声フレームメタデータ（JSON/base64 形式）
constexpr const char* AUDIO_CHUNK        = "audio.chunk";
/// 音声ストリームの終端通知
constexpr const char* AUDIO_END          = "audio.end";
/// 音声ストリームの途中中断通知
constexpr const char* AUDIO_STREAM_ABORT = "audio.stream_abort";
/// 会話ターンの中断通知
constexpr const char* CONVERSATION_CANCEL = "conversation.cancel";
/// TTS バッファ watermark 状態変化通知（P8-19）
constexpr const char* TTS_BUFFER_WATERMARK = "tts.buffer.watermark";

// ── Server → Firmware ─────────────────────────────────────────────────
/// セッション確立通知（heartbeat_interval_ms を含む）
constexpr const char* SESSION_WELCOME    = "session.welcome";
/// STT 処理完了：認識テキストを通知する
constexpr const char* STT_FINAL          = "stt.final";
/// TTS 音声チャンクを通知する
constexpr const char* TTS_CHUNK          = "tts.chunk";
/// TTS 合成完了：音声データと再生メタデータを通知する
constexpr const char* TTS_END            = "tts.end";
/// 再生中 TTS の即時停止を指示する
constexpr const char* TTS_STOP           = "tts.stop";
/// 表情状態を更新する
constexpr const char* AVATAR_EXPRESSION  = "avatar.expression";
/// モーション再生を指示する
constexpr const char* MOTION_PLAY        = "motion.play";
/// エラー通知（双方向）
constexpr const char* ERROR_EVENT        = "error";

// ── Server → Firmware （ハードウェア制御、P11-05） ───────────────────────────
/// サーボ Y/X 軸を指定論理角度へ移動する（firmware が校正値を適用）
constexpr const char* DEVICE_SERVO_MOVE              = "device.servo.move";
/// 現在のサーボ校正値を要求する（firmware が calibration.response を返す）
constexpr const char* DEVICE_SERVO_CALIBRATION_GET   = "device.servo.calibration.get";
/// 校正値を差分更新し不揮発ストレージへ保存する
constexpr const char* DEVICE_SERVO_CALIBRATION_SET   = "device.servo.calibration.set";

// ── Firmware → Server （ハードウェア応答、P11-05） ──────────────────────
/// calibration.get への応答（校正値 + 現在角度）
constexpr const char* DEVICE_SERVO_CALIBRATION_RESPONSE = "device.servo.calibration.response";
/// camera.capture への応答（撮影結果メタデータ）
constexpr const char* DEVICE_CAMERA_CAPTURE_RESULT = "device.camera.capture.result";

// ── Server → Firmware （LED/NeoPixel 制御、P11-06） ─────────────────────
/// M5GO Bottom3 の RGB LED を制御する（必須ハードウェア）
constexpr const char* DEVICE_LED_SET  = "device.led.set";
/// NECO MIMI（NeoPixel）を制御する（オプションハードウェア：未接続時は警告ログのみ）
constexpr const char* DEVICE_EARS_SET = "device.ears.set";

// ── Server → Firmware （診断系制御、P11-07） ───────────────────────────
/// スピーカーテストトーン再生を指示する
constexpr const char* DEVICE_AUDIO_TEST_PLAY = "device.audio.test.play";
/// マイクテスト収音の開始を指示する
constexpr const char* DEVICE_MIC_TEST_START  = "device.mic.test.start";
/// カメラ静止画取得を指示する
constexpr const char* DEVICE_CAMERA_CAPTURE  = "device.camera.capture";

// ── Bidirectional （ハードウェア診断状態、P11-10） ─────────────────────
/// ハードウェア診断状態を要求・通知する（server->firmware 要求 / firmware->server レポート）
constexpr const char* DEVICE_STATE_REPORT = "device.state.report";

}  // namespace EventType

/// このファームウェアが使用するプロトコルバージョン（events.md と一致させること）
constexpr const char* VERSION = "1.0";

}  // namespace Protocol
