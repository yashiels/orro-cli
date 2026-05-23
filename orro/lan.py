"""Tuya LAN (local network) device control via TinyTuya."""

from __future__ import annotations

import contextlib
import os
from typing import Any

import tinytuya

from orro.config import Config


class TuyaLan:
    """Direct LAN connection to a Tuya device using TinyTuya."""

    def __init__(self, config: Config) -> None:
        if not config.local_key:
            raise RuntimeError(
                "No local key configured. Set ORRO_LOCAL_KEY, add 'local_key' "
                "to the config file, or add 'Local Key' to the 1Password item."
            )
        self.config = config
        self.ip = config.lan_ip or self._discover_ip()
        if not self.ip:
            raise RuntimeError("Could not discover LAN IP for device")
        self.device = self._connect()

    def _discover_ip(self) -> str | None:
        self.config.debug("lan: scanning for device on local network")
        with (
            open(os.devnull, "w") as sink,
            contextlib.redirect_stdout(sink),
            contextlib.redirect_stderr(sink),
        ):
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
        self.config.debug(f"lan: connecting to {self.ip}")
        last_error: Exception | None = None
        for version in self._candidate_versions():
            self.config.debug(f"lan: trying protocol v{version}")
            device = tinytuya.Device(
                self.config.device_id,
                self.ip,
                self.config.local_key or "",
                version=version,
                connection_timeout=3,
            )
            with (
                open(os.devnull, "w") as sink,
                contextlib.redirect_stdout(sink),
                contextlib.redirect_stderr(sink),
            ):
                try:
                    status = device.status()
                except Exception as exc:
                    last_error = exc
                    continue
            if isinstance(status, dict) and "Error" not in status:
                self.config.lan_version = str(version)
                self.config.debug(f"lan: connected with v{version}")
                return device
            last_error = RuntimeError(str(status))
        raise RuntimeError(f"LAN connection failed for {self.ip}: {last_error}")

    def status(self) -> dict[str, Any]:
        """Return the current device status dict from the LAN."""
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
        """Send a list of ``{code, value}`` commands to the device over LAN."""
        results = []
        for command in commands:
            results.append(self._set(command["code"], command["value"]))
        return results
