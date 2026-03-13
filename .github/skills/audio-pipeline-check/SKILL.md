---
name: audio-pipeline-check
description: Validate Stackchan audio pipeline assumptions for Opus streaming, including sample rate, frame duration, packet cadence, latency budget, and stream lifecycle checks. Use when asked to review or tune audio quality and real-time behavior.
---

# Audio Pipeline Check

Use this skill when the request is about audio pipeline quality, latency diagnostics, Opus framing consistency, and stream lifecycle correctness.

## Scope

In scope:
- Validate sample rate and frame duration consistency.
- Validate Opus stream framing and packet cadence assumptions.
- Check latency budget breakdown and bottlenecks.
- Check audio lifecycle events: start, chunk, end, cancel.
- Produce actionable fixes for quality and delay issues.

Out of scope:
- Full DSP implementation.
- Hardware driver bring-up.
- Unrelated protocol or UI redesign.

## Inputs To Collect

Collect these before analysis:
- Capture and playback sample_rate_hz.
- frame_duration_ms and channel count.
- Opus bitrate target and packet loss assumption.
- Device-to-server and server-to-device network conditions.
- Timing logs for stt, llm, tts, and playback start.
- Session identifier and representative trace window.

If some inputs are missing, continue with explicit assumptions.

## Target Files

Prioritize these paths when present:
- protocol/websocket/events.md
- protocol/websocket/schemas/
- firmware/runtime/audio/
- firmware/runtime/network/
- server/internal/audio/
- server/internal/session/

## Validation Workflow

1) Verify format assumptions.
- Confirm sample_rate_hz is explicit and consistent.
- Confirm frame_duration_ms is fixed and documented.
- Confirm mono/stereo choice is explicit.

2) Verify framing and pacing.
- Confirm audio chunk cadence matches frame_duration_ms.
- Confirm chunk sequence is monotonic.
- Confirm end-of-stream and cancel semantics exist.

3) Verify buffering strategy.
- Identify capture buffer, network buffer, decode buffer, playback queue.
- Check queue growth and underflow thresholds.
- Check backpressure behavior when downstream is slow.

4) Verify latency budget.
- Break down end-to-end latency into stages:
  - capture
  - encode
  - uplink
  - stt
  - llm
  - tts
  - downlink
  - decode and playback
- Flag stages violating budget.

5) Verify resilience behavior.
- Packet loss and jitter tolerance strategy.
- Reconnect behavior and session recovery.
- Timeout and cancel propagation across stages.

6) Verify observability readiness.
- Ensure logs include session_id and request_id.
- Ensure key metrics exist for queue depth and stage latency.
- Ensure failure events include retryable signals.

## Recommended Baselines

Use these as initial guidance unless project constraints require otherwise:
- sample_rate_hz: 16000 for speech-first pipeline.
- frame_duration_ms: 20 ms.
- channels: 1 (mono).
- opus_bitrate_bps: 16000 to 32000 for conversational speech.

Treat these as defaults, not hard requirements.

## Metric Checklist

Minimum metrics to require:
- capture_to_first_chunk_ms
- ws_uplink_rtt_ms
- stt_latency_ms
- llm_latency_ms
- tts_latency_ms
- tts_to_playback_start_ms
- playback_queue_depth_frames
- ws_disconnect_count

## Common Failure Patterns

- Sample rate mismatch between capture and decode path.
- Frame duration mismatch between sender and receiver assumptions.
- Sequence gaps without recovery handling.
- Buffer growth with no backpressure policy.
- End event missing, causing stuck playback state.

## Fix Recommendation Rules

When suggesting fixes:
- Prefer low-risk configuration changes first.
- Propose one change per experiment cycle.
- Include expected metric movement for each fix.
- Define rollback condition and validation window.

## Deliverable Format

Return outputs in this order:

1) Audio pipeline findings by severity
2) Stage-by-stage latency table
3) Contract and lifecycle issues
4) Recommended fix sequence
5) Residual risk and follow-up metrics

## References

- references/best_practices.md
- references/docs_links.md
- scripts/basic_example.py
