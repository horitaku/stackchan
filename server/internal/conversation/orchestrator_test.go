package conversation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stackchan/server/internal/conversation"
	"github.com/stackchan/server/internal/providers"
	"github.com/stackchan/server/internal/providers/mock"
)

func TestOrchestrator_ProcessAudioStream_Success(t *testing.T) {
	o := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 2 * time.Second, MaxAttempts: 2, BaseDelay: 10 * time.Millisecond},
	)

	res, err := o.ProcessAudioStream(context.Background(), "sess-001", "stream-001", []providers.AudioChunk{{
		StreamID:   "stream-001",
		ChunkIndex: 0,
		Codec:      "opus",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Transcript == "" {
		t.Fatal("transcript must not be empty")
	}
	if res.ReplyText == "" {
		t.Fatal("reply must not be empty")
	}
	if res.TTSDuration <= 0 {
		t.Fatal("tts duration must be > 0")
	}
}

func TestOrchestrator_ProcessAudioStream_FailureMappedAsProviderError(t *testing.T) {
	o := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 2 * time.Second, MaxAttempts: 1, BaseDelay: 10 * time.Millisecond},
	)

	// stream_id 空で STT 側 invalid_input を発生させます。
	_, err := o.ProcessAudioStream(context.Background(), "sess-001", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	pe, ok := providers.AsProviderError(err)
	if !ok {
		t.Fatalf("expected provider error, got %T", err)
	}
	if pe.Code != providers.CodeInvalidInput {
		t.Fatalf("expected invalid_input, got %s", pe.Code)
	}
}
