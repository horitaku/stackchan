// Package audio は音声フレームのコーデック検証・変換ユーティリティを提供します。
//
// P8-05: WebSocket binary Opus パスの統合
// - binary フレームのコーデック検証と期待サイズ計算
// - PCM / Opus 双方に対応した検証ヘルパー
// - Opus OGG コンテナ構築（STT API 用）
// - WAV コンテナ構築（PCM フレームの STT API 用ラッパー）
package audio

import "fmt"

// コーデック識別子の定数です。
// BinaryStreamMeta.Codec および AudioChunk.Codec と一致させてください。
const (
	// CodecPCM は 16-bit signed little-endian PCM を示します。
	CodecPCM = "pcm"
	// CodecOpus は raw Opus フレーム（OGG コンテナなし）を示します。
	CodecOpus = "opus"
)

// PCM16BytesPerSample は 16-bit PCM のサンプルあたりバイト数です。
const PCM16BytesPerSample = 2

// ExpectedPCMFrameBytes は PCM フレームの期待バイト数を計算して返します。
// codec が "pcm" 以外、またはパラメータが不正の場合は -1 を返します。
//
// 計算式: sampleRateHz × frameDurationMs(秒) × channelCount × PCM16BytesPerSample
// 例: 16000Hz × 20ms × 1ch × 2 = 640 bytes
func ExpectedPCMFrameBytes(codec string, sampleRateHz, frameDurationMs, channelCount int) int {
	if codec != CodecPCM {
		return -1
	}
	if sampleRateHz <= 0 || frameDurationMs <= 0 || channelCount <= 0 {
		return -1
	}
	return sampleRateHz * frameDurationMs / 1000 * channelCount * PCM16BytesPerSample
}

// ValidateCodec は codec が対応済みかどうかを検証します。
// 対応コーデック: "pcm", "opus"
func ValidateCodec(codec string) error {
	switch codec {
	case CodecPCM, CodecOpus:
		return nil
	default:
		return fmt.Errorf("unsupported codec: %q (accepted: pcm, opus)", codec)
	}
}

// ValidateFramePayload は受信フレームのペイロードサイズを検証します。
//
// PCM の場合: expectedBytes と実際のバイト数が一致するかを確認します。
// Opus の場合: 最小バイト数（4 bytes = TOC + 最低 1 Opus フレーム）のみ確認します。
//
// ok=false の場合、warning に不一致の詳細が含まれます。
func ValidateFramePayload(codec string, payloadBytes, expectedBytes int) (ok bool, warning string) {
	switch codec {
	case CodecPCM:
		if expectedBytes <= 0 {
			// メタ情報が不足のため検証をスキップします
			return true, ""
		}
		if payloadBytes != expectedBytes {
			return false, fmt.Sprintf(
				"pcm frame size mismatch: received %d bytes, expected %d bytes",
				payloadBytes, expectedBytes)
		}
		return true, ""

	case CodecOpus:
		// Opus フレームは可変長（典型的: 10〜1500 bytes 程度）。
		// TOC byte が最低 1 byte 存在することのみ確認します。
		if payloadBytes < 4 {
			return false, fmt.Sprintf("opus frame suspiciously small: %d bytes (minimum expected: 4)", payloadBytes)
		}
		return true, ""

	default:
		return false, fmt.Sprintf("unknown codec %q: cannot validate frame size", codec)
	}
}

// Opus48HzSamplesPerFrame は OGG/Opus の granule_position 計算用に
// 指定フレーム長(ms)に対応する 48kHz 単位のサンプル数を返します。
//
// OGG/Opus では実際のサンプルレートに関わらず、granule_position は常に 48kHz 単位です。
// （RFC 7845 §5.1 参照）
//
// 例:
//   - 20ms → 48000 × 20/1000 = 960 samples
//   - 10ms → 48000 × 10/1000 = 480 samples
func Opus48HzSamplesPerFrame(frameDurationMs int) int64 {
	return int64(48000) * int64(frameDurationMs) / 1000
}
