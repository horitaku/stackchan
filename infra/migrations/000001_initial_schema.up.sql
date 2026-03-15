CREATE TABLE sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_started_at ON sessions (started_at DESC);

CREATE TABLE utterances (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    stt_latency_ms INTEGER,
    llm_latency_ms INTEGER,
    tts_latency_ms INTEGER,
    total_latency_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_utterances_session FOREIGN KEY (session_id) REFERENCES sessions (session_id)
);

CREATE INDEX idx_utterances_session_request ON utterances (session_id, request_id);
CREATE INDEX idx_utterances_created_at ON utterances (created_at DESC);

CREATE TABLE conversation_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    request_id TEXT,
    sequence BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_events_session FOREIGN KEY (session_id) REFERENCES sessions (session_id),
    CONSTRAINT uq_events_sequence UNIQUE (session_id, sequence)
);

CREATE INDEX idx_events_session_created ON conversation_events (session_id, created_at DESC);
CREATE INDEX idx_events_request_id ON conversation_events (request_id);

CREATE TABLE runtime_metrics (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT,
    request_id TEXT,
    metric_name TEXT NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    metric_unit TEXT,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_metrics_session FOREIGN KEY (session_id) REFERENCES sessions (session_id)
);

CREATE INDEX idx_metrics_observed_at ON runtime_metrics (observed_at DESC);
CREATE INDEX idx_metrics_name_observed ON runtime_metrics (metric_name, observed_at DESC);

CREATE TABLE system_settings (
    id BIGSERIAL PRIMARY KEY,
    category TEXT NOT NULL,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_system_settings_key UNIQUE (category, key)
);

CREATE INDEX idx_system_settings_category ON system_settings (category);
