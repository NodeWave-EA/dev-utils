#!/usr/bin/env python3
# DESC: Generate UUIDv4/UUIDv7 identifiers or inspect existing UUIDs
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import time
import uuid
from datetime import UTC, datetime


def generate_uuidv7() -> uuid.UUID:
    # Prefer stdlib uuid.uuid7 when running on Python 3.14+
    native_v7 = getattr(uuid, "uuid7", None)
    if callable(native_v7):
        return native_v7()

    timestamp_ms = int(time.time() * 1000)
    rand_bytes = os.urandom(10)

    b = bytearray(16)
    b[0:6] = timestamp_ms.to_bytes(6, "big")
    b[6:8] = rand_bytes[0:2]
    b[6] = (b[6] & 0x0F) | 0x70  # Version 7
    b[8:16] = rand_bytes[2:10]
    b[8] = (b[8] & 0x3F) | 0x80  # RFC 9562 variant (10xx)
    return uuid.UUID(bytes=bytes(b))


def format_uuid(u: uuid.UUID, upper: bool = False, compact: bool = False) -> str:
    val = u.hex if compact else str(u)
    return val.upper() if upper else val.lower()


def copy_to_clipboard(text: str) -> str | None:
    candidates = [
        ["wl-copy"],
        ["xclip", "-selection", "clipboard"],
        ["xsel", "--clipboard", "--input"],
        ["pbcopy"],
    ]
    for cmd in candidates:
        if shutil.which(cmd[0]):
            subprocess.run(cmd, input=text.encode("utf-8"), check=True)
            return cmd[0]
    return None


def inspect_uuid_str(raw: str) -> None:
    try:
        u = uuid.UUID(raw.strip())
    except ValueError as exc:
        print(f"Error: Invalid UUID '{raw}': {exc}", file=sys.stderr)
        sys.exit(1)

    print(f"Canonical : {u}")
    print(f"Compact   : {u.hex}")
    print(f"URN       : {u.urn}")
    print(
        f"Version   : {u.version if u.version is not None else 'Unknown (non-standard)'}"
    )
    print(f"Variant   : {u.variant}")

    if u.version == 7:
        ts_ms = int.from_bytes(u.bytes[0:6], "big")
        dt_utc = datetime.fromtimestamp(ts_ms / 1000.0, tz=UTC)
        dt_local = dt_utc.astimezone()
        print(f"Timestamp : {dt_utc.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]} UTC")
        print(
            f"Local     : {dt_local.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]} {dt_local.tzname()}"
        )
    elif u.version == 1:
        # UUIDv1 timestamp is 100ns intervals since 1582-10-15 00:00:00 UTC
        unix_ts = (u.time - 0x01B21DD213814000) / 1e7
        dt_utc = datetime.fromtimestamp(unix_ts, tz=UTC)
        print(f"Timestamp : {dt_utc.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]} UTC")


def prompt_interactive() -> tuple[str, int, bool, bool, bool]:
    try:
        ver = input("UUID version [4/7] (default: 7): ").strip() or "7"
        if ver not in ("4", "7"):
            print("Error: Version must be 4 or 7.", file=sys.stderr)
            sys.exit(1)
        count_raw = input("How many? (default: 1): ").strip() or "1"
        count = max(1, int(count_raw))
        upper = input("Uppercase? [y/N]: ").strip().lower() in ("y", "yes")
        compact = input("Remove hyphens? [y/N]: ").strip().lower() in ("y", "yes")
        copy = input("Copy to clipboard? [Y/n]: ").strip().lower() not in ("n", "no")
        return ver, count, upper, compact, copy
    except (EOFError, KeyboardInterrupt):
        print(file=sys.stderr)
        sys.exit(130)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="dev uuid",
        description="Generate UUIDv4 or time-ordered UUIDv7 identifiers.",
    )
    ver_group = parser.add_mutually_exclusive_group()
    ver_group.add_argument(
        "--v4",
        action="store_const",
        const="4",
        dest="version",
        help="Generate random UUIDv4 (default)",
    )
    ver_group.add_argument(
        "--v7",
        action="store_const",
        const="7",
        dest="version",
        help="Generate time-ordered UUIDv7",
    )
    ver_group.add_argument(
        "-v",
        "--version",
        choices=["4", "7"],
        dest="version",
        help="UUID version (4 or 7)",
    )

    parser.add_argument(
        "-n",
        "--count",
        type=int,
        default=1,
        help="Number of UUIDs to generate (default: 1)",
    )
    parser.add_argument(
        "-u", "--upper", action="store_true", help="Output in UPPERCASE"
    )
    parser.add_argument(
        "--compact",
        "--no-hyphens",
        action="store_true",
        help="Output 32-character hex without hyphens",
    )
    parser.add_argument(
        "-c", "--copy", action="store_true", help="Copy generated UUID(s) to clipboard"
    )
    parser.add_argument(
        "--inspect",
        metavar="UUID",
        help="Inspect an existing UUID and decode its timestamp",
    )
    parser.add_argument(
        "-i", "--interactive", action="store_true", help="Interactive prompt mode"
    )
    parser.set_defaults(version="4")
    return parser


def main() -> None:
    args = build_parser().parse_args()

    if args.inspect:
        inspect_uuid_str(args.inspect)
        return

    if args.interactive:
        version, count, upper, compact, do_copy = prompt_interactive()
    else:
        version, count, upper, compact, do_copy = (
            args.version,
            args.count,
            args.upper,
            args.compact,
            args.copy,
        )

    if count < 1 or count > 10000:
        print("Error: --count must be between 1 and 10000.", file=sys.stderr)
        sys.exit(1)

    results = [
        format_uuid(
            generate_uuidv7() if version == "7" else uuid.uuid4(),
            upper=upper,
            compact=compact,
        )
        for _ in range(count)
    ]
    output = "\n".join(results)
    print(output)

    if do_copy:
        tool = copy_to_clipboard(output)
        if tool:
            print(f"Copied to clipboard via {tool}.", file=sys.stderr)
        else:
            print(
                "Warning: No clipboard utility found (install wl-copy, xclip, or xsel).",
                file=sys.stderr,
            )


if __name__ == "__main__":
    main()
