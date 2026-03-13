---
name: runtime-observability
description: Define and validate Stackchan runtime observability with structured logs, session_id and request_id correlation, latency and queue metrics, and health check coverage. Use when asked to design or review monitoring and diagnostics.
---

# Runtime Observability

Use this skill when the request is about logging, metrics, health checks, traceability, and operational diagnostics.

## Scope

In scope:
- Define structured log fields and correlation rules.
- Define runtime metrics and alert-friendly dimensions.
- Define health and readiness checks.
- Review observability gaps and propose minimal fixes.
- Standardize incident-friendly output formats.

Out of scope:
- Full dashboard implementation.
- Deep vendor-specific APM setup.
- Feature logic unrelated to runtime diagnostics.

## Inputs To Collect

Collect these before design or review:
- Runtime target: firmware, server, or both.
- Current log format and logger stack.
- Existing metrics backend and scrape model.
- SLO or latency objectives.
- Incident examples and pain points.
- Current health endpoint behavior.

If information is missing, continue with explicit assumptions.

## Target Paths

Prioritize these locations when present:
- server/internal/session/
- server/internal/conversation/
- server/internal/audio/
- server/internal/protocol/
- server/internal/web/
- firmware/runtime/network/
- firmware/runtime/audio/

## Core Observability Contract

### Correlation Fields

Every runtime log event should carry:
- timestamp
- level
- component
- message
- session_id
- request_id

Recommended additional fields:
- trace_id
- device_id
- sequence
- event_type
- retry_count

### Log Format Rules

- Use structured JSON logs in production paths.
- Keep message concise and machine searchable.
- Avoid secrets and personally sensitive values.
- Keep field names stable and snake_case.

### Metric Families

Minimum metric groups:
- latency histograms
- queue depth gauges
- request/stream counters
- error counters by code and retryable
- websocket disconnect and reconnect counters

Recommended labels:
- component
- provider
- direction
- outcome

### Health Model

- liveness: process loop is alive.
- readiness: dependencies are usable now.
- deep health: optional detailed dependency checks.

Health checks should include:
- websocket loop state
- provider reachability snapshot
- queue saturation threshold state
- degraded mode indicator

## Review Workflow

1) Inventory existing telemetry.
- List emitted logs, metrics, and health endpoints.
- Map each to runtime stage.

2) Correlation validation.
- Verify session_id propagation across boundaries.
- Verify request_id continuity from input to output.
- Flag places where correlation context is dropped.

3) Latency and queue validation.
- Verify stage metrics exist:
  - stt_latency_ms
  - llm_latency_ms
  - tts_latency_ms
  - playback_queue_depth
- Verify end-to-end timing metric availability.

4) Error and retry observability.
- Confirm error code and retryable tagging.
- Confirm retry attempts are measurable.
- Confirm timeout and cancel reasons are visible.

5) Health endpoint validation.
- Confirm liveness and readiness separation.
- Confirm degraded and hard-fail states are distinguishable.

6) Actionability review.
- Ensure each alert condition links to a troubleshooting signal.
- Ensure logs and metrics can answer who, where, and why.

## Common Gaps

- Missing request_id on asynchronous callbacks.
- Queue metrics without labels, making triage hard.
- Health endpoint always green despite degraded dependencies.
- Error logs without retryability context.
- Metrics available but no threshold guidance.

## Fix Recommendation Rules

When proposing fixes:
- Start with highest operational impact and lowest code risk.
- Prefer additive telemetry before behavior changes.
- Add one metric or field per patch when feasible.
- Include validation steps and expected signal changes.

## Deliverable Format

Return outputs in this order:

1) Findings by severity
2) Missing correlation points
3) Metric coverage table
4) Health check assessment
5) Prioritized fix plan
6) Residual risks

## References

- references/best_practices.md
- references/docs_links.md
- scripts/basic_example.py
