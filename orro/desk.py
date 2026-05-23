"""High-level desk operations: status, movement, presets, height targeting."""

from __future__ import annotations

import base64
import contextlib
import time
from typing import Any

from orro.cloud import TuyaCloud
from orro.config import Config
from orro.lan import TuyaLan


def movement_commands(
    action: str, config: Config, target: str | None = None
) -> list[dict[str, Any]]:
    """Build the DP command list for a given movement action."""
    wake = [{"code": "child_lock", "value": False}]
    if action == "up":
        return wake + [
            {"code": "move_down", "value": False},
            {"code": "move_up", "value": True},
        ]
    if action == "down":
        return wake + [
            {"code": "move_up", "value": False},
            {"code": "move_down", "value": True},
        ]
    if action == "stop":
        return [{"code": "move_up", "value": False}, {"code": "move_down", "value": False}]
    if action in {"sit", "stand", "goto"}:
        preset = target or config.presets.get(action, "mem1")
        return wake + [{"code": "memory_location", "value": preset}]
    raise ValueError(f"Unknown action: {action}")


def cloud_send(config: Config, commands: list[dict[str, Any]]) -> Any:
    """Send commands via the Cloud API."""
    return TuyaCloud(config).send(commands)


def lan_then_cloud(
    config: Config, commands: list[dict[str, Any]], force_cloud: bool
) -> dict[str, Any]:
    """Try LAN first, fall back to Cloud.  Returns ``{path, result, ...}``."""
    if not force_cloud:
        try:
            result = TuyaLan(config).send_codes(commands)
            return {"path": "lan", "result": result}
        except Exception as exc:
            lan_error = str(exc)
        config.debug(f"lan failed ({lan_error}), falling back to cloud")
        result = cloud_send(config, commands)
        return {"path": "cloud", "lan_error": lan_error, "result": result}
    return {"path": "cloud", "result": cloud_send(config, commands)}


def get_status(config: Config, force_cloud: bool) -> dict[str, Any]:
    """Fetch desk status, preferring LAN when available."""
    if not force_cloud:
        try:
            lan = TuyaLan(config)
            return {
                "path": "lan",
                "ip": lan.ip,
                "version": config.lan_version,
                "status": lan.status(),
            }
        except Exception as exc:
            config.debug(f"lan status failed ({exc}), falling back to cloud")
            cloud_status = TuyaCloud(config).status()
            return {"path": "cloud", "lan_error": str(exc), "status": cloud_status}
    return {"path": "cloud", "status": TuyaCloud(config).status()}


def current_height_mm(status: dict[str, Any], config: Config, path: str) -> int | None:
    """Extract the current height in mm from a status dict."""
    if path == "lan":
        dps = status.get("dps", status)
        value = dps.get(str(config.lan_dps["height_display"])) or dps.get(
            config.lan_dps["height_display"]
        )
    else:
        value = status.get("height_display")
    if value is None:
        return None
    with contextlib.suppress(TypeError, ValueError):
        return int(value)
    return None


def decode_memory_heights(raw_value: str | None) -> dict[str, int]:
    """Decode base64-encoded memory slot heights into ``{memN: mm}``."""
    if not raw_value:
        return {}
    raw = base64.b64decode(raw_value)
    heights: dict[str, int] = {}
    for index in range(0, min(len(raw), 8), 2):
        heights[f"mem{index // 2 + 1}"] = int.from_bytes(raw[index : index + 2], "big")
    return heights


def get_presets(config: Config) -> dict[str, Any]:
    """Fetch preset/memory data from the Cloud shadow properties."""
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
    """Move the desk to a specific height in cm, polling until target is reached."""
    target_mm = int(round(cm * 10))
    status_payload = get_status(config, force_cloud)
    path = status_payload["path"]
    status = status_payload["status"]
    current = current_height_mm(status, config, path)
    if current is None:
        raise RuntimeError("height_display is not available; use presets or up/down/stop")
    tolerance_mm = 5
    if abs(current - target_mm) <= tolerance_mm:
        return {
            "path": path,
            "target_mm": target_mm,
            "height_mm": current,
            "result": "already_at_target",
        }
    direction = "up" if target_mm > current else "down"
    commands = movement_commands(direction, config)
    use_cloud = force_cloud or path == "cloud"
    lan_then_cloud(config, commands, use_cloud)
    started = time.time()
    last = current
    try:
        while time.time() - started < 30:
            time.sleep(0.5)
            status_payload = get_status(config, use_cloud)
            last = (
                current_height_mm(status_payload["status"], config, status_payload["path"]) or last
            )
            if abs(last - target_mm) <= tolerance_mm:
                break
            if direction == "up" and last > target_mm:
                break
            if direction == "down" and last < target_mm:
                break
    finally:
        lan_then_cloud(config, movement_commands("stop", config), use_cloud)
    return {"path": path, "target_mm": target_mm, "height_mm": last, "result": "stopped"}
