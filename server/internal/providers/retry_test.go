package providers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/horitaku/stackchan/server/internal/providers"
)

func TestCallWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	attempts := 0
	res, err := providers.CallWithRetry(context.Background(), providers.CallPolicy{
		Timeout:     300 * time.Millisecond,
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
	}, func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", providers.NewError("mock", providers.CodeTemporary, "temporary failure", true, nil)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("unexpected result: %s", res)
	}
	if attempts != 2 {
		t.Fatalf("unexpected attempts: %d", attempts)
	}
}

func TestCallWithRetry_StopOnNonRetryable(t *testing.T) {
	attempts := 0
	_, err := providers.CallWithRetry(context.Background(), providers.CallPolicy{
		Timeout:     300 * time.Millisecond,
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
	}, func(context.Context) (string, error) {
		attempts++
		return "", providers.NewError("mock", providers.CodeInvalidInput, "invalid", false, nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected single attempt, got %d", attempts)
	}
}

func TestCallWithRetry_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0
	_, err := providers.CallWithRetry(ctx, providers.CallPolicy{
		Timeout:     300 * time.Millisecond,
		MaxAttempts: 5,
		BaseDelay:   200 * time.Millisecond,
	}, func(context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			cancel()
		}
		return "", providers.NewError("mock", providers.CodeTemporary, "temporary", true, nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := providers.AsProviderError(err)
	if !ok {
		t.Fatalf("expected provider error, got %T", err)
	}
	if pe.Code != providers.CodeTimeout {
		t.Fatalf("expected timeout code, got %s", pe.Code)
	}
}

func TestCallWithRetry_PropagatesLastError(t *testing.T) {
	finalErr := providers.NewError("mock", providers.CodeInternal, "boom", false, errors.New("root"))
	_, err := providers.CallWithRetry(context.Background(), providers.CallPolicy{
		Timeout:     300 * time.Millisecond,
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
	}, func(context.Context) (string, error) {
		return "", finalErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, finalErr) {
		t.Fatalf("expected wrapped final error")
	}
}
