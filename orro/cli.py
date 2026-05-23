#!/usr/bin/env python3
"""Control a Tuya-based standing desk from the command line."""

import argparse
import base64
import contextlib
import hashlib
import hmac
import json
import os
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlencode

import requests
import tinytuya


VAULT = "OpenClaw"
TUYA_ITEM = "Tuya IoT Platform"

DEFAULT_PRESETS = {"sit": "mem1", "stand": "mem3"}

# Verified LAN status DPs for this desk. Movement/preset command DPs are not
# exposed in the passive status response, so they are intentionally not guessed.
# Override with 1Password field "LAN DP Map" or ORRO_LAN_DP_MAP after verifying.
DEFAULT_LAN_DPS = {
    "child_lock": 6,
    "move_up": 150,
    "move_down": 151,
    "height_display": 152,
    "memory_location": 155,
}


def _op_available() -> bool:
    """Return True if the 1Password CLI (``op``) is on PATH."""
    return shutil.which("op") is not None


def op_read(item: str, field: str, required: bool = True) -> str | None:
    """Read a field from the 1Password vault.

    Returns ``None`` instead of raising when *required* is ``False`` and the
    field cannot be read (or ``op`` is not installed).
    """
    if not _op_available():
        if required:
            raise RuntimeError(
                f"1Password CLI (op) not found and no environment variable fallback "
                f"for {item}/{field}"
            )
        return None
    ref = f"op://{VAULT}/{item}/{field}"
    result = subprocess.run(
        ["op", "read", ref],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if result.returncode == 0:
        return result.stdout.rstrip("\n")
    if required:
        raise RuntimeError(f"Could not read {ref}: {result.stderr.strip()}")
    return None


def canonical_query(params: dict[str, Any] | None) -> str:
    if not params:
        return ""
    pairs: list[tuple[str, str]] = []
    for key in sorted(params):
        value = params[key]
        if value is None:
            continue
        if isinstance(value, (list, tuple)):
            for item in value:
                pairs.append((key, str(item)))
        else:
            pairs.append((key, str(value)))
    return urlencode(pairs)


def _env_or_op(env_key: str, item: str, field: str, required: bool = True) -> str | None:
    """Try environment variable first, then 1Password, then error or None."""
    value = os.environ.get(env_key)
    if value:
        return value
    return op_read(item, field, required=required)


@dataclass
class Config:
    endpoint: str
    access_id: str
    access_secret: str
    device_id: str
    local_key: str | None
    lan_ip: str | None
    lan_version: str | None
    lan_dps: dict[str, int]
    presets: dict[str, str]

    @classmethod
    def load(cls) -> "Config":
        """Load configuration from environment variables or 1Password.

        Environment variables (checked first):
            ORRO_ENDPOINT       — Tuya API endpoint URL
            ORRO_ACCESS_ID      — Tuya IoT Platform Access ID
            ORRO_ACCESS_SECRET  — Tuya IoT Platform Access Secret
            ORRO_DEVICE_ID      — Tuya device ID
            ORRO_LOCAL_KEY      — device local encryption key (optional)
            ORRO_LAN_IP         — device LAN IP address (optional, skips discovery)
            ORRO_LAN_VERSION    — Tuya LAN protocol version (optional, e.g. "3.4")
            ORRO_LAN_DP_MAP     — JSON object overriding LAN DP IDs (optional)
            ORRO_PRESETS        — JSON object overriding preset mapping (optional)

        Falls back to 1Password vault ``OpenClaw``, item ``Tuya IoT Platform``
        when environment variables are not set and ``op`` is available.
        """
        lan_dps = dict(DEFAULT_LAN_DPS)
        raw_map = os.environ.get("ORRO_LAN_DP_MAP") or op_read(TUYA_ITEM, "LAN DP Map", required=False)
        if raw_map:
            lan_dps.update({key: int(value) for key, value in json.loads(raw_map).items()})

        presets = dict(DEFAULT_PRESETS)
        raw_presets = os.environ.get("ORRO_PRESETS") or op_read(TUYA_ITEM, "Presets", required=False)
        if raw_presets:
            presets.update({key: str(value) for key, value in json.loads(raw_presets).items()})

        endpoint = _env_or_op("ORRO_ENDPOINT", TUYA_ITEM, "API Endpoint")
        access_id = _env_or_op("ORRO_ACCESS_ID", TUYA_ITEM, "Access ID")
        access_secret = _env_or_op("ORRO_ACCESS_SECRET", TUYA_ITEM, "Access Secret")
        device_id = _env_or_op("ORRO_DEVICE_ID", TUYA_ITEM, "Device ID")

        if not all([endpoint, access_id, access_secret, device_id]):
            missing = [
                name
                for name, val in [
                    ("ORRO_ENDPOINT", endpoint),
                    ("ORRO_ACCESS_ID", access_id),
                    ("ORRO_ACCESS_SECRET", access_secret),
                    ("ORRO_DEVICE_ID", device_id),
                ]
                if not val
            ]
            raise RuntimeError(
                f"Missing required configuration: {', '.join(missing)}. "
                "Set environment variables or install 1Password CLI (op)."
            )

        return cls(
            endpoint=endpoint or "",
            access_id=access_id or "",
            access_secret=access_secret or "",
            device_id=device_id or "",
            local_key=_env_or_op("ORRO_LOCAL_KEY", TUYA_ITEM, "Local Key", required=False),
            lan_ip=os.environ.get("ORRO_LAN_IP") or op_read(TUYA_ITEM, "LAN IP", required=False),
            lan_version=os.environ.get("ORRO_LAN_VERSION") or op_read(TUYA_ITEM, "LAN Version", required=False),
            lan_dps=lan_dps,
            presets=presets,
        )


class TuyaCloud:
    def __init__(self, config: Config) -> None:
        self.endpoint = config.endpoint.rstrip("/")
        self.access_id = config.access_id
        self.access_secret = config.access_secret.encode()
        self.device_id = config.device_id
        self.access_token: str | None = None

    @staticmethod
    def _body_text(body: Any | None) -> str:
        return "" if body is None else json.dumps(body, separators=(",", ":"))

    def _sign(
        self,
        method: str,
        path: str,
        params: dict[str, Any] | None = None,
        body: Any | None = None,
    ) -> dict[str, str]:
        query = canonical_query(params)
        url_path = path + (f"?{query}" if query else "")
        body_text = self._body_text(body)
        body_hash = hashlib.sha256(body_text.encode()).hexdigest()
        string_to_sign = "\n".join([method.upper(), body_hash, "", url_path])
        timestamp = str(int(time.time() * 1000))
        token_part = self.access_token or ""
        sign_source = f"{self.access_id}{token_part}{timestamp}{string_to_sign}"
        signature = hmac.new(self.access_secret, sign_source.encode(), hashlib.sha256).hexdigest().upper()
        headers = {
            "client_id": self.access_id,
            "sign": signature,
            "t": timestamp,
            "sign_method": "HMAC-SHA256",
            "Content-Type": "application/json",
        }
        if self.access_token:
            headers["access_token"] = self.access_token
        return headers

    def request(
        self,
        method: str,
        path: str,
        params: dict[str, Any] | None = None,
        body: Any | None = None,
        raise_on_error: bool = True,
    ) -> Any:
        response = requests.request(
            method.upper(),
            f"{self.endpoint}{path}",
            params=params,
            data=self._body_text(body) if body is not None else None,
            headers=self._sign(method, path, params, body),
            timeout=30,
        )
        try:
            payload = response.json()
        except ValueError:
            response.raise_for_status()
            return response.text
        if raise_on_error and (not response.ok or payload.get("success") is False):
            raise RuntimeError(f"Tuya API {method} {path} failed: HTTP {response.status_code}: {payload}")
        return payload

    def connect(self) -> None:
        payload = self.request("GET", "/v1.0/token", {"grant_type": 1})
        token = payload.get("result", {}).get("access_token")
        if not token:
            raise RuntimeError(f"Tuya token response did not contain an access_token: {payload}")
        self.access_token = token

    def status(self) -> dict[str, Any]:
        self.connect()
        payload = self.request("GET", f"/v1.0/iot-03/devices/{self.device_id}/status", raise_on_error=False)
        if payload.get("success") is not True:
            payload = self.request("GET", f"/v1.0/devices/{self.device_id}/status")
        rows = payload.get("result") or []
        return {row.get("code"): row.get("value") for row in rows if isinstance(row, dict)}

    def properties(self) -> dict[str, dict[str, Any]]:
        self.connect()
        payload = self.request("GET", f"/v2.0/cloud/thing/{self.device_id}/shadow/properties")
        rows = payload.get("result", {}).get("properties", [])
        return {row.get("code"): row for row in rows if isinstance(row, dict) and row.get("code")}

    def send(self, commands: list[dict[str, Any]]) -> Any:
        self.connect()
        body = {"commands": commands}
        payload = self.request(
            "POST",
            f"/v1.0/iot-03/devices/{self.device_id}/commands",
            body=body,
            raise_on_error=False,
        )
        if payload.get("success") is True:
            return payload
        legacy = self.request("POST", f"/v1.0/devices/{self.device_id}/commands", body=body, raise_on_error=False)
        if legacy.get("success") is not True:
            raise RuntimeError(f"Tuya command failed: iot-03={payload}; legacy={legacy}")
        return legacy


class TuyaLan:
    def __init__(self, config: Config) -> None:
        if not config.local_key:
            raise RuntimeError(
                "No local key configured. Set ORRO_LOCAL_KEY or add 'Local Key' "
                "to the 1Password item."
            )
        self.config = config
        self.ip = config.lan_ip or self._discover_ip()
        if not self.ip:
            raise RuntimeError("Could not discover LAN IP for device")
        self.device = self._connect()

    def _discover_ip(self) -> str | None:
        with open(os.devnull, "w") as sink, contextlib.redirect_stdout(sink), contextlib.redirect_stderr(sink):
            found = tinytuya.find_device(dev_id=self.config.device_id)
        ip = found.get("ip") if isinstance(found, dict) else None
        version = found.get("version") if isinstance(found, dict) else None
        if version and not self.config.lan_version:
            self.config.lan_version = str(version)
        return ip

    def _candidate_versions(self) -> list[float]:
        versions: list[float] = []
        if self.config.lan_version:
            with contextlib.suppress(ValueError):
                versions.append(float(self.config.lan_version))
        for version in (3.5, 3.4, 3.3, 3.1):
            if version not in versions:
                versions.append(version)
        return versions

    def _connect(self) -> tinytuya.Device:
        last_error: Exception | None = None
        for version in self._candidate_versions():
            device = tinytuya.Device(
                self.config.device_id,
                self.ip,
                self.config.local_key or "",
                version=version,
                connection_timeout=3,
            )
            with open(os.devnull, "w") as sink, contextlib.redirect_stdout(sink), contextlib.redirect_stderr(sink):
                try:
                    status = device.status()
                except Exception as exc:
                    last_error = exc
                    continue
            if isinstance(status, dict) and "Error" not in status:
                self.config.lan_version = str(version)
                return device
            last_error = RuntimeError(str(status))
        raise RuntimeError(f"LAN connection failed for {self.ip}: {last_error}")

    def status(self) -> dict[str, Any]:
        payload = self.device.status()
        if not isinstance(payload, dict) or "Error" in payload:
            raise RuntimeError(f"LAN status failed: {payload}")
        return payload

    def _dp(self, code: str) -> int:
        try:
            return self.config.lan_dps[code]
        except KeyError:
            raise RuntimeError(f"No LAN DP mapping configured for {code}") from None

    def _set(self, code: str, value: Any) -> Any:
        return self.device.set_status(value, self._dp(code))

    def send_codes(self, commands: list[dict[str, Any]]) -> list[Any]:
        results = []
        for command in commands:
            results.append(self._set(command["code"], command["value"]))
        return results


def movement_commands(action: str, config: Config, target: str | None = None) -> list[dict[str, Any]]:
    wake = [{"code": "child_lock", "value": False}]
    if action == "up":
        return wake + [{"code": "move_down", "value": False}, {"code": "move_up", "value": True}]
    if action == "down":
        return wake + [{"code": "move_up", "value": False}, {"code": "move_down", "value": True}]
    if action == "stop":
        return [{"code": "move_up", "value": False}, {"code": "move_down", "value": False}]
    if action in {"sit", "stand"}:
        preset = target or config.presets[action]
        return wake + [{"code": "memory_location", "value": preset}]
    raise ValueError(action)


def cloud_send(config: Config, commands: list[dict[str, Any]]) -> Any:
    return TuyaCloud(config).send(commands)


def lan_then_cloud(config: Config, commands: list[dict[str, Any]], force_cloud: bool) -> dict[str, Any]:
    if not force_cloud:
        try:
            result = TuyaLan(config).send_codes(commands)
            return {"path": "lan", "result": result}
        except Exception as exc:
            lan_error = str(exc)
        result = cloud_send(config, commands)
        return {"path": "cloud", "lan_error": lan_error, "result": result}
    return {"path": "cloud", "result": cloud_send(config, commands)}


def get_status(config: Config, force_cloud: bool) -> dict[str, Any]:
    if not force_cloud:
        try:
            lan = TuyaLan(config)
            return {"path": "lan", "ip": lan.ip, "version": config.lan_version, "status": lan.status()}
        except Exception as exc:
            cloud_status = TuyaCloud(config).status()
            return {"path": "cloud", "lan_error": str(exc), "status": cloud_status}
    return {"path": "cloud", "status": TuyaCloud(config).status()}


def current_height_mm(status: dict[str, Any], config: Config, path: str) -> int | None:
    if path == "lan":
        dps = status.get("dps", status)
        value = dps.get(str(config.lan_dps["height_display"])) or dps.get(config.lan_dps["height_display"])
    else:
        value = status.get("height_display")
    if value is None:
        return None
    with contextlib.suppress(TypeError, ValueError):
        return int(value)
    return None


def decode_memory_heights(raw_value: str | None) -> dict[str, int]:
    if not raw_value:
        return {}
    raw = base64.b64decode(raw_value)
    heights: dict[str, int] = {}
    for index in range(0, min(len(raw), 8), 2):
        heights[f"mem{index // 2 + 1}"] = int.from_bytes(raw[index:index + 2], "big")
    return heights


def get_presets(config: Config) -> dict[str, Any]:
    properties = TuyaCloud(config).properties()
    memory_heights = decode_memory_heights(properties.get("memory_height", {}).get("value"))
    current_height = properties.get("height_display", {}).get("value")
    memory_location = properties.get("memory_location", {}).get("value")
    return {
        "configured": config.presets,
        "device": memory_heights,
        "device_cm": {key: value / 10 for key, value in memory_heights.items()},
        "current_height_mm": current_height,
        "current_height_cm": current_height / 10 if isinstance(current_height, int) else None,
        "memory_location": memory_location,
        "raw": {
            "memory_height": properties.get("memory_height", {}).get("value"),
            "memory_height_dp": properties.get("memory_height", {}).get("dp_id"),
            "memory_location_dp": properties.get("memory_location", {}).get("dp_id"),
        },
    }


def go_to_height(config: Config, cm: float, force_cloud: bool) -> dict[str, Any]:
    target_mm = int(round(cm * 10))
    status_payload = get_status(config, force_cloud)
    path = status_payload["path"]
    status = status_payload["status"]
    current = current_height_mm(status, config, path)
    if current is None:
        raise RuntimeError("height_display is not available; use presets or up/down/stop")
    tolerance_mm = 5
    if abs(current - target_mm) <= tolerance_mm:
        return {"path": path, "target_mm": target_mm, "height_mm": current, "result": "already_at_target"}
    direction = "up" if target_mm > current else "down"
    commands = movement_commands(direction, config)
    lan_then_cloud(config, commands, force_cloud or path == "cloud")
    started = time.time()
    last = current
    try:
        while time.time() - started < 30:
            time.sleep(0.5)
            status_payload = get_status(config, force_cloud or path == "cloud")
            last = current_height_mm(status_payload["status"], config, status_payload["path"]) or last
            if abs(last - target_mm) <= tolerance_mm:
                break
            if direction == "up" and last > target_mm:
                break
            if direction == "down" and last < target_mm:
                break
    finally:
        lan_then_cloud(config, movement_commands("stop", config), force_cloud or path == "cloud")
    return {"path": path, "target_mm": target_mm, "height_mm": last, "result": "stopped"}


def print_result(payload: Any, as_json: bool) -> None:
    if as_json:
        print(json.dumps(payload, indent=2, sort_keys=True))
    else:
        if isinstance(payload, dict):
            for key, value in payload.items():
                print(f"{key}: {value}")
        else:
            print(payload)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Control the Orro Tuya standing desk.")
    parser.add_argument("--cloud", action="store_true", help="force Tuya Cloud API instead of LAN first")
    parser.add_argument("--json", action="store_true", help="print JSON output")
    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--cloud", action="store_true", default=argparse.SUPPRESS, help=argparse.SUPPRESS)
    common.add_argument("--json", action="store_true", default=argparse.SUPPRESS, help=argparse.SUPPRESS)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("status", "up", "down", "stand", "sit", "stop", "presets"):
        sub.add_parser(name, parents=[common])
    height = sub.add_parser("height", parents=[common])
    height.add_argument("cm", type=float)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    config = Config.load()

    if args.command == "status":
        print_result(get_status(config, args.cloud), args.json)
        return 0
    if args.command == "presets":
        print_result(get_presets(config), args.json)
        return 0
    if args.command == "height":
        print_result(go_to_height(config, args.cm, args.cloud), args.json)
        return 0

    commands = movement_commands(args.command, config)
    print_result(lan_then_cloud(config, commands, args.cloud), args.json)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
    except Exception as exc:
        print(f"orro: {exc}", file=sys.stderr)
        raise SystemExit(1)
