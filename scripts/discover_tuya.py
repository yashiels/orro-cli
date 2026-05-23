#!/usr/bin/env python3
"""Tuya Cloud DP discovery script.

Queries multiple Tuya API endpoints for a device and writes the full
response set, normalised DP map, and shadow properties to
``dp-discovery.json`` in the current directory.

Credentials are read from 1Password (vault ``OpenClaw``, item
``Tuya IoT Platform``).  Requires the ``op`` CLI and the ``requests``
package.
"""

import hashlib
import hmac
import json
import subprocess
import sys
import time
from datetime import datetime, timezone
from typing import Any
from urllib.parse import urlencode

import requests


VAULT = "OpenClaw"
ITEM = "Tuya IoT Platform"


def op_read(field: str) -> str:
    ref = f"op://{VAULT}/{ITEM}/{field}"
    result = subprocess.run(
        ["op", "read", ref],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return result.stdout.rstrip("\n")


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


class TuyaCloud:
    def __init__(self, endpoint: str, access_id: str, access_secret: str) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.access_id = access_id
        self.access_secret = access_secret.encode()
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
        headers = self._sign(method, path, params, body)
        response = requests.request(
            method.upper(),
            f"{self.endpoint}{path}",
            params=params,
            data=self._body_text(body) if body is not None else None,
            headers=headers,
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


def result_list(payload: Any) -> list[dict[str, Any]]:
    result = payload.get("result") if isinstance(payload, dict) else None
    if isinstance(result, list):
        return [item for item in result if isinstance(item, dict)]
    if isinstance(result, dict):
        for key in ("status", "functions"):
            value = result.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
    return []


def build_dp_map(status: Any, specification: Any, functions: Any, shadow_properties: Any) -> dict[str, Any]:
    spec_result = specification.get("result", {}) if isinstance(specification, dict) else {}
    status_rows = result_list(status)
    function_rows = result_list(functions)
    spec_status_rows = spec_result.get("status", []) if isinstance(spec_result, dict) else []
    spec_function_rows = spec_result.get("functions", []) if isinstance(spec_result, dict) else []

    by_code: dict[str, dict[str, Any]] = {}
    for row in status_rows:
        code = str(row.get("code", ""))
        if code:
            by_code.setdefault(code, {})["current"] = row.get("value")
    for row in spec_status_rows:
        code = str(row.get("code", ""))
        if code:
            by_code.setdefault(code, {})["status_spec"] = row
    for row in spec_function_rows:
        code = str(row.get("code", ""))
        if code:
            by_code.setdefault(code, {})["function_spec"] = row
    for row in function_rows:
        code = str(row.get("code", ""))
        if code:
            by_code.setdefault(code, {})["function"] = row
    for row in shadow_properties.get("result", {}).get("properties", []):
        if not isinstance(row, dict):
            continue
        code = str(row.get("code", ""))
        if code:
            by_code.setdefault(code, {})["shadow_property"] = row
            by_code.setdefault(code, {})["dp_id"] = row.get("dp_id")
            by_code.setdefault(code, {})["current"] = row.get("value")
    return by_code


def main() -> int:
    access_id = op_read("Access ID")
    access_secret = op_read("Access Secret")
    endpoint = op_read("API Endpoint")
    device_id = op_read("Device ID")

    cloud = TuyaCloud(endpoint, access_id, access_secret)
    cloud.connect()

    paths = {
        "device": f"/v1.0/devices/{device_id}",
        "status": f"/v1.0/devices/{device_id}/status",
        "specification_requested": f"/v1.0/devices/{device_id}/specification",
        "specification": f"/v1.0/devices/{device_id}/specifications",
        "iot03_specification": f"/v1.0/iot-03/devices/{device_id}/specification",
        "functions": f"/v1.0/devices/{device_id}/functions",
        "shadow_properties": f"/v2.0/cloud/thing/{device_id}/shadow/properties",
    }
    responses: dict[str, Any] = {}
    for name, path in paths.items():
        responses[name] = cloud.request(
            "GET",
            path,
            raise_on_error=name not in {"specification_requested", "iot03_specification"},
        )
    specification = responses["specification"]
    if responses["iot03_specification"].get("success") is True:
        specification = responses["iot03_specification"]
    output = {
        "collected_at": datetime.now(timezone.utc).isoformat(),
        "endpoint": endpoint,
        "device_id": device_id,
        "paths": paths,
        "dp_map": build_dp_map(
            responses["status"],
            specification,
            responses["functions"],
            responses["shadow_properties"],
        ),
        "shadow_dp_map": {
            item["code"]: {
                "dp_id": item.get("dp_id"),
                "type": item.get("type"),
                "value": item.get("value"),
                "time": item.get("time"),
            }
            for item in responses["shadow_properties"].get("result", {}).get("properties", [])
            if isinstance(item, dict) and item.get("code")
        },
        "responses": responses,
    }
    with open("dp-discovery.json", "w", encoding="utf-8") as handle:
        json.dump(output, handle, indent=2, sort_keys=True)
        handle.write("\n")
    local_key = responses["device"].get("result", {}).get("local_key")
    print(json.dumps({
        "saved": "dp-discovery.json",
        "dp_count": len(output["dp_map"]),
        "has_local_key": bool(local_key),
        "local_key_len": len(local_key or ""),
    }, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
