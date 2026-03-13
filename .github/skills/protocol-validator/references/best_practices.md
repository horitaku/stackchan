# Protocol Validation Best Practices

## Validate The Contract, Not Just Examples

- Treat examples as illustrative, not authoritative.
- If schema and docs disagree, flag both and propose one source of truth.

## Prefer Deterministic Rules

- Check explicit required fields and constraints.
- Avoid subjective checks that cannot be repeated consistently.

## Classify Compatibility Clearly

- Additive: new optional fields, new event types, tolerant readers preserved.
- Breaking: rename/remove required fields, semantic repurpose, strict parser changes.

## Enforce One Error Shape

Recommended fields:

- code
- message
- retryable
- details (optional)

## Sequence Integrity Matters

- Require monotonic sequence by direction.
- Require duplicate and out-of-order behavior documentation.
- Require stream end/cancel rules.

## Make Findings Actionable

Each finding should include:

- precise location
- impact
- minimal fix

## Reduce Noise

- Aggregate repeated issues into one concise finding with affected paths list.
- Separate style nits from compatibility risks.
