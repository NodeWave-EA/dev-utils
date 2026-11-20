# `scripts/make-executable.sh` — Executable Permissions Helper

Applies executable (`+x`) permissions across repository entrypoints, utilities in `commands/`, and helper scripts in `scripts/`.

- **Source:** `scripts/make-executable.sh`
- **Runtime:** Bash

---

## Usage

```bash
./scripts/make-executable.sh
```

---

## When to Run

- After cloning the repository on a filesystem or archive that stripped file modes.
- After adding a new script under `commands/` or `scripts/`.
- After pulling upstream updates (`git pull`) that introduce new command files.

*(Note: `bin/dev.sh` also attempts to auto-heal missing `+x` permissions at runtime and falls back to invoking the registered interpreter directly if the filesystem is mounted `noexec`.)*
