// Package providers は provider 境界のエラー定義と protocol 変換を提供します。
package providers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/stackchan/server/internal/protocol"
)

const (
	// CodeInvalidInput は provider 入力が不正な場合のコードです。
	CodeInvalidInput = "invalid_input"
	// CodeTimeout は provider 呼び出しがタイムアウトした場合のコードです。
	CodeTimeout = "timeout"
	// CodeTemporary は一時エラーを示すコードです。
	CodeTemporary = "temporary"
	// CodeInternal は provider 内部エラーを示すコードです。
	CodeInternal = "internal"
)

// Error は provider 境界の統一エラーです。
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("provider=%s code=%s message=%s", e.Provider, e.Code, e.Message)
	}
	return fmt.Sprintf("provider=%s code=%s message=%s cause=%v", e.Provider, e.Code, e.Message, e.Cause)
}

// Unwrap は errors.Is / errors.As 用のアンラップを提供します。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewError は provider 境界エラーを生成します。
func NewError(provider, code, message string, retryable bool, cause error) error {
	return &Error{
		Provider:  provider,
		Code:      code,
		Message:   message,
		Retryable: retryable,
		Cause:     cause,
	}
}

// AsProviderError は任意の error を provider.Error として取り出します。
func AsProviderError(err error) (*Error, bool) {
	var pe *Error
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

// ToProtocolError は provider エラーを protocol error コードへ変換します。
func ToProtocolError(err error) (code, message string, retryable bool) {
	if err == nil {
		return "", "", false
	}

	pe, ok := AsProviderError(err)
	if !ok {
		return protocol.ErrCodeProviderFailed, err.Error(), false
	}

	switch strings.ToLower(pe.Code) {
	case CodeInvalidInput:
		return protocol.ErrCodeInvalidPayload, pe.Message, false
	case CodeTimeout:
		return protocol.ErrCodeProviderTimeout, pe.Message, true
	case CodeTemporary:
		return protocol.ErrCodeProviderUnavailable, pe.Message, true
	default:
		return protocol.ErrCodeProviderFailed, pe.Message, pe.Retryable
	}
}
