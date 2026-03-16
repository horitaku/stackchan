// ogg.go は raw Opus フレーム列から OGG/Opus コンテナファイルを構築します。
//
// 用途: OpenAI Whisper STT API など OGG/Opus ファイルを受け取るサービスへ
// バイナリストリームから受信した raw Opus フレームを渡す際に使用します。
//
// 準拠仕様:
//   - RFC 3533: OGG ページ構造
//   - RFC 7845: OGG/Opus カプセル化
//
// 注意: 本実装は CGO 不要の pure-Go 実装です。
// Opus エンコード/デコード自体は行いません（フレームラッピングのみ）。
package audio

import (
	"bytes"
	"encoding/binary"
)

// BuildOpusOGG は raw Opus フレーム列（バイナリストリームから受信したもの）を
// OGG/Opus コンテナ形式にラップして返します。
//
// 引数:
//   - frames: raw Opus フレームのスライス（各要素が 1 フレーム）
//   - sampleRateHz: 入力サンプルレート（OpusHead の input_sample_rate に記録）
//   - frameDurationMs: 1 フレームの時間（ms）。OGG granule_position の計算に使用
//   - channelCount: 1=モノ, 2=ステレオ
//
// 戻り値: OGG/Opus 形式のバイト列（.ogg ファイルとして扱えます）
func BuildOpusOGG(frames [][]byte, sampleRateHz, frameDurationMs, channelCount int) []byte {
	var buf bytes.Buffer

	// ストリームシリアル番号（任意の固定値）
	// 複数ストリームのシリアル番号が衝突しないよう固定値を使用します
	const serial = uint32(0x5CA7)

	// ── ページ 1: BOS（Begin Of Stream）/ OpusHead ────────────────────────
	// BOS ページは OGG ストリームの最初のページであることを示します。
	// (header_type_flag = 0x02)
	opusHead := buildOpusHead(channelCount, sampleRateHz)
	writePage(&buf, serial, 0, 0, 0x02, opusHead)

	// ── ページ 2: OpusTags（メタデータ）─────────────────────────────────
	// ベンダー文字列と空のコメントリストを含みます。
	opusTags := buildOpusTags()
	writePage(&buf, serial, 1, 0, 0x00, opusTags)

	// ── 音声ページ群 ──────────────────────────────────────────────────────
	// 各 Opus フレームを 1 つの OGG ページに格納します。
	// granule_position は 48kHz 単位で累積されます（RFC 7845 §5.1）。
	samplesPerFrame := Opus48HzSamplesPerFrame(frameDurationMs)
	var granule int64
	for i, frame := range frames {
		granule += samplesPerFrame
		headerType := uint8(0x00)
		// 最後のページは EOS（End Of Stream）フラグをセットします
		if i == len(frames)-1 {
			headerType = 0x04
		}
		// ページシーケンス番号 = 0,1 がヘッダページなので 2 から開始
		writePage(&buf, serial, uint32(i+2), granule, headerType, frame)
	}

	return buf.Bytes()
}

// buildOpusHead は OpusHead 識別パケットを構築します（RFC 7845 §5.1）。
//
// OpusHead フォーマット:
//
//	Bytes 0-7:   "OpusHead" (magic signature)
//	Byte  8:     version (= 1)
//	Byte  9:     output_channel_count
//	Bytes 10-11: pre_skip (uint16 LE, 48kHz 単位)
//	Bytes 12-15: input_sample_rate (uint32 LE)
//	Bytes 16-17: output_gain (int16 LE, = 0 で変化なし)
//	Byte  18:    channel_mapping_family (0 = RTP マッピング: 1ch or 2ch)
func buildOpusHead(channelCount, sampleRateHz int) []byte {
	buf := make([]byte, 19)
	copy(buf[0:8], "OpusHead")
	buf[8] = 1                                                      // version
	buf[9] = byte(channelCount)                                     // output channel count
	binary.LittleEndian.PutUint16(buf[10:12], 312)                  // pre-skip: 312 samples ≈ 6.5ms at 48kHz（推奨デフォルト）
	binary.LittleEndian.PutUint32(buf[12:16], uint32(sampleRateHz)) // input sample rate
	binary.LittleEndian.PutUint16(buf[16:18], 0)                    // output gain = 0
	buf[18] = 0                                                     // channel mapping family = 0 (RTP)
	return buf
}

