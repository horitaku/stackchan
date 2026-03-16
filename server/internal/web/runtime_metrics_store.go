package web

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// RuntimeMetricWrite は runtime_metrics へ保存する 1 レコード分の入力です。
type RuntimeMetricWrite struct {
	SessionID   string
	RequestID   string
	MetricName  string
	MetricValue float64
	MetricUnit  string
	ObservedAt  time.Time
}

// RuntimeMetricRow は runtime_metrics 取得 API 向けの返却モデルです。
type RuntimeMetricRow struct {
	ID          int64   `json:"id"`
	SessionID   string  `json:"session_id,omitempty"`
	RequestID   string  `json:"request_id,omitempty"`
	MetricName  string  `json:"metric_name"`
	MetricValue float64 `json:"metric_value"`
	MetricUnit  string  `json:"metric_unit,omitempty"`
	ObservedAt  string  `json:"observed_at"`
	CreatedAt   string  `json:"created_at"`
}

// RuntimeMetricsQuery は runtime_metrics 一覧取得時のフィルターです。
type RuntimeMetricsQuery struct {
	SessionID  string
	RequestID  string
	MetricName string
	From       *time.Time
	To         *time.Time
	Limit      int
}

// RuntimeMetricsStore は runtime_metrics の永続化と取得を担当します。
type RuntimeMetricsStore struct {
	db *sql.DB
}

// NewRuntimeMetricsStore は DATABASE_URL から Postgres 接続を初期化します。
func NewRuntimeMetricsStore(databaseURL string) (*RuntimeMetricsStore, error) {
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

	return &RuntimeMetricsStore{db: db}, nil
}

// Close は DB 接続をクローズします。
func (s *RuntimeMetricsStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// InsertMetrics は複数メトリクスをまとめて runtime_metrics へ保存します。
func (s *RuntimeMetricsStore) InsertMetrics(metrics []RuntimeMetricWrite) error {
	if s == nil || s.db == nil || len(metrics) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin metrics transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO runtime_metrics (session_id, request_id, metric_name, metric_value, metric_unit, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare metrics insert: %w", err)
	}
	defer stmt.Close()

	for _, m := range metrics {
		name := strings.TrimSpace(m.MetricName)
		if name == "" {
			continue
		}
		observedAt := m.ObservedAt.UTC()
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx,
			nullableString(m.SessionID),
			nullableString(m.RequestID),
			name,
			m.MetricValue,
			nullableString(m.MetricUnit),
			observedAt,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert runtime metric: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit metrics transaction: %w", err)
	}
	return nil
}

// ListMetrics は runtime_metrics を observed_at 降順で返します。
func (s *RuntimeMetricsStore) ListMetrics(q RuntimeMetricsQuery) ([]RuntimeMetricRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("metrics store is not configured")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	base := `
		SELECT id, COALESCE(session_id, ''), COALESCE(request_id, ''), metric_name, metric_value,
		       COALESCE(metric_unit, ''), observed_at, created_at
		FROM runtime_metrics
		WHERE deleted_at IS NULL
	`

	conditions := make([]string, 0, 5)
	args := make([]any, 0, 6)
	argIndex := 1

	if sessionID := strings.TrimSpace(q.SessionID); sessionID != "" {
		conditions = append(conditions, fmt.Sprintf("session_id = $%d", argIndex))
		args = append(args, sessionID)
		argIndex++
	}
	if requestID := strings.TrimSpace(q.RequestID); requestID != "" {
		conditions = append(conditions, fmt.Sprintf("request_id = $%d", argIndex))
		args = append(args, requestID)
		argIndex++
	}
	if metricName := strings.TrimSpace(q.MetricName); metricName != "" {
		conditions = append(conditions, fmt.Sprintf("metric_name = $%d", argIndex))
		args = append(args, metricName)
		argIndex++
	}
	if q.From != nil {
		conditions = append(conditions, fmt.Sprintf("observed_at >= $%d", argIndex))
		args = append(args, q.From.UTC())
		argIndex++
	}
	if q.To != nil {
		conditions = append(conditions, fmt.Sprintf("observed_at <= $%d", argIndex))
		args = append(args, q.To.UTC())
		argIndex++
	}

	query := base
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY observed_at DESC LIMIT $%d", argIndex)
	args = append(args, limit)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query runtime metrics: %w", err)
	}
	defer rows.Close()

	result := make([]RuntimeMetricRow, 0, limit)
	for rows.Next() {
		var row RuntimeMetricRow
		var observedAt time.Time
		var createdAt time.Time
		if err := rows.Scan(
			&row.ID,
			&row.SessionID,
			&row.RequestID,
			&row.MetricName,
			&row.MetricValue,
			&row.MetricUnit,
			&observedAt,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan runtime metric row: %w", err)
		}
		row.ObservedAt = observedAt.UTC().Format(time.RFC3339)
		row.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("runtime metrics rows iteration failed: %w", err)
	}

	return result, nil
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
