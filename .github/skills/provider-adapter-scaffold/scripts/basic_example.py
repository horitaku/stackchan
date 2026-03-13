#!/usr/bin/env python3
"""Minimal helper for provider adapter scaffold file layout suggestions."""

import argparse
from pathlib import Path


def build_paths(root: Path, domain: str, vendor: str) -> list[Path]:
    """Build the default scaffold file paths for a provider adapter package."""
    base = root / "providers" / domain / vendor
    return [
        base / "adapter.go",
        base / "client.go",
        base / "config.go",
        base / "errors.go",
        base / "mock_client.go",
        base / "adapter_test.go",
    ]


def main() -> None:
    """Parse arguments and print the recommended scaffold file list."""
    parser = argparse.ArgumentParser(
        description="Show provider adapter scaffold file list"
    )
    parser.add_argument(
        "domain", choices=["stt", "llm", "tts"], help="Provider domain"
    )
    parser.add_argument(
        "vendor", help="Vendor folder name, for example openai"
    )
    parser.add_argument("--root", default=".", help="Repository root path")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    files = build_paths(root, args.domain, args.vendor)

    print("Suggested scaffold files:")
    for file_path in files:
        print(f"- {file_path}")


if __name__ == "__main__":
    main()
