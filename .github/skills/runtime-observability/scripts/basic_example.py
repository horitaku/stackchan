#!/usr/bin/env python3
"""Build a minimal structured log line with correlation fields."""

import argparse
import json
from datetime import datetime, timezone


def iso_timestamp() -> str:
    """Return current UTC timestamp in ISO 8601 format."""
    return datetime.now(timezone.utc).isoformat()


def make_log_event(
    level: str,
    component: str,
    message: str,
    session_id: str,
    request_id: str,
) -> dict[str, str]:
    """Create a structured log event dictionary for runtime diagnostics."""
    return {
        "timestamp": iso_timestamp(),
        "level": level,
        "component": component,
        "message": message,
        "session_id": session_id,
        "request_id": request_id,
    }


def main() -> None:
    """Parse arguments and print a structured JSON log event."""
    parser = argparse.ArgumentParser(
        description="Print runtime observability log event as JSON"
    )
    parser.add_argument("--level", default="info")
    parser.add_argument("--component", default="server.session")
    parser.add_argument("--message", default="session started")
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--request-id", required=True)
    args = parser.parse_args()

    event = make_log_event(
        level=args.level,
        component=args.component,
        message=args.message,
        session_id=args.session_id,
        request_id=args.request_id,
    )
    print(json.dumps(event, ensure_ascii=True))


if __name__ == "__main__":
    main()
    main()
