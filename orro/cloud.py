"""Tuya Cloud API client."""

from __future__ import annotations

import hashlib
import hmac
import json
import time
from typing import Any
from urllib.parse import urlencode

import requests

from orro.config import Config


def canonical_query(params: dict[str, Any] | None) -> str:
    """Sort and encode query parameters for Tuya API signing."""
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


class TuyaCloud:
    """Tuya Cloud HTTP API client with HMAC-SHA256 request signing."""

    def __init__(self, config: Config) -> None:
        self.endpoint = config.endpoint.rstrip("/")
        self.access_id = config.access_id
        self.access_secret = config.access_secret.encode()
        self.device_id = config.device_id
        self._config = config
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
        signature = (
            hmac.new(self.access_secret, sign_source.encode(), hashlib.sha256).hexdigest().upper()
        )
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
        self._config.debug(f"cloud {method} {path}")
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
            raise RuntimeError(
                f"Tuya API {method} {path} failed: HTTP {response.status_code}: {payload}"
            )
        return payload

    def connect(self) -> None:
        """Obtain an access token from the Tuya token endpoint."""
        self._config.debug("cloud: obtaining access token")
        payload = self.request("GET", "/v1.0/token", {"grant_type": 1})
        token = payload.get("result", {}).get("access_token")
        if not token:
            raise RuntimeError(f"Tuya token response did not contain an access_token: {payload}")
        self.access_token = token

    def status(self) -> dict[str, Any]:
        """Fetch current device status DPs via the Cloud API."""
        self.connect()
        payload = self.request(
            "GET",
            f"/v1.0/iot-03/devices/{self.device_id}/status",
            raise_on_error=False,
        )
        if payload.get("success") is not True:
            payload = self.request("GET", f"/v1.0/devices/{self.device_id}/status")
        rows = payload.get("result") or []
        return {row.get("code"): row.get("value") for row in rows if isinstance(row, dict)}

    def properties(self) -> dict[str, dict[str, Any]]:
        """Fetch shadow properties (includes memory_height and preset data)."""
        self.connect()
        payload = self.request("GET", f"/v2.0/cloud/thing/{self.device_id}/shadow/properties")
        rows = payload.get("result", {}).get("properties", [])
        return {row.get("code"): row for row in rows if isinstance(row, dict) and row.get("code")}

    def send(self, commands: list[dict[str, Any]]) -> Any:
        """Send commands to the device, falling back to the legacy endpoint."""
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
        legacy = self.request(
            "POST",
            f"/v1.0/devices/{self.device_id}/commands",
            body=body,
            raise_on_error=False,
        )
        if legacy.get("success") is not True:
            raise RuntimeError(f"Tuya command failed: iot-03={payload}; legacy={legacy}")
        return legacy
