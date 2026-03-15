package web

import (
	"fmt"
	"sync"
	"time"
)

// RuntimeSettings は WebUI から更新する設定値です。
type RuntimeSettings struct {
	PlaybackVolume     int     `json:"playback_volume"`
	ExpressionPreset   string  `json:"expression_preset"`
	LipSyncSensitivity float64 `json:"lip_sync_sensitivity"`
	LipSyncDamping     float64 `json:"lip_sync_damping"`
	MotionEnabled      bool    `json:"motion_enabled"`
	UpdatedAt          string  `json:"updated_at"`
}

// SettingsStore は設定値をスレッドセーフに保持します。
type SettingsStore struct {
	mu       sync.RWMutex
	settings RuntimeSettings
}

// NewSettingsStore はデフォルト設定入りのストアを返します。
func NewSettingsStore() *SettingsStore {
	return &SettingsStore{
		settings: RuntimeSettings{
			PlaybackVolume:     70,
			ExpressionPreset:   "neutral",
			LipSyncSensitivity: 1.0,
			LipSyncDamping:     0.3,
			MotionEnabled:      true,
			UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
		},
	}
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
