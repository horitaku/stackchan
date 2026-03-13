# Protocol Design Best Practices

## Stability First

- Prefer additive evolution over replacement.
- Keep unknown-field tolerant readers.
- Keep envelope shape stable across event types.

## Explicitness Over Guessing

- Define required fields and constraints in schema.
- Use explicit enum values for states.
- Include units in numeric field names.

## Sequence And Recovery

- Define monotonic sequence per direction.
- Define duplicate and out-of-order handling.
- Define reconnect and session resume behavior.

## Error Contract

Use one shared error payload shape:

- code: short machine-readable code.
- message: human-readable summary.
- retryable: true or false.
- details: optional object for diagnostics.

## Security And Privacy

- Avoid sending secrets in protocol payloads.
- Tag sensitive fields and apply masking in logs.
- Include minimum identifiers needed for tracing.

## Testing Expectations

- Validate examples against schemas.
- Add compatibility tests for old/new versions.
- Add replay tests for stream events.

## Anti-Patterns

- Embedding version in event name only.
- Using free-form strings for important states.
- Mixing transport metadata and business payload without boundaries.
