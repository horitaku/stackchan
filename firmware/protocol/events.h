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

}  // namespace EventType

/// このファームウェアが使用するプロトコルバージョン（events.md と一致させること）
constexpr const char* VERSION = "1.0";

}  // namespace Protocol
