---
name: protocol-validator
description: Validate Stackchan WebSocket protocol contracts with schema checks, envelope requirements, compatibility review, and actionable fix reports. Use when asked to verify protocol events, schemas, examples, or migration safety.
---

# Protocol Validator

Use this skill when the request is about checking protocol quality, schema validity, envelope field compliance, backward compatibility, and migration safety.

## Scope

In scope:
- Validate event contracts and schema consistency.
- Detect missing required envelope fields.
- Verify examples against schema intent.
- Review additive vs breaking changes.
- Produce actionable findings and fix suggestions.

Out of scope:
- Implement firmware/server runtime logic.
- Optimize non-protocol application behavior.
- Change provider internals unrelated to message contracts.

## Inputs To Collect

Collect these before validation:
- Changed files or target files.
- Target protocol version and previous baseline.
- Direction and event lifecycle expectations.
- Rollout assumptions: dual-read, dual-write, cutover strategy.

If baseline is missing, use best available previous contract snapshot and state assumptions.

## Files To Inspect

Prioritize these locations:
- protocol/websocket/events.md
- protocol/websocket/schemas/
- protocol/versioning.md
- protocol/examples/

Optional supporting files:
- server/internal/protocol/
- firmware/protocol/

## Validation Workflow

1. Contract inventory.
- List event types and associated schemas.
- Ensure each event has clear purpose and payload shape.

2. Envelope compliance.
- Verify required fields exist in all events:
  - type
  - timestamp
  - session_id
  - sequence
- Check type stability and timestamp format consistency.

3. Payload schema quality.
- Verify required vs optional fields are explicit.
- Verify enums and numeric constraints exist when needed.
- Verify nullability rules are clear.
- Verify units are explicit for numeric fields.

4. Sequence and idempotency semantics.
- Confirm monotonic sequence rule per direction.
- Confirm duplicate, late, out-of-order handling is defined.
- Confirm cancel/end semantics for stream-like events.

5. Error contract consistency.
- Ensure a shared error payload shape exists.
- Expect code, message, retryable, and optional details.
- Ensure errors reference request_id when applicable.

6. Example consistency.
- Compare examples with schema fields and required envelope.
- Flag examples that are invalid, ambiguous, or outdated.

7. Compatibility review.
- Classify each change as additive or breaking.
- Verify versioning and migration notes for breaking changes.
- Ensure unknown-field tolerant reading remains possible.

8. Rollout readiness.
- Verify deployment order is safe.
- Verify fallback and rollback behavior is documented.

## Severity Model

Use this severity model in findings:
- Critical: likely production breakage or data loss.
- High: strong compatibility or runtime risk.
- Medium: ambiguity or maintainability issue.
- Low: style or minor clarity issue.

## Finding Format

Report findings in this order:

1) Findings
- [severity] path: reason
- impact
- recommended fix

2) Open questions
- assumptions or missing info that blocks certainty

3) Validation summary
- pass/fail per checklist category

4) Suggested next patch set
- minimal ordered list of safe edits

## Checklist

### A. Envelope
- Every event includes type, timestamp, session_id, sequence.
- Envelope shape is stable across event types.
- No payload field duplicates envelope meaning.

### B. Schema
- Required/optional definitions are explicit.
- Constraints exist for range, length, enum, and pattern when relevant.
- Field names use snake_case and unit suffixes where needed.

### C. Lifecycle
- started/partial/final/end/cancel/error semantics are clear.
- Stream completion and cancellation behavior is explicit.

### D. Errors
- Shared error object shape is used consistently.
- retryable policy is defined.

### E. Compatibility
- Additive changes avoid breaking old readers.
- Breaking changes have version bump and migration notes.
- Deprecation path and removal timeline are stated.

### F. Examples
- Minimal valid example exists.
- Full example with optional fields exists.
- Error example exists when event can fail.

## Common Failure Patterns

- Missing sequence definition for one direction.
- Required field listed in docs but absent from schema.
- Example contains fields not defined in schema.
- Breaking rename shipped without version strategy.
- Free-form status strings instead of enums.

## Migration Safety Review Template

1) Change classification:
- additive or breaking

2) Risk summary:
- reader risk
- writer risk
- rollout risk

3) Safe rollout plan:
- Step 1: dual-read or compatibility parser.
- Step 2: deploy writer update.
- Step 3: monitor and verify.
- Step 4: deprecate old path.

4) Exit criteria:
- contract tests pass
- replay tests pass
- failure-mode tests pass

## Deliverable Format

Return outputs in this order:

1) Validation findings by severity
2) Checklist pass/fail matrix
3) Compatibility and migration assessment
4) Minimal fix plan
5) Residual risks

## References

- references/best_practices.md
- references/docs_links.md
