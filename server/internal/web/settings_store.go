package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const defaultSystemPrompt = "Stack-chan です。話しかけてくれてありがとう。"

// RuntimeSettings は WebUI から更新する設定値です。
type RuntimeSettings struct {
	PlaybackVolume     int     `json:"playback_volume"`
	ExpressionPreset   string  `json:"expression_preset"`
	LipSyncSensitivity float64 `json:"lip_sync_sensitivity"`
	LipSyncDamping     float64 `json:"lip_sync_damping"`
	MotionEnabled      bool    `json:"motion_enabled"`
	UpdatedAt          string  `json:"updated_at"`
}

// LLMSettings は WebUI から更新する LLM 設定値です。
type LLMSettings struct {
	SystemPrompt string `json:"system_prompt"`
	UpdatedAt    string `json:"updated_at"`
}

// SettingsStore は設定値をスレッドセーフに保持します。
type SettingsStore struct {
	mu                sync.RWMutex
	settings          RuntimeSettings
	llmSettings       LLMSettings
	llmCacheRefreshed time.Time
	llmCacheTTL       time.Duration
	db                *sql.DB
}

// NewSettingsStore はデフォルト設定入りのストアを返します。
func NewSettingsStore() *SettingsStore {
	return NewSettingsStoreWithDatabaseURL(strings.TrimSpace(os.Getenv("DATABASE_URL")))
}

// NewSettingsStoreWithDatabaseURL は DB 接続付きの設定ストアを返します。
func NewSettingsStoreWithDatabaseURL(databaseURL string) *SettingsStore {
	now := time.Now().UTC().Format(time.RFC3339)
	store := &SettingsStore{
		settings: RuntimeSettings{
			PlaybackVolume:     70,
			ExpressionPreset:   "neutral",
			LipSyncSensitivity: 1.0,
			LipSyncDamping:     0.3,
			MotionEnabled:      true,
			UpdatedAt:          now,
		},
		llmSettings: LLMSettings{
			SystemPrompt: defaultSystemPrompt,
			UpdatedAt:    now,
		},
		llmCacheTTL: 5 * time.Minute,
	}

	dsn := strings.TrimSpace(databaseURL)
	if dsn == "" {
		return store
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return store
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return store
	}
	store.db = db
	return store
}

// Close は DB 接続をクローズします。
func (s *SettingsStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func validateSettings(v RuntimeSettings) error {
	if v.PlaybackVolume < 0 || v.PlaybackVolume > 100 {
		return fmt.Errorf("playback_volume must be between 0 and 100")
	}
	switch v.ExpressionPreset {
	case "neutral", "happy", "sad", "surprised":
	default:
		return fmt.Errorf("expression_preset must be one of neutral, happy, sad, surprised")
	}
	if v.LipSyncSensitivity < 0.0 || v.LipSyncSensitivity > 2.0 {
		return fmt.Errorf("lip_sync_sensitivity must be between 0.0 and 2.0")
	}
	if v.LipSyncDamping < 0.0 || v.LipSyncDamping > 1.0 {
		return fmt.Errorf("lip_sync_damping must be between 0.0 and 1.0")
	}
	return nil
}

// Get は現在設定を返します。
func (s *SettingsStore) Get() RuntimeSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// Update は設定値を検証して保存します。
func (s *SettingsStore) Update(v RuntimeSettings) (RuntimeSettings, error) {
	if err := validateSettings(v); err != nil {
		return RuntimeSettings{}, err
	}
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.settings = v
	s.mu.Unlock()
	return v, nil
}

// GetLLMSettings は LLM 設定を返します。
func (s *SettingsStore) GetLLMSettings() LLMSettings {
	s.mu.RLock()
	if s.db == nil {
		defer s.mu.RUnlock()
		return s.llmSettings
	}
	if time.Since(s.llmCacheRefreshed) <= s.llmCacheTTL {
		defer s.mu.RUnlock()
		return s.llmSettings
	}
	s.mu.RUnlock()

	v, err := s.loadLLMSettingsFromDB()
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.llmSettings
	}

	s.mu.Lock()
	s.llmSettings = v
	s.llmCacheRefreshed = time.Now().UTC()
	s.mu.Unlock()
	return v
}

// UpdateLLMSettings は LLM 設定を更新します。
func (s *SettingsStore) UpdateLLMSettings(v LLMSettings) (LLMSettings, error) {
	prompt := strings.TrimSpace(v.SystemPrompt)
	if prompt == "" {
		prompt = defaultSystemPrompt
	}
	v.SystemPrompt = prompt
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if s.db != nil {
		if err := s.upsertLLMSettingsToDB(v); err != nil {
			return LLMSettings{}, err
		}
	}

	s.mu.Lock()
	s.llmSettings = v
	s.llmCacheRefreshed = time.Now().UTC()
	s.mu.Unlock()
	return v, nil
}

func (s *SettingsStore) loadLLMSettingsFromDB() (LLMSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const query = `
		SELECT value, updated_at
		FROM system_settings
		WHERE category = 'llm' AND key = 'system_prompt' AND deleted_at IS NULL
		LIMIT 1
	`

	var raw []byte
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, query).Scan(&raw, &updatedAt)
	if err == sql.ErrNoRows {
		return LLMSettings{
			SystemPrompt: defaultSystemPrompt,
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		}, nil
	}
	if err != nil {
		return LLMSettings{}, fmt.Errorf("failed to load llm settings: %w", err)
	}

	var payload struct {
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return LLMSettings{}, fmt.Errorf("failed to decode llm settings payload: %w", err)
	}
	prompt := strings.TrimSpace(payload.SystemPrompt)
	if prompt == "" {
		prompt = defaultSystemPrompt
	}
	return LLMSettings{SystemPrompt: prompt, UpdatedAt: updatedAt.UTC().Format(time.RFC3339)}, nil
}

func (s *SettingsStore) upsertLLMSettingsToDB(v LLMSettings) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	payload, err := json.Marshal(map[string]string{"system_prompt": v.SystemPrompt})
	if err != nil {
		return fmt.Errorf("failed to encode llm settings payload: %w", err)
	}

	const query = `
		INSERT INTO system_settings (category, key, value, updated_by)
		VALUES ('llm', 'system_prompt', $1::jsonb, 'webui')
		ON CONFLICT (category, key)
		DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = NOW()
	`

	if _, err := s.db.ExecContext(ctx, query, string(payload)); err != nil {
		return fmt.Errorf("failed to upsert llm settings: %w", err)
	}
	return nil
}
