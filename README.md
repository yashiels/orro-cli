# orro — control your standing desk from the terminal

A modern Python CLI for Orro and other Tuya-based standing desks. Move up, down, recall presets, target exact heights — all from the command line.

- **LAN-first, cloud fallback** — sub-second local control via TinyTuya; automatic fallback to Tuya Cloud API
- **Exact height targeting** — `orro height 110` drives to 110 cm and stops; preset slots for sit/stand
- **1Password integration** — reads credentials from vault if the 1Password CLI is installed

## Install

```bash
brew install yashiels/tap/orro  # auto-taps yashiels/tap
```

Or install from PyPI:

```bash
pip install orro
# or: pipx install orro
```

Build from source:

```bash
git clone https://github.com/yashiels/orro-cli.git
cd orro-cli
pip install .
```

## Quick Start

```bash
orro config init    # interactive first-time setup
orro status         # show desk height, connection, protocol
orro stand          # move to standing preset
orro sit            # move to sitting preset
orro height 110     # move to exact height in cm
```

## Commands

| Command              | Description                                          |
|----------------------|------------------------------------------------------|
| `orro status`        | Show desk status (height, connection path, protocol) |
| `orro up`            | Move desk up (hold until `orro stop`)                |
| `orro down`          | Move desk down (hold until `orro stop`)              |
| `orro stop`          | Stop desk movement                                   |
| `orro sit`           | Move to configured sit preset (default: mem1)        |
| `orro stand`         | Move to configured stand preset (default: mem3)      |
| `orro goto <slot>`   | Move to a specific memory preset (mem1–mem4)         |
| `orro height <cm>`   | Move to exact height in cm                           |
| `orro presets`       | Show preset configuration and device memory heights  |
| `orro config show`   | Print current config (secrets redacted)              |
| `orro config path`   | Print config file location                           |
| `orro config set`    | Set config values                                    |
| `orro config init`   | Interactive first-time setup                         |

Global flags: `--output table|json`, `--cloud`, `--verbose/-v`, `--quiet/-q`, `--config <path>`, `--version`

## Configuration

Priority: flags → environment variables → config file → 1Password → defaults.

Config file: `~/.config/orro/config.yaml` (permissions `0600`)

```yaml
endpoint: "https://openapi.tuyaeu.com"
access_id: "your_access_id"
access_secret: "your_access_secret"
device_id: "your_device_id"
local_key: "your_local_key"
lan_ip: "192.168.10.95"
lan_version: "3.4"
presets:
  sit: mem1
  stand: mem3
```

Environment variables: `ORRO_ENDPOINT`, `ORRO_ACCESS_ID`, `ORRO_ACCESS_SECRET`, `ORRO_DEVICE_ID`, `ORRO_LOCAL_KEY`, `ORRO_LAN_IP`, `ORRO_LAN_VERSION`.

### Getting Tuya Credentials

1. Add desk to Tuya Smart or SmartLife app
2. Create free account at [iot.tuya.com](https://iot.tuya.com), create Cloud Project
3. Subscribe to IoT Core and Authorization Token Management APIs
4. Link SmartLife account under Devices → Link Tuya App Account
5. Note Access ID, Access Secret, API Endpoint
6. Find Device ID in device list; Local Key in device details

### Supported Desks

Tested with Orro standing desks (Tuya device category `sjz`). Should work with any Tuya-based standing desk using standard DP codes.

## Disclaimer

Not affiliated with Orro or Tuya. Talks to the Tuya IoT Platform API. Use at your own risk.

## Development

```bash
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
make lint    # ruff check
make fmt     # ruff format
make test    # pytest
```

Releases are automated via GitHub Actions. Go to **Actions → Ship**, pick `patch`, `minor`, or `major` — it bumps the version, publishes to PyPI, creates a GitHub release, and updates the [Homebrew tap](https://github.com/yashiels/homebrew-tap).

## License

MIT — Yashiel Sookdeo
