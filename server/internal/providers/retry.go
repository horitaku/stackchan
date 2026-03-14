// Package providers は provider 呼び出しのタイムアウトとリトライ制御を提供します。
package providers

import (
	"context"
	"time"
)

// CallPolicy は provider 呼び出しの共通ポリシーです。
type CallPolicy struct {
	Timeout     time.Duration
	MaxAttempts int
	BaseDelay   time.Duration
}

// Normalize は未設定項目に既定値を補います。
func (p CallPolicy) Normalize() CallPolicy {
	n := p
	if n.Timeout <= 0 {
		n.Timeout = 3 * time.Second
	}
	if n.MaxAttempts <= 0 {
		n.MaxAttempts = 2
	}
	if n.BaseDelay <= 0 {
		n.BaseDelay = 100 * time.Millisecond
	}
	return n
}

// CallWithRetry は provider 呼び出しをタイムアウト + 指数バックオフで実行します。
func CallWithRetry[T any](ctx context.Context, policy CallPolicy, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	p := policy.Normalize()

	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, p.Timeout)
		res, err := fn(callCtx)
		cancel()
		if err == nil {
			return res, nil
		}
		lastErr = err

		pe, ok := AsProviderError(err)
		if ok && !pe.Retryable {
			return zero, err
		}
		if attempt == p.MaxAttempts {
			break
		}

		delay := p.BaseDelay * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, NewError("retry-wrapper", CodeTimeout, "context cancelled during retry wait", false, ctx.Err())
		case <-timer.C:
		}
	}

	if lastErr == nil {
		return zero, NewError("retry-wrapper", CodeInternal, "provider call failed without detailed error", false, nil)
	}
	return zero, lastErr
}
