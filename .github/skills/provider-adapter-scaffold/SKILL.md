---
name: provider-adapter-scaffold
description: Scaffold Stackchan provider adapters for STT, LLM, and TTS with interface-first design, mockable clients, timeout and retry wiring, and test-ready project structure. Use when asked to add or refactor provider adapters.
---

# Provider Adapter Scaffold

Use this skill when the request is about creating or refactoring provider adapters under providers/ and server/internal/providers/.

## Scope

In scope:
- Scaffold adapter structure for stt, llm, tts.
- Define provider interfaces and constructor signatures.
- Add timeout, context cancel, retry boundaries.
- Add mock-friendly seams for testing.
- Produce minimal compile-ready skeletons.

Out of scope:
- Full business feature implementation.
- Protocol event redesign.
- Firmware-side hardware behavior changes.

## Inputs To Collect

Collect this before scaffolding:
- Provider domain: stt, llm, tts.
- Vendor name: openai, voicevox, gemini, ollama, others.
- Required operations and return data.
- Timeout and retry policy.
- Error mapping policy.
- Target package path and naming preference.

If details are missing, choose conservative defaults and state assumptions.

## Default Paths

Use these paths unless user asks otherwise:
- providers/<domain>/<vendor>/
- server/internal/providers/<domain>/

Typical files:
- adapter.go
- client.go
- config.go
- errors.go
- mock_client.go
- adapter_test.go

## Design Rules

1. Interface first.
- Define minimal interface consumed by orchestration layer.
- Keep transport implementation behind client interface.

2. Context everywhere.
- Every external call accepts context.Context.
- Respect cancellation and deadlines.

3. Explicit configuration.
- Put endpoint, model, timeout, retry in Config.
- Validate Config at constructor time.

4. Error translation.
- Convert vendor-specific errors into domain-level errors.
- Preserve root cause in wrapped error for diagnostics.

5. Retry boundaries.
- Retry only transient failures.
- Use bounded attempts and exponential backoff.
- Do not retry context cancellation or validation failures.

6. Testability.
- Inject client interface via constructor.
- Keep side effects at the edge.
- Add table-driven tests for success and error paths.

## Scaffold Workflow

1) Confirm domain and vendor.
2) Create package layout.
3) Create Config and validation.
4) Define domain interface.
5) Implement adapter skeleton.
6) Add vendor client abstraction.
7) Add error mapping helpers.
8) Add mock and baseline tests.
9) Return next-implementation checklist.

## Interface Template Guidance

Recommended shape:
- New(cfg Config, c VendorClient) (*Adapter, error)
- Method(ctx context.Context, input Input) (Output, error)

Keep interfaces small:
- STT: Transcribe
- LLM: Generate or Chat
- TTS: Synthesize

## Config Template Guidance

Config should include:
- BaseURL
- APIKey or token reference
- Model
- Timeout
- MaxRetries
- RetryBaseDelay
- RetryMaxDelay

Validation must check:
- required fields present
- positive timeout and retry values
- supported model or operation mode where applicable

## Error Model Guidance

Domain-level sentinel errors:
- ErrInvalidConfig
- ErrUnauthorized
- ErrRateLimited
- ErrUpstreamUnavailable
- ErrTimeout
- ErrBadResponse

Keep mapping in one function per adapter package.

## Test Checklist

Minimum tests:
- constructor validation failure
- success path with mocked client
- timeout propagation
- retry on transient error then success
- non-retryable error immediate fail
- error mapping correctness

## Deliverable Format

Return outputs in this order:

1) Created file list
2) Public interface summary
3) Config and retry policy summary
4) Test coverage summary
5) Next implementation steps

## References

- references/best_practices.md
- references/docs_links.md
- scripts/basic_example.py
