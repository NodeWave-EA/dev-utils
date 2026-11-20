#!/usr/bin/env python3
# DESC: Decode a JWT locally, inspect timestamps, and verify expiration
from __future__ import annotations

import argparse
import base64
import binascii
import json
import os
import sys
from datetime import UTC, datetime
from typing import Any


class Style:
    def __init__(self, enabled: bool) -> None:
        self.bold = "\033[1m" if enabled else ""
        self.dim = "\033[2m" if enabled else ""
        self.cyan = "\033[36m" if enabled else ""
        self.green = "\033[32m" if enabled else ""
        self.yellow = "\033[33m" if enabled else ""
        self.red = "\033[31m" if enabled else ""
        self.reset = "\033[0m" if enabled else ""


def decode_b64url_json(segment: str, label: str) -> dict[str, Any]:
    if not segment:
        raise ValueError(f"Empty {label} segment.")
    padded = segment + ("=" * (-len(segment) % 4))
    try:
        raw = base64.urlsafe_b64decode(padded.encode("ascii"))
        parsed = json.loads(raw.decode("utf-8"))
    except (binascii.Error, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"Malformed {label} segment: {exc}") from exc

    if not isinstance(parsed, dict):
        raise TypeError(
            f"Expected JSON object in {label}, got {type(parsed).__name__}."
        )
    return parsed


def format_delta(seconds: float) -> str:
    abs_sec = int(abs(seconds))
    days, rem = divmod(abs_sec, 86400)
    hours, rem = divmod(rem, 3600)
    minutes, secs = divmod(rem, 60)

    parts: list[str] = []
    if days:
        parts.append(f"{days}d")
    if hours:
        parts.append(f"{hours}h")
    if minutes:
        parts.append(f"{minutes}m")
    if secs or not parts:
        parts.append(f"{secs}s")

    human = " ".join(parts[:3])
    return f"in {human}" if seconds >= 0 else f"{human} ago"


def inspect_timestamp(ts: Any, now_utc: datetime) -> dict[str, Any] | None:
    if not isinstance(ts, (int, float)):
        return None
    dt_utc = datetime.fromtimestamp(ts, tz=UTC)
    dt_local = dt_utc.astimezone()
    delta = (dt_utc - now_utc).total_seconds()
    return {
        "unix": ts,
        "utc": dt_utc.strftime("%Y-%m-%d %H:%M:%S UTC"),
        "local": dt_local.strftime("%Y-%m-%d %H:%M:%S %Z"),
        "relative": format_delta(delta),
        "is_past": delta < 0,
    }


def read_token(arg_token: str | None) -> str:
    raw = ""
    if arg_token:
        raw = arg_token
    elif not sys.stdin.isatty():
        raw = sys.stdin.read()
    elif sys.stdout.isatty():
        try:
            raw = input("Paste JWT token: ")
        except (EOFError, KeyboardInterrupt):
            print(file=sys.stderr)
            sys.exit(130)

    token = raw.strip()
    if token.lower().startswith("bearer "):
        token = token[7:].strip()
    return token


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="dev jwt",
        description="Decode a JWT offline, inspect claims, and check expiration.",
    )
    parser.add_argument(
        "token",
        nargs="?",
        help="JWT string (also reads from stdin or interactive prompt if omitted)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output structured JSON (useful for piping to jq)",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit with code 2 if the token is expired or not yet active (nbf)",
    )
    parser.add_argument(
        "--no-color",
        action="store_true",
        help="Disable ANSI color output",
    )
    return parser


def main() -> None:
    args = build_parser().parse_args()
    use_color = (
        sys.stdout.isatty() and not args.no_color and not os.environ.get("NO_COLOR")
    )
    st = Style(use_color)

    token = read_token(args.token)
    if not token:
        print(
            "Error: No JWT provided. Pass a token, pipe via stdin, or run interactively.",
            file=sys.stderr,
        )
        sys.exit(1)

    parts = token.split(".")
    if len(parts) == 5:
        print(
            "Error: Token appears to be an encrypted JWE (5 segments); payload cannot be decoded offline.",
            file=sys.stderr,
        )
        sys.exit(1)
    if len(parts) not in (2, 3):
        print(
            f"Error: Invalid JWT format (expected 3 dot-separated segments, got {len(parts)}).",
            file=sys.stderr,
        )
        sys.exit(1)

    try:
        header = decode_b64url_json(parts[0], "header")
        payload = decode_b64url_json(parts[1], "payload")
    except (ValueError, TypeError) as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)

    now_utc = datetime.now(UTC)
    time_claims = {}
    for key in ("iat", "nbf", "exp", "auth_time"):
        if key in payload:
            info = inspect_timestamp(payload[key], now_utc)
            if info:
                time_claims[key] = info

    warnings: list[str] = []
    alg = str(header.get("alg", ""))
    if alg.lower() == "none" or len(parts) == 2 or not parts[-1]:
        warnings.append("Unsecured JWT (alg=none or missing signature)")

    is_expired = bool(time_claims.get("exp", {}).get("is_past", False))
    not_yet_valid = "nbf" in time_claims and not time_claims["nbf"]["is_past"]

    if is_expired:
        status = "EXPIRED"
    elif not_yet_valid:
        status = "NOT_YET_VALID"
    elif "exp" in time_claims:
        status = "VALID"
    else:
        status = "NO_EXPIRATION"

    if args.json:
        print(
            json.dumps(
                {
                    "status": status,
                    "header": header,
                    "payload": payload,
                    "timestamps": time_claims,
                    "warnings": warnings,
                },
                indent=2,
            )
        )
    else:
        for warn in warnings:
            print(f"{st.yellow}[WARN] {warn}{st.reset}")

        print(f"{st.bold}{st.cyan}--- HEADER ---{st.reset}")
        print(json.dumps(header, indent=2))

        print(f"\n{st.bold}{st.cyan}--- PAYLOAD ---{st.reset}")
        print(json.dumps(payload, indent=2))

        if time_claims or status != "NO_EXPIRATION":
            status_color = {
                "VALID": st.green,
                "EXPIRED": st.red,
                "NOT_YET_VALID": st.yellow,
                "NO_EXPIRATION": st.dim,
            }[status]
            print(
                f"\n{st.bold}{st.cyan}--- STATUS ({status_color}{status}{st.reset}{st.bold}{st.cyan}) ---{st.reset}"
            )
            labels = {
                "iat": "Issued At ",
                "nbf": "Not Before",
                "exp": "Expires At",
                "auth_time": "Auth Time ",
            }
            for key, info in time_claims.items():
                print(
                    f"  {labels[key]} : {info['utc']} | {info['local']} ({st.bold}{info['relative']}{st.reset})"
                )

    if args.check and status in ("EXPIRED", "NOT_YET_VALID"):
        sys.exit(2)


if __name__ == "__main__":
    main()
