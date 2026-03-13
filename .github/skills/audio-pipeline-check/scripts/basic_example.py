#!/usr/bin/env python3
"""Compute basic audio pipeline expectations from frame settings."""

import argparse


def packets_per_second(frame_duration_ms: int) -> float:
    """Return expected packet cadence for the given frame duration."""
    if frame_duration_ms <= 0:
        raise ValueError("frame_duration_ms must be positive")
    return 1000.0 / frame_duration_ms


def pcm_bitrate_bps(
    sample_rate_hz: int, channels: int, bits_per_sample: int
) -> int:
    """Return uncompressed PCM bitrate in bits per second."""
    if sample_rate_hz <= 0 or channels <= 0 or bits_per_sample <= 0:
        raise ValueError(
            "sample_rate_hz, channels, bits_per_sample must be positive"
        )
    return sample_rate_hz * channels * bits_per_sample


def main() -> None:
    """Parse arguments and print expected packet cadence and bitrate."""
    parser = argparse.ArgumentParser(
        description="Show packet cadence and PCM bitrate for audio settings"
    )
    parser.add_argument("--sample-rate-hz", type=int, default=16000)
    parser.add_argument("--channels", type=int, default=1)
    parser.add_argument("--bits-per-sample", type=int, default=16)
    parser.add_argument("--frame-duration-ms", type=int, default=20)
    args = parser.parse_args()

    pps = packets_per_second(args.frame_duration_ms)
    bitrate = pcm_bitrate_bps(
        args.sample_rate_hz, args.channels, args.bits_per_sample
    )

    print(f"packets_per_second={pps:.2f}")
    print(f"pcm_bitrate_bps={bitrate}")


if __name__ == "__main__":
    main()
