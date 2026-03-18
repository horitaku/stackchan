// opus_downlink.go は downlink 向けに WAV 音声を Opus パケット列へ変換します。
//
// 方針:
// - 変換自体は ffmpeg(libopus) を利用します（実行時依存）。
// - 生成された OGG/Opus から Opus 音声パケットを抽出し、tts.chunk(v1.1) へ載せます。
// - 変換不可時は呼び出し側で PCM fallback できるようにエラーを返します。
package audio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	defaultOpusDownlinkSampleRateHz = 16000
	defaultOpusDownlinkBitrateBps   = 24000
)

// ErrOpusTranscodeUnavailable は Opus 変換実行環境が利用できない場合に返されます。
var ErrOpusTranscodeUnavailable = errors.New("opus transcoder is unavailable")

// OpusDownlinkOptions は WAV -> Opus 変換条件です。
type OpusDownlinkOptions struct {
	SampleRateHz    int
	BitrateBps      int
	FrameDurationMs int
	Timeout         time.Duration
}

// EncodeWAVToOpusPackets は WAV バイト列を Opus パケット列へ変換します。
// 戻り値の int はデコード時の想定サンプルレートです。
func EncodeWAVToOpusPackets(ctx context.Context, wavBytes []byte, opt OpusDownlinkOptions) ([][]byte, int, error) {
	if len(wavBytes) == 0 {
		return nil, 0, fmt.Errorf("wav bytes must not be empty")
	}

	sampleRateHz := opt.SampleRateHz
	if sampleRateHz <= 0 {
		sampleRateHz = defaultOpusDownlinkSampleRateHz
	}
	bitrateBps := opt.BitrateBps
	if bitrateBps <= 0 {
		bitrateBps = defaultOpusDownlinkBitrateBps
	}
	frameDurationMs := opt.FrameDurationMs
	if frameDurationMs <= 0 {
		frameDurationMs = 20
	}
	timeout := opt.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	txCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-vn",
		"-ac", "1",
		"-ar", strconv.Itoa(sampleRateHz),
		"-c:a", "libopus",
		"-application", "voip",
		"-frame_duration", strconv.Itoa(frameDurationMs),
		"-b:a", fmt.Sprintf("%dk", bitrateBps/1000),
		"-f", "ogg",
		"pipe:1",
	}

	cmd := exec.CommandContext(txCtx, "ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(wavBytes)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, 0, fmt.Errorf("%w: ffmpeg command is not found", ErrOpusTranscodeUnavailable)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, 0, fmt.Errorf("failed to transcode wav to opus: %s", msg)
	}

	packets, err := ExtractOpusPacketsFromOGG(out.Bytes())
	if err != nil {
		return nil, 0, err
	}
	if len(packets) == 0 {
		return nil, 0, fmt.Errorf("transcoded ogg does not contain opus packets")
	}

	return packets, sampleRateHz, nil
}

// ExtractOpusPacketsFromOGG は OGG/Opus バイト列から Opus 音声パケット列を抽出します。
// OpusHead / OpusTags は除外されます。
func ExtractOpusPacketsFromOGG(oggBytes []byte) ([][]byte, error) {
	if len(oggBytes) < 27 {
		return nil, fmt.Errorf("invalid ogg data: too short")
	}

	var packets [][]byte
	packetBuf := make([]byte, 0, 1024)
	offset := 0

	for {
		if offset >= len(oggBytes) {
			break
		}
		if offset+27 > len(oggBytes) {
			return nil, fmt.Errorf("invalid ogg page: truncated header")
		}
		if string(oggBytes[offset:offset+4]) != "OggS" {
			return nil, fmt.Errorf("invalid ogg page: missing capture pattern at offset %d", offset)
		}

		segmentCount := int(oggBytes[offset+26])
		headerLen := 27 + segmentCount
		if offset+headerLen > len(oggBytes) {
			return nil, fmt.Errorf("invalid ogg page: truncated lacing table")
		}

		lacing := oggBytes[offset+27 : offset+headerLen]
		dataLen := 0
		for _, seg := range lacing {
			dataLen += int(seg)
		}

		dataStart := offset + headerLen
		dataEnd := dataStart + dataLen
		if dataEnd > len(oggBytes) {
			return nil, fmt.Errorf("invalid ogg page: truncated page payload")
		}

		cursor := dataStart
		for _, seg := range lacing {
			segLen := int(seg)
			if segLen > 0 {
				packetBuf = append(packetBuf, oggBytes[cursor:cursor+segLen]...)
			}
			cursor += segLen
			if segLen < 255 {
				if len(packetBuf) > 0 {
					packet := append([]byte(nil), packetBuf...)
					if !bytes.HasPrefix(packet, []byte("OpusHead")) && !bytes.HasPrefix(packet, []byte("OpusTags")) {
						packets = append(packets, packet)
					}
				}
				packetBuf = packetBuf[:0]
			}
		}

		offset = dataEnd
	}

	return packets, nil
}
