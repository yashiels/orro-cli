# Orro Desk

Python CLI for the Orro / Tuya SmartLife standing desk.

## Files

- `orro.py` - executable CLI, symlinked as `orro`
- `discover_tuya.py` - Tuya Cloud discovery script
- `dp-discovery.json` - raw Tuya Cloud responses and normalized DP map
- `dp-discovery.example.json` - redacted discovery summary suitable for Git
- `.venv/` - project Python environment with `tinytuya` and `tuya-connector-python`

## Credentials

Credentials are loaded at runtime from 1Password vault `OpenClaw`.

Required item: `Tuya IoT Platform`

- `Access ID`
- `Access Secret`
- `API Endpoint`
- `Device ID`
- `Local Key`

Optional item fields:

- `LAN IP` - skips TinyTuya LAN discovery
- `LAN Version` - Tuya LAN protocol version, for example `3.4`
- `LAN DP Map` - JSON object overriding local DP IDs, for example:

```json
{"child_lock":1,"move_up":18,"move_down":19,"height_display":20,"memory_location":22}
```

- `Presets` - JSON object overriding preset mapping:

```json
{"sit":"mem1","stand":"mem2"}
```

## Discovery

Run:

```bash
cd ~/Developer/orro-desk
.venv/bin/python discover_tuya.py
```

The requested singular Tuya path `/v1.0/devices/{device_id}/specification` currently returns `uri path invalid`; `discover_tuya.py` records that response and uses Tuya's plural `/v1.0/devices/{device_id}/specifications` plus `/v1.0/iot-03/devices/{device_id}/specification`.

`dp-discovery.json` is intentionally ignored by Git because it contains raw device responses. Commit only redacted summaries such as `dp-discovery.example.json`.

Current cloud discovery for this device exposes only:

- `child_lock`

The Tuya Query Properties endpoint returned the full 31-code DP map in `dp-discovery.json` under `dp_map` and `shadow_dp_map`.
LAN discovery found the desk at `192.168.10.95` using Tuya protocol `3.4`. The control mappings used by `orro.py` are:

```json
{"child_lock":6,"move_up":150,"move_down":151,"height_display":152,"memory_location":155}
```

Known Tuya `sjz` standing-desk command/status codes observed for similar devices include `move_up`, `move_down`, `memory_location`, and `height_display`; see the Home Assistant community report: https://community.home-assistant.io/t/tuya-new-device-standing-desk/597025 and the Ergolutions local DP notes: https://www.benjamin-schieder.de/blog/2024/09/15/using-an-ergolutions-tuya-desk-controller-with-home-assistant.html

## Usage

```bash
orro status --json
orro up
orro down
orro stop
orro sit
orro stand
orro presets
orro height 110
orro --cloud status --json
```

Default presets:

- `sit` -> `mem1`
- `stand` -> `mem3`

Current device preset heights decoded from DP `159` (`memory_height`):

- `mem1`: 76.0 cm
- `mem2`: 78.0 cm
- `mem3`: 111.8 cm
- `mem4`: 76.0 cm

## LAN vs Cloud

By default, `orro.py` tries LAN first through TinyTuya using the device ID, local key, and either `LAN IP` or network discovery. If LAN setup fails, commands fall back to Tuya Cloud automatically.

Use `--cloud` to skip LAN and force Tuya Cloud:

```bash
orro --cloud stand
```

LAN status and control work with the discovered IP, local key, and DP map. `up`, `down`, and `stop` use local boolean DPs `150` and `151`; `sit` and `stand` use local enum DP `155` (`memory_location`). Cloud control uses Tuya function codes and does not require a LAN IP, but this project currently rejects the unexposed movement codes, so LAN is the reliable control path.
