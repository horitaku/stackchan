package audio

import (
	"bytes"
	"testing"
)

// TestBuildOpusOGG は BuildOpusOGG が有効な OGG ページ構造を生成するかを検証します。
func TestBuildOpusOGG(t *testing.T) {
	// ダミーの Opus フレームを 3 個作成します（最小限の valid Opus-like バイト列）
	frames := [][]byte{
		{0x78, 0x01, 0x02, 0x03, 0x04}, // 5 bytes = ダミー Opus frame
		{0x78, 0x05, 0x06, 0x07, 0x08},
		{0x78, 0x09, 0x0A, 0x0B, 0x0C},
	}

	ogg := BuildOpusOGG(frames, 16000, 20, 1)

	if len(ogg) == 0 {
		t.Fatal("BuildOpusOGG returned empty output")
	}

	// ── ページ 1: BOS ページの検証 ─────────────────────────────────────
	t.Run("capture_pattern", func(t *testing.T) {
		if !bytes.HasPrefix(ogg, []byte("OggS")) {
			t.Error("OGG output does not start with capture_pattern 'OggS'")
		}
	})

	t.Run("BOS flag on first page", func(t *testing.T) {
		// ページの header_type_flag はオフセット 5 にあります
		if ogg[5] != 0x02 {
			t.Errorf("first page header_type_flag = 0x%02X, want 0x02 (BOS)", ogg[5])
		}
	})

	t.Run("page_sequence_number ascending", func(t *testing.T) {
		// オフセット 18 の 4 bytes が page_sequence_number（uint32 LE）
		// 1ページ目は 0 であることをチェックします
		seq0 := uint32(ogg[18]) | uint32(ogg[19])<<8 | uint32(ogg[20])<<16 | uint32(ogg[21])<<24
		if seq0 != 0 {
			t.Errorf("first page sequence_number = %d, want 0", seq0)
		}
	})

	// ── 最小ページ数の確認 ─────────────────────────────────────────────
	// ヘッダ 2 ページ + 音声フレーム 3 ページ = 合計 5 ページが期待されます
	t.Run("minimum page count", func(t *testing.T) {
		count := countOGGPages(ogg)
		want := 2 + len(frames) // 2 header + N audio
		if count != want {
			t.Errorf("OGG page count = %d, want %d", count, want)
		}
	})
}

// TestBuildOpusOGGEmpty は空のフレームリストで正常に動作するかを検証します。
func TestBuildOpusOGGEmpty(t *testing.T) {
	ogg := BuildOpusOGG(nil, 16000, 20, 1)
	// 空でも OpusHead + OpusTags の 2 ページは生成されます
	if count := countOGGPages(ogg); count != 2 {
		t.Errorf("empty frames: page count = %d, want 2", count)
	}
}

// TestLacingEncode はラッキングエンコードの正確性を検証します。
func TestLacingEncode(t *testing.T) {
	tests := []struct {
		name      string
		dataLen   int
		wantLaces []byte
	}{
		{
			name:      "0 bytes",
			dataLen:   0,
			wantLaces: []byte{0},
		},
		{
			name:      "100 bytes",
			dataLen:   100,
			wantLaces: []byte{100},
		},
		{
			name:      "255 bytes (exactly full segment + terminaton 0)",
			dataLen:   255,
			wantLaces: []byte{255, 0},
		},
		{
			name:      "256 bytes (255 + 1)",
			dataLen:   256,
			wantLaces: []byte{255, 1},
		},
		{
			name:      "510 bytes (255*2 + 0)",
			dataLen:   510,
			wantLaces: []byte{255, 255, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataLen)
			got := lacingEncode(data)
			if !bytes.Equal(got, tt.wantLaces) {
				t.Errorf("lacingEncode(%d bytes) = %v, want %v", tt.dataLen, got, tt.wantLaces)
			}
		})
	}
}

// countOGGPages は OGG バイト列に含まれる OggS キャプチャパターンの数をカウントします。
// 正確なパーサーではなく、ページ数のおおよそのカウントに使用します。
func countOGGPages(ogg []byte) int {
	count := 0
	pattern := []byte("OggS")
	for i := 0; i <= len(ogg)-4; i++ {
		if bytes.Equal(ogg[i:i+4], pattern) {
			count++
		}
	}
	return count
}
