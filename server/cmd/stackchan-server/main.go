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
	"github.com/horitaku/stackchan/server/internal/conversation"
	"github.com/horitaku/stackchan/server/internal/logging"
	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/providers/mock"
	"github.com/horitaku/stackchan/server/internal/providers/openai"
	"github.com/horitaku/stackchan/server/internal/session"
	"github.com/horitaku/stackchan/server/internal/web"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
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
	settingsStore := web.NewSettingsStore()
	defer settingsStore.Close()
	policy := providers.CallPolicy{
		Timeout:     time.Duration(getEnvInt("PROVIDER_TIMEOUT_MS", 3000)) * time.Millisecond,
		MaxAttempts: getEnvInt("PROVIDER_MAX_ATTEMPTS", 2),
		BaseDelay:   time.Duration(getEnvInt("PROVIDER_RETRY_BASE_DELAY_MS", 100)) * time.Millisecond,
	}

	llmProvider := selectLLMProvider(log)
	orchestrator := conversation.NewOrchestrator(&mock.STT{}, llmProvider, &mock.TTS{}, policy)
	// LLM は STT/TTS よりも応答が遅いため、専用タイムアウトを設定します
	llmTimeoutMs := getEnvInt("LLM_PROVIDER_TIMEOUT_MS", 30000)
	orchestrator.SetLLMPolicy(providers.CallPolicy{
		Timeout:     time.Duration(llmTimeoutMs) * time.Millisecond,
		MaxAttempts: 1, // LLM はリトライが高コストのため 1 回のみ
		BaseDelay:   policy.BaseDelay,
	})
	orchestrator.SetSystemPromptLoader(func(_ string) string {
		return settingsStore.GetLLMSettings().SystemPrompt
	})

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL != "" {
		contextStore, err := session.NewUtteranceStore(databaseURL)
		if err != nil {
			log.Warn().Err(err).Msg("conversation context store is disabled")
		} else {
			defer contextStore.Close()
			maxTurns := getEnvInt("LLM_CONTEXT_MAX_TURNS", 5)
			maxTokens := getEnvInt("LLM_CONTEXT_MAX_TOKENS", 2000)
			orchestrator.SetConversationContext(session.NewConversationContextManager(contextStore, maxTurns, maxTokens))
			log.Info().Int("max_turns", maxTurns).Int("max_tokens", maxTokens).Msg("conversation context manager enabled")
		}
	}

	wsHandler := web.NewWSHandler(manager, readTimeout, writeTimeout, orchestrator)
	apiHandler := web.NewAPIHandler(wsHandler.RuntimeState(), settingsStore, orchestrator)
	apiHandler.AttachWSHandler(wsHandler)

	if databaseURL != "" {
		metricsStore, err := web.NewRuntimeMetricsStore(databaseURL)
		if err != nil {
			log.Warn().Err(err).Msg("runtime metrics store is disabled")
		} else {
			defer metricsStore.Close()
			wsHandler.RuntimeState().SetMetricsStore(metricsStore)
			apiHandler.AttachRuntimeMetricsStore(metricsStore)
			log.Info().Msg("runtime metrics store enabled")
		}
	} else {
		log.Warn().Msg("DATABASE_URL is empty; runtime metrics persistence is disabled")
	}

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

func selectLLMProvider(log zerolog.Logger) providers.LLMProvider {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		log.Warn().Msg("OPENAI_API_KEY is empty; fallback to mock LLM provider")
		return &mock.LLM{}
	}

	provider, err := openai.NewLLM(openai.LLMConfig{
		APIKey:      apiKey,
		BaseURL:     strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")),
		Model:       getEnv("OPENAI_MODEL_CHAT", "gpt-4o-mini"),
		Temperature: getEnvFloat("OPENAI_TEMPERATURE", 0.7),
	}, &http.Client{Timeout: resolveOpenAIHTTPTimeout()})
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize OpenAI LLM provider; fallback to mock")
		return &mock.LLM{}
	}
	log.Info().Str("provider", provider.Name()).Msg("OpenAI LLM provider enabled")
	return provider
}

func resolveOpenAIHTTPTimeout() time.Duration {
	sec := getEnvInt("OPENAI_HTTP_TIMEOUT_SEC", 45)
	if sec <= 0 {
		sec = 45
	}
	return time.Duration(sec) * time.Second
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

func getEnvFloat(key string, fallback float64) float64 {
	s, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}
