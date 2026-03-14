// Package logging は構造化ログの初期化とコンテキスト連携を提供します。
// session_id などの相関フィールドを context 経由で全レイヤーに引き回します。
package logging

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Logger はアプリケーション全体で使用するグローバルロガーです。
var Logger zerolog.Logger

func init() {
	// デフォルトは info レベルで初期化します。
	Init("info")
}

// Init はログレベルと出力先を設定してロガーを初期化します。
// level には "debug", "info", "warn", "error" を指定します。
func Init(level string) {
	zerolog.TimeFieldFormat = time.RFC3339

	Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// WithSessionID は session_id を付与したロガーをコンテキストに格納して返します。
// 以降の FromContext 呼び出しで session_id 付きのロガーが取得できます。
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	log := Logger.With().Str("session_id", sessionID).Logger()
	return log.WithContext(ctx)
}

// FromContext はコンテキストに格納されたロガーを返します。
// ロガーが格納されていない場合はグローバルロガーへのポインタを返します。
func FromContext(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
