# Provider Adapter Best Practices

## Keep Provider Boundaries Clear

- Orchestration layer depends on domain interfaces, not SDK details.
- Adapter packages own vendor-specific request and response mapping.

## Fail Fast On Config

- Validate config in constructor.
- Do not defer required-field checks to first request.

## Context And Timeout Discipline

- Require context.Context in all network methods.
- Derive per-call timeout if absent in incoming context.
- Return quickly on cancellation.

## Retry Carefully

- Retry only on transient categories: timeout, 429, 5xx.
- Use bounded exponential backoff and jitter.
- Record retry attempts in structured logs.

## Error Translation Policy

- Translate upstream errors into domain-level errors.
- Preserve wrapped error details for debugging.
- Avoid leaking secrets in error messages.

## Test Strategy

- Use mock client interfaces, not real network calls.
- Cover success, retry success, and permanent failure.
- Verify retry count and timeout behavior deterministically.

## Naming Conventions

- Package path: `providers/{domain}/{vendor}`
- Keep method names domain oriented: Transcribe, Generate, Synthesize
- Keep file names stable: adapter.go, client.go, config.go, errors.go
