# Audio Pipeline Best Practices

## Keep Timing Contracts Explicit

- Always include sample_rate_hz and frame_duration_ms in metadata.
- Keep channel count explicit across encode and decode stages.

## Favor Stable Framing

- Keep frame duration fixed during one session.
- Use monotonic sequence to detect gaps and reorder issues.
- Define clear end and cancel events for stream lifecycle.

## Budget Latency By Stage

- Measure each stage independently.
- Track p50, p90, and p99 where possible.
- Optimize highest contributors first.

## Protect Playback Smoothness

- Keep playback queue depth bounded.
- Handle underflow and overflow explicitly.
- Apply backpressure when producer outruns consumer.

## Retry And Recovery

- Retry only transient network and upstream failures.
- Keep reconnect behavior deterministic.
- Preserve session context for short disconnect windows.

## Observability First

- Attach session_id and request_id in all related logs.
- Emit queue and latency metrics at stable cadence.
- Keep error payloads actionable and retry-aware.