// buildOpusTags は OpusTags 識別パケットを構築します（RFC 7845 §5.2）。
//
// OpusTags フォーマット:
//
//	Bytes 0-7:              "OpusTags" (magic signature)
//	Bytes 8-(8+L-1):        vendor_string_length (uint32 LE) + vendor_string
//	Bytes (8+L)-(8+L+3):    user_comment_list_length (uint32 LE, = 0)
func buildOpusTags() []byte {
	const vendor = "Stackchan/P8-05"
	buf := make([]byte, 8+4+len(vendor)+4)
	copy(buf[0:8], "OpusTags")
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(vendor)))
	copy(buf[12:12+len(vendor)], vendor)
	binary.LittleEndian.PutUint32(buf[12+len(vendor):], 0) // user_comment_list_length = 0
	return buf
}

// writePage は 1 つの OGG ページを buf に書き込みます。
// data は 1 つの論理パケットです（255 バイトを超える場合はセグメントを自動分割します）。
//
// OGG ページ構造:
//
//	Bytes 0-3:   capture_pattern "OggS"
//	Byte  4:     stream_structure_version (= 0)
//	Byte  5:     header_type_flag
//	Bytes 6-13:  granule_position (int64 LE)
//	Bytes 14-17: bitstream_serial_number (uint32 LE)
//	Bytes 18-21: page_sequence_number (uint32 LE)
//	Bytes 22-25: CRC_checksum (uint32 LE, フィールドをゼロにして計算)
//	Byte  26:    number_page_segments
//	Bytes 27+:   segment_table (各セグメントのバイト数)
//	After:       ページデータ
func writePage(buf *bytes.Buffer, serial, pageSeq uint32, granule int64, headerType uint8, data []byte) {
	// セグメントラッキング（OGG のパケット境界エンコーディング）を計算します
	lacingValues := lacingEncode(data)

	// ── ページヘッダを構築します ──────────────────────────────────────────
	header := make([]byte, 27+len(lacingValues))
	copy(header[0:4], "OggS")                                    // capture_pattern
	header[4] = 0                                                // stream_structure_version
	header[5] = headerType                                       // header_type_flag
	binary.LittleEndian.PutUint64(header[6:14], uint64(granule)) // granule_position
	binary.LittleEndian.PutUint32(header[14:18], serial)         // bitstream_serial_number
	binary.LittleEndian.PutUint32(header[18:22], pageSeq)        // page_sequence_number
	// header[22:26]: CRC フィールドは後で書き込むためゼロのままにします
	header[26] = byte(len(lacingValues)) // number_page_segments
	copy(header[27:], lacingValues)      // segment_table

	// ── CRC を計算します ──────────────────────────────────────────────────
	// CRC はページ全体（header + data）に対して計算します。
	// その際 CRC フィールド（header[22:26]）はゼロとして計算します（RFC 3533 §6.3.1）。
	pageData := make([]byte, len(header)+len(data))
	copy(pageData, header)
	copy(pageData[len(header):], data)
	crc := oggCRC32(pageData)
	binary.LittleEndian.PutUint32(pageData[22:26], crc)

	buf.Write(pageData)
}

// lacingEncode は OGG のラッキングを計算します。
// パケットを 255 バイト単位のセグメントに分割し、最後のセグメント長で終端します。
// 最後のセグメントがちょうど 255 バイトの場合、パケット境界を示すため 0 バイトのセグメントを追加します。
func lacingEncode(data []byte) []byte {
	n := len(data)
	laces := make([]byte, 0, n/255+2)
	for n >= 255 {
		laces = append(laces, 255)
		n -= 255
	}
	// 最後のセグメント長（0〜254 バイト）。0 の場合はパケット境界の終端を示します。
	laces = append(laces, byte(n))
	return laces
}

// oggCRC32 は OGG ページ用の CRC32 を計算します。
//
// OGG は polynomial 0x04C11DB7 の非反射 (non-reflected) CRC32 を使用します。
// これは Go 標準ライブラリの hash/crc32（IEEE: reflected）とは異なります（RFC 3533 §6.3.1）。
func oggCRC32(data []byte) uint32 {
	crc := uint32(0)
	for _, b := range data {
		crc = (crc << 8) ^ oggCRC32Table[((crc>>24)^uint32(b))&0xFF]
	}
	return crc
}

// oggCRC32Table は OGG CRC32 テーブルです（polynomial 0x04C11DB7、非反射）。
var oggCRC32Table [256]uint32

func init() {
	for i := range oggCRC32Table {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
		oggCRC32Table[i] = crc
	}
}
