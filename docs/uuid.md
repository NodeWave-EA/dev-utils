# dev uuid

Cryptographic UUIDv4 and RFC 9562 time-ordered UUIDv7 generator, clipboard helper, and UUID metadata inspector.

- **Source:** `commands/uuid.py`
- **Runtime:** Python 3 (standard library only)

---

## Overview

`dev uuid` generates random or time-sortable UUIDs and inspects existing UUID strings:

- **UUIDv4 & UUIDv7 Generation:** Generates random UUIDv4 identifiers by default, or RFC 9562 millisecond-precision time-ordered UUIDv7 identifiers (`--v7`) using Python 3.14's native `uuid.uuid7()` when available or a compliant `os.urandom` fallback on earlier Python 3 versions.
- **Batch & Formatting Controls:** Generate up to 10,000 IDs in one call (`-n`), format in uppercase (`-u`), or strip hyphens (`--compact`).
- **Cross-Desktop Clipboard Integration:** Copies generated output directly to the clipboard (`-c`) via `wl-copy` (Wayland), `xclip`, `xsel` (X11), or `pbcopy` (macOS).
- **UUID Inspector:** Decodes version, variant, URN, and embedded UTC/local creation timestamps from UUIDv7 and UUIDv1 strings (`--inspect`).

---

## Usage

```bash
dev uuid [options]
```

---

## Options

| Flag | Description |
| :--- | :--- |
| `--v4` | Generate random UUIDv4 identifiers (default). |
| `--v7` | Generate RFC 9562 time-ordered UUIDv7 identifiers. |
| `-v, --version {4,7}` | Explicitly select UUID version `4` or `7`. |
| `-n, --count <N>` | Number of UUIDs to generate (`1`–`10000`, default: `1`). |
| `-u, --upper` | Output UUIDs in `UPPERCASE`. |
| `--compact`, `--no-hyphens` | Output 32-character hexadecimal strings without hyphens. |
| `-c, --copy` | Copy generated UUID(s) to the system clipboard. |
| `--inspect <UUID>` | Inspect an existing UUID and decode its version, variant, and timestamp. |
| `-i, --interactive` | Launch interactive guided prompt mode. |
| `-h, --help` | Show help message and exit. |

---

## Examples

```bash
# Generate a single random UUIDv4
dev uuid

# Generate a time-ordered UUIDv7 and copy it to the clipboard
dev uuid --v7 --copy

# Generate 5 uppercase, hyphenless UUIDv7 strings
dev uuid --v7 -n 5 --upper --compact

# Inspect a UUIDv7 to extract its embedded creation timestamp
dev uuid --inspect 019344a8-7c10-7b32-9f1a-4e2c8d1b0a99

# Run in interactive prompt mode
dev uuid -i
```
