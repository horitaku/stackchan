---
name: protocol-designer
description: Design and evolve versioned WebSocket event contracts for Stackchan with required common fields, schema-first definitions, compatibility checks, and concrete examples. Use when asked to define, revise, or review protocol events, payloads, and message versioning.
---

# Protocol Designer

Use this skill when the request is about WebSocket protocol design, event schemas, required envelope fields, event lifecycle, sequence rules, or backward compatibility.

## Scope

In scope:
- Define or revise WebSocket event contracts.
- Add required common fields and validation constraints.
- Add schema and examples for new events.
- Verify compatibility and migration impact.

Out of scope:
- Implement runtime handlers in firmware/server.
- Tune audio codec internals beyond protocol contract fields.
- Refactor unrelated application layers.

## Inputs To Collect

Collect these before design work:
- Goal for the event change.
- Runtime direction: firmware -> server, server -> firmware, or bidirectional.
- Event timing type: stream chunk, control command, state update, request/response.
- Real-time constraints: latency, ordering, acceptable loss.
- Security needs: auth context, sensitive fields, masking policy.
- Rollout needs: additive change or breaking change.

If missing, ask concise questions and continue once minimum is clear.

## Output Artifacts

Always produce:
- Updated event contract summary.
- JSON schema proposal or schema delta.
- Example messages for happy path and error path.
- Compatibility note: additive/breaking and migration steps.

When editing repository files, prefer these paths:
- protocol/websocket/events.md
- protocol/websocket/schemas/
- protocol/versioning.md
- protocol/examples/

## Protocol Baseline

### Required Common Envelope Fields

Every message must include:
- type: stable event type string.
- timestamp: RFC3339 or unix_ms; pick one and document.
- session_id: stable per connection session.
- sequence: monotonic integer within stream direction.

Recommended:
- version: semantic event schema version string.
- request_id: correlation id for request/response flows.
- source: firmware|server|webui.

### Envelope Rules

- Keep envelope stable across all event types.
- Put event-specific payload under payload object.
- Do not overload type to encode dynamic substate.
- Use explicit enums for state values.

## Event Design Workflow

1. Classify event role.
- stream: repeated chunks, order-sensitive, may need chunk_id/end marker.
- control: immediate command, usually ack/fail.
- lifecycle: started/progressed/completed/cancelled/failed.
- telemetry: periodic state report.

2. Define payload contract.
- Enumerate required vs optional fields.
- Add units in field names or schema description.
- Define min/max constraints and enum sets.
- Document nullability explicitly.

3. Define sequencing and idempotency.
- Specify sequence behavior per direction.
- Specify duplicate handling rule.
- Specify late/out-of-order behavior.

4. Define failure semantics.
- Standard error object with code/message/retryable.
- Include originating request_id when available.
- Define what can be retried safely.

5. Define compatibility strategy.
- Additive first: new optional fields or new event type.
- Breaking change requires version bump and migration note.
- Keep old readers tolerant for unknown fields.

6. Provide examples.
- Include at least one minimal valid example.
- Include one full example with optional fields.
- Include one error example where relevant.

## Message Conventions

- Use snake_case for JSON field names.
- Use explicit units: duration_ms, sample_rate_hz.
- Use ISO language tags when language appears.
- Keep payload shallow unless nested structure is meaningful.

## Audio Event Guidance

For audio stream events:
- Distinguish metadata JSON vs binary frame transport.
- If binary frames are used, define companion control events.
- Define frame_duration_ms and sample_rate_hz in metadata.
- Define end-of-stream marker event and cancellation behavior.

## Naming Guidance

Preferred lifecycle naming:
- <domain>.started
- <domain>.partial
- <domain>.final
- <domain>.end
- <domain>.cancel
- <domain>.error

Avoid:
- Verbose ad-hoc names per feature.
- Ambiguous names like update1 or data_event.

## Schema Checklist

Before finalizing, verify:
- Envelope fields are complete and typed.
- Required/optional flags are explicit.
- Constraints exist for numeric and enum fields.
- Error shape is consistent.
- Example messages validate against schema.
- Compatibility note is written.

## Review Checklist

When asked to review protocol changes, focus on:
- Backward compatibility risk.
- Missing constraints causing ambiguous parsing.
- Missing sequence/idempotency rules.
- Missing cancellation and timeout semantics.
- Missing observability fields for troubleshooting.

## Migration Template

Use this template for breaking changes:

1) Change summary:
- Old event:
- New event:
- Reason:

2) Compatibility impact:
- Reader impact:
- Writer impact:
- Deployment order:

3) Rollout steps:
- Step 1: add dual-write or dual-read.
- Step 2: deploy server support.
- Step 3: deploy firmware support.
- Step 4: remove deprecated path.

4) Validation:
- Contract tests updated.
- Replay tests passed.
- Error path tested.

## Deliverable Format

Return outputs in this order:

1) Event contract summary
2) Schema snippets or file diff summary
3) Example messages
4) Compatibility and migration notes
5) Open questions

## References

- references/best_practices.md
- references/docs_links.md
