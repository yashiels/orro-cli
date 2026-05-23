# AGENTS.md — orro

CLI for Tuya-based standing desks. Python 3.12+, uses TinyTuya for LAN and requests for Cloud API.

## Structure

```
orro/
  __init__.py    — version string
  __main__.py    — python -m orro entry
  cli.py         — argument parsing, command dispatch
  config.py      — Config dataclass, YAML/env/1Password loading
  cloud.py       — TuyaCloud HTTP client with HMAC signing
  lan.py         — TuyaLan local device control via TinyTuya
  desk.py        — high-level desk operations (status, movement, presets, height)
  output.py      — table/JSON output formatting
scripts/
  discover_tuya.py — DP discovery script (standalone, reads from 1Password)
```

## Build / Test

```bash
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
make lint
make test
```

## Key Design Decisions

- **LAN-first**: always try local TinyTuya connection before falling back to Cloud API
- **Config layering**: CLI flags > env vars > YAML file > 1Password > defaults
- **No async**: synchronous requests throughout; desk operations are inherently sequential
- **DP codes are desk-specific**: defaults work for Orro desks; override via config for other Tuya `sjz` devices
- **Height targeting polls at 500ms**: stops within 5mm tolerance, 30s timeout, always sends stop command in finally block

## Config

Config file: `~/.config/orro/config.yaml`. Credentials in 1Password vault `OpenClaw`, item `Tuya IoT Platform`.

## Constraints

- Do not switch from `requests` to httpx/aiohttp
- Do not remove `tinytuya` dependency
- Keep 1Password as a config source
- The `scripts/discover_tuya.py` is standalone (does not import from orro)
