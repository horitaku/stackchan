package providers_test

import (
	"errors"
	"testing"

	"github.com/horitaku/stackchan/server/internal/protocol"
	"github.com/horitaku/stackchan/server/internal/providers"
)

func TestToProtocolError_Mapping(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCode     string
		wantRetry    bool
		wantContains string
	}{
		{
			name:         "invalid input maps to invalid_payload",
			err:          providers.NewError("mock", providers.CodeInvalidInput, "bad request", false, nil),
			wantCode:     protocol.ErrCodeInvalidPayload,
			wantRetry:    false,
			wantContains: "bad request",
		},
		{
			name:         "timeout maps to provider_timeout",
			err:          providers.NewError("mock", providers.CodeTimeout, "timeout", true, nil),
			wantCode:     protocol.ErrCodeProviderTimeout,
			wantRetry:    true,
			wantContains: "timeout",
		},
		{
			name:         "temporary maps to provider_unavailable",
			err:          providers.NewError("mock", providers.CodeTemporary, "temporary", true, nil),
			wantCode:     protocol.ErrCodeProviderUnavailable,
			wantRetry:    true,
			wantContains: "temporary",
		},
		{
			name:         "unknown provider code maps to provider_failed",
			err:          providers.NewError("mock", "unknown_code", "unknown", true, nil),
			wantCode:     protocol.ErrCodeProviderFailed,
			wantRetry:    true,
			wantContains: "unknown",
		},
		{
			name:         "non provider error maps to provider_failed",
			err:          errors.New("plain error"),
			wantCode:     protocol.ErrCodeProviderFailed,
			wantRetry:    false,
			wantContains: "plain error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg, retryable := providers.ToProtocolError(tt.err)
			if code != tt.wantCode {
				t.Fatalf("unexpected code: got=%s want=%s", code, tt.wantCode)
			}
			if retryable != tt.wantRetry {
				t.Fatalf("unexpected retryable: got=%v want=%v", retryable, tt.wantRetry)
			}
			if msg == "" {
				t.Fatal("message must not be empty")
			}
		})
	}
}

func TestAsProviderError(t *testing.T) {
	err := providers.NewError("mock", providers.CodeInternal, "boom", false, errors.New("root"))
	pe, ok := providers.AsProviderError(err)
	if !ok {
		t.Fatal("expected provider error")
	}
	if pe.Provider != "mock" {
		t.Fatalf("unexpected provider: %s", pe.Provider)
	}
}
