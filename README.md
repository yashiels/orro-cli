# Orro

CLI for Tuya-based standing desks. Control your desk from the terminal — move up, down, go to presets, set a specific height, or check status. Talks to the desk over LAN first (fast, no cloud roundtrip) and falls back to the Tuya Cloud API automatically.

## Features

- **LAN-first control** via [TinyTuya](https://github.com/jasonacox/tinytuya) — sub-second response
- **Cloud fallback** via the Tuya IoT Platform API when LAN isn't available
- **Preset commands** — `orro sit` / `orro stand` mapped to device memory slots
- **Height targeting** — `orro height 110` moves to 110 cm and stops
- **Status reporting** — current height, connection path, device state
- **Flexible credentials** — 1Password CLI or plain environment variables

## Requirements

- Python 3.12+
- A Tuya-based standing desk (category `sjz` — tested with Orro, should work with other Tuya smart desks)
- A [Tuya IoT Platform](https://iot.tuya.com/) account with a Cloud project linked to your device
- Either the [1Password CLI](https://developer.1password.com/docs/cli/) (`op`) **or** environment variables for credentials

## Installation

```bash
# Clone and install
git clone https://github.com/apex-skyner/orro.git
cd orro
pip install .

# Or with pipx for isolated install
pipx install .
```

You can also run without installing:

```bash
python -m orro status
```

## Configuration

Orro needs your Tuya IoT Platform credentials. You can provide them via environment variables or 1Password.

### Option A: Environment Variables

```bash
export ORRO_ENDPOINT="https://openapi.tuyaeu.com"   # Your Tuya API endpoint
export ORRO_ACCESS_ID="your_access_id"
export ORRO_ACCESS_SECRET="your_access_secret"
export ORRO_DEVICE_ID="your_device_id"

# Optional — enables LAN control (recommended)
export ORRO_LOCAL_KEY="your_local_key"
export ORRO_LAN_IP="192.168.1.100"        # Skip LAN discovery
export ORRO_LAN_VERSION="3.4"             # Tuya protocol version
```

### Option B: 1Password CLI

If you have the [1Password CLI](https://developer.1password.com/docs/cli/) installed, Orro reads credentials from:

- **Vault:** `OpenClaw`
- **Item:** `Tuya IoT Platform`
- **Fields:** `API Endpoint`, `Access ID`, `Access Secret`, `Device ID`, `Local Key`, `LAN IP`, `LAN Version`

Create the item:

```bash
op item create \
  --vault "OpenClaw" \
  --title "Tuya IoT Platform" \
  --category login \
  "API Endpoint=https://openapi.tuyaeu.com" \
  "Access ID=your_access_id" \
  "Access Secret=your_access_secret" \
  "Device ID=your_device_id" \
  "Local Key=your_local_key"
```

Environment variables take priority over 1Password when both are set.

### Optional Configuration

Override LAN DP mappings or preset names with JSON:

```bash
# Custom DP map (if your desk uses different DP IDs)
export ORRO_LAN_DP_MAP='{"child_lock":1,"move_up":18,"move_down":19,"height_display":20,"memory_location":22}'

# Custom presets
export ORRO_PRESETS='{"sit":"mem1","stand":"mem2"}'
```

## Usage

```bash
orro status          # Current desk state (height, connection path)
orro status --json   # Machine-readable JSON output
orro up              # Move up (hold until stop)
orro down            # Move down (hold until stop)
orro stop            # Stop movement
orro sit             # Go to sit preset (default: mem1)
orro stand           # Go to stand preset (default: mem3)
orro height 110      # Move to 110 cm and stop
orro presets         # Show configured and device preset heights
orro --cloud status  # Force Tuya Cloud API (skip LAN)
```

Every command accepts `--json` for structured output and `--cloud` to bypass LAN.

## LAN vs Cloud

By default, Orro tries **LAN first** using TinyTuya:

1. Connects to the desk directly on your local network
2. If LAN fails (no local key, device offline, firewall), falls back to the Tuya Cloud API
3. Use `--cloud` to skip LAN entirely

LAN is faster (~100ms vs ~500ms) and doesn't depend on Tuya's servers. For LAN to work you need the device's `Local Key` — get it from the Tuya IoT Platform or by running the discovery script.

## Discovery

The `scripts/discover_tuya.py` script queries Tuya Cloud for your device's full DP (data point) map. This is useful for finding DP IDs if your desk uses different mappings than the defaults.

```bash
cd orro
python scripts/discover_tuya.py
```

This writes `dp-discovery.json` with the complete device specification. The file is gitignored because it contains device secrets. See `dp-discovery.example.json` for the expected structure.

## Supported Desks

Tested with the **Orro** standing desk (Tuya category `sjz`). Should work with any Tuya-connected standing desk that uses the standard DP codes:

- `move_up` / `move_down` — boolean movement controls
- `height_display` — current height in mm
- `memory_location` — preset slot selector (`mem1`–`mem4`)
- `child_lock` — safety lock (toggled off before movement)

If your desk uses different DP IDs, override them with `ORRO_LAN_DP_MAP` or the 1Password `LAN DP Map` field.

## Getting Tuya Credentials

1. Create an account on the [Tuya IoT Platform](https://iot.tuya.com/)
2. Create a Cloud Project (choose your data centre region)
3. Link your Smart Life / Tuya Smart app account under **Devices → Link Tuya App Account**
4. Find your device in the device list — note the **Device ID**
5. Your project page shows the **Access ID** and **Access Secret**
6. The **API Endpoint** depends on your region (e.g. `https://openapi.tuyaeu.com` for Europe)
7. The **Local Key** appears in the device details

## License

[MIT](LICENSE)
