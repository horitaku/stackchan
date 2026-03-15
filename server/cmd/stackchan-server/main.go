// Package main は stackchan サーバーのエントリーポイントです。
// 環境変数の読み込み、Gin サーバーの起動、WebSocket エンドポイントの登録を行います。
package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/stackchan/server/internal/conversation"
	"github.com/stackchan/server/internal/logging"
	"github.com/stackchan/server/internal/providers"
	"github.com/stackchan/server/internal/providers/mock"
	"github.com/stackchan/server/internal/session"
	"github.com/stackchan/server/internal/web"
)

func main() {
	// .env ファイルを読み込みます（存在しない場合は無視します）
	_ = godotenv.Load()

	logLevel := getEnv("LOG_LEVEL", "info")
	logging.Init(logLevel)
	log := logging.Logger

	serverAddr := getEnv("SERVER_ADDR", ":8080")
	// WS_READ_TIMEOUT は heartbeat 間隔（15s）の 3 倍（45s）をデフォルトとします
	readTimeout := getEnvInt("WS_READ_TIMEOUT", 45)
	writeTimeout := getEnvInt("WS_WRITE_TIMEOUT", 30)

	// 本番環境では Gin をリリースモードで動かします
	if getEnv("APP_ENV", "local") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// セッションマネージャーと WebSocket ハンドラを初期化します
	manager := session.NewManager()
	policy := providers.CallPolicy{
		Timeout:     time.Duration(getEnvInt("PROVIDER_TIMEOUT_MS", 3000)) * time.Millisecond,
		MaxAttempts: getEnvInt("PROVIDER_MAX_ATTEMPTS", 2),
		BaseDelay:   time.Duration(getEnvInt("PROVIDER_RETRY_BASE_DELAY_MS", 100)) * time.Millisecond,
	}
	orchestrator := conversation.NewOrchestrator(&mock.STT{}, &mock.LLM{}, &mock.TTS{}, policy)
	wsHandler := web.NewWSHandler(manager, readTimeout, writeTimeout, orchestrator)
	apiHandler := web.NewAPIHandler(wsHandler.RuntimeState(), web.NewSettingsStore(), orchestrator)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"})))

	// ヘルスチェックエンドポイント
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// WebSocket エンドポイント
	r.GET("/ws", wsHandler.Handle)

	api := r.Group("/api")
	apiHandler.RegisterRoutes(api)

	// フェーズ 7: Svelte ビルド成果物を静的配信します
	r.Static("/ui", "./webui/dist")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})

	log.Info().Str("addr", serverAddr).Msg("starting stackchan server")
	if err := r.Run(serverAddr); err != nil {
		log.Fatal().Err(err).Msg("server failed to start")
	}
}

// corsMiddleware は CORS_ALLOWED_ORIGINS に基づいた簡易 CORS ミドルウェアです。
// 本番運用前にオリジンの厳密な検証を実装してください。
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAll || containsString(allowedOrigins, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	s, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvSlice(key string, fallback []string) []string {
	s, ok := os.LookupEnv(key)
	if !ok || s == "" {
		return fallback
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
