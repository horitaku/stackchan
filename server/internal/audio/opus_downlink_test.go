package audio

import (
	"bytes"
	"testing"
)

func TestExtractOpusPacketsFromOGG(t *testing.T) {
	frames := [][]byte{
		{0x11, 0x22, 0x33, 0x44},
		bytes.Repeat([]byte{0x55}, 300),
		{0x66, 0x77, 0x88},
	}

	ogg := BuildOpusOGG(frames, 16000, 20, 1)
	got, err := ExtractOpusPacketsFromOGG(ogg)
	if err != nil {
		t.Fatalf("extract packets failed: %v", err)
	}
	if len(got) != len(frames) {
		t.Fatalf("packet count mismatch: got=%d want=%d", len(got), len(frames))
	}

	for i := range frames {
		if !bytes.Equal(got[i], frames[i]) {
			t.Fatalf("packet[%d] mismatch", i)
		}
	}
}

func TestExtractOpusPacketsFromOGG_Invalid(t *testing.T) {
	_, err := ExtractOpusPacketsFromOGG([]byte("not-ogg"))
	if err == nil {
		t.Fatal("expected error for invalid ogg")
	}
}
