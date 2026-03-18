package session

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Utterance は会話履歴 1 件分の保存モデルです。
type Utterance struct {
	SessionID      string
	RequestID      string
	Role           string
	Content        string
	STTLatencyMs   int64
	LLMLatencyMs   int64
	TTSLatencyMs   int64
	TotalLatencyMs int64
}

// UtteranceStore は utterances テーブルの read/write を担当します。
type UtteranceStore struct {
	db *sql.DB
}

// NewUtteranceStore は DATABASE_URL から Postgres 接続を初期化します。
func NewUtteranceStore(databaseURL string) (*UtteranceStore, error) {
	dsn := strings.TrimSpace(databaseURL)
	if dsn == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &UtteranceStore{db: db}, nil
}

// Close は DB 接続を閉じます。
func (s *UtteranceStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// InsertUtterance は utterances へ 1 レコードを保存します。
func (s *UtteranceStore) InsertUtterance(v Utterance) error {
	if s == nil || s.db == nil {
		return nil
	}
	if strings.TrimSpace(v.SessionID) == "" || strings.TrimSpace(v.RequestID) == "" || strings.TrimSpace(v.Role) == "" {
		return fmt.Errorf("session_id, request_id, role are required")
	}
	if strings.TrimSpace(v.Content) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (session_id, started_at)
		VALUES ($1, NOW())
		ON CONFLICT (session_id) DO NOTHING
	`, v.SessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to ensure session row: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO utterances (
			session_id, request_id, role, content,
			stt_latency_ms, llm_latency_ms, tts_latency_ms, total_latency_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		v.SessionID,
		v.RequestID,
		v.Role,
		v.Content,
		nullableLatency(v.STTLatencyMs),
		nullableLatency(v.LLMLatencyMs),
		nullableLatency(v.TTSLatencyMs),
		nullableLatency(v.TotalLatencyMs),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to insert utterance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit utterance transaction: %w", err)
	}
	return nil
}

// ListRecentUtterances は直近 N 件の発話を古い順で返します。
func (s *UtteranceStore) ListRecentUtterances(sessionID string, limit int) ([]Utterance, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, request_id, role, content,
		       COALESCE(stt_latency_ms, 0), COALESCE(llm_latency_ms, 0),
		       COALESCE(tts_latency_ms, 0), COALESCE(total_latency_ms, 0)
		FROM (
			SELECT session_id, request_id, role, content,
			       stt_latency_ms, llm_latency_ms, tts_latency_ms, total_latency_ms,
			       created_at
			FROM utterances
			WHERE session_id = $1
			  AND deleted_at IS NULL
			  AND role IN ('user', 'assistant')
			ORDER BY created_at DESC
			LIMIT $2
		) recent
		ORDER BY created_at ASC
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query utterances: %w", err)
	}
	defer rows.Close()

	items := make([]Utterance, 0, limit)
	for rows.Next() {
		var v Utterance
		if err := rows.Scan(
			&v.SessionID,
			&v.RequestID,
			&v.Role,
			&v.Content,
			&v.STTLatencyMs,
			&v.LLMLatencyMs,
			&v.TTSLatencyMs,
			&v.TotalLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan utterance row: %w", err)
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate utterance rows: %w", err)
	}
	return items, nil
}

func nullableLatency(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
