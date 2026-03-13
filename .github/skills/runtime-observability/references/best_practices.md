# Runtime Observability Best Practices

## Preserve Correlation End To End

- Propagate session_id and request_id across every boundary.
- Include correlation fields on success and failure logs.
- Keep correlation values stable for the lifetime of the flow.

## Make Metrics Operationally Useful

- Use histograms for latency and gauges for queue depth.
- Add low-cardinality labels only.
- Track retries and timeout reasons explicitly.

## Design Health Checks For Decisions

- Keep liveness minimal and fast.
- Keep readiness dependency-aware.
- Surface degraded mode separately from hard failure.

## Log For Humans And Machines

- Keep structured JSON with stable field names.
- Write concise messages with clear action context.
- Never log secrets.

## Alert Readiness

- Define threshold guidance next to each key metric.
- Pair each alert with a primary debug query.
- Validate that alert noise is manageable.

## Validation Loop

- Review telemetry at startup, steady state, and failure modes.
- Confirm observed signals match expected behavior.
- Record gaps and patch incrementally.
