"""Configuration loading: YAML file → env vars → 1Password → defaults."""

from __future__ import annotations

import json
import os
import shutil
import stat
import subprocess
import sys
from dataclasses import dataclass, field, fields
from pathlib import Path
from typing import Any

import yaml

VAULT = "OpenClaw"
TUYA_ITEM = "Tuya IoT Platform"

DEFAULT_PRESETS = {"sit": "mem1", "stand": "mem3"}

# Verified LAN status DPs for this desk.  Movement/preset command DPs are not
# exposed in the passive status response, so they are intentionally not guessed.
# Override with 1Password field "LAN DP Map" or ORRO_LAN_DP_MAP after verifying.
DEFAULT_LAN_DPS = {
    "child_lock": 6,
    "move_up": 150,
    "move_down": 151,
    "height_display": 152,
    "memory_location": 155,
}

# Env-var → Config-field mapping.
_ENV_MAP = {
    "ORRO_ENDPOINT": "endpoint",
    "ORRO_ACCESS_ID": "access_id",
    "ORRO_ACCESS_SECRET": "access_secret",
    "ORRO_DEVICE_ID": "device_id",
    "ORRO_LOCAL_KEY": "local_key",
    "ORRO_LAN_IP": "lan_ip",
    "ORRO_LAN_VERSION": "lan_version",
}

# 1Password field → Config-field mapping.
_OP_MAP = {
    "API Endpoint": "endpoint",
    "Access ID": "access_id",
    "Access Secret": "access_secret",
    "Device ID": "device_id",
    "Local Key": "local_key",
    "LAN IP": "lan_ip",
    "LAN Version": "lan_version",
}

# Fields that should be redacted in ``config show``.
SECRET_FIELDS = {"access_secret", "local_key"}


# ---------------------------------------------------------------------------
# 1Password helpers
# ---------------------------------------------------------------------------


def _op_available() -> bool:
    """Return True if the 1Password CLI (``op``) is on PATH."""
    return shutil.which("op") is not None


def op_read(item: str, op_field: str, required: bool = True) -> str | None:
    """Read a field from the 1Password vault.

    Returns ``None`` instead of raising when *required* is ``False`` and the
    field cannot be read (or ``op`` is not installed).
    """
    if not _op_available():
        if required:
            raise RuntimeError(
                f"1Password CLI (op) not found and no fallback for {item}/{op_field}"
            )
        return None
    ref = f"op://{VAULT}/{item}/{op_field}"
    result = subprocess.run(
        ["op", "read", ref],
        text=True,
        capture_output=True,
    )
    if result.returncode == 0:
        return result.stdout.rstrip("\n")
    if required:
        raise RuntimeError(f"Could not read {ref}: {result.stderr.strip()}")
    return None


def _env_or_op(env_key: str, item: str, op_field: str, required: bool = True) -> str | None:
    """Try environment variable first, then 1Password, then error or None."""
    value = os.environ.get(env_key)
    if value:
        return value
    return op_read(item, op_field, required=required)


# ---------------------------------------------------------------------------
# Config file helpers
# ---------------------------------------------------------------------------


def default_config_path() -> Path:
    """Return the default YAML config path (``~/.config/orro/config.yaml``)."""
    xdg = os.environ.get("XDG_CONFIG_HOME")
    base = Path(xdg) if xdg else Path.home() / ".config"
    return base / "orro" / "config.yaml"


def _read_config_file(path: Path | None) -> dict[str, Any]:
    """Read and parse a YAML config file.  Returns ``{}`` on missing file."""
    if path is None:
        env_cfg = os.environ.get("ORRO_CONFIG")
        path = Path(env_cfg) if env_cfg else default_config_path()
    if not path.is_file():
        return {}
    with open(path, encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
    return data if isinstance(data, dict) else {}


def write_config_file(values: dict[str, Any], path: Path | None = None) -> Path:
    """Write *values* to the YAML config file.

    Merges with existing content so that unspecified keys are preserved.
    Ensures the file is created with ``0o600`` permissions.
    """
    path = path or default_config_path()
    existing = _read_config_file(path)
    existing.update(values)
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        yaml.safe_dump(existing, fh, default_flow_style=False, sort_keys=False)
    path.chmod(stat.S_IRUSR | stat.S_IWUSR)
    return path


# ---------------------------------------------------------------------------
# Config dataclass
# ---------------------------------------------------------------------------


@dataclass
class Config:
    """Resolved configuration for the orro CLI."""

    endpoint: str = ""
    access_id: str = ""
    access_secret: str = ""
    device_id: str = ""
    local_key: str | None = None
    lan_ip: str | None = None
    lan_version: str | None = None
    lan_dps: dict[str, int] = field(default_factory=lambda: dict(DEFAULT_LAN_DPS))
    presets: dict[str, str] = field(default_factory=lambda: dict(DEFAULT_PRESETS))

    # --- runtime flags (not persisted) ---
    verbose: bool = False
    quiet: bool = False

    def debug(self, msg: str) -> None:
        """Print *msg* to stderr when verbose mode is enabled."""
        if self.verbose:
            print(f"[debug] {msg}", file=sys.stderr)

    def info(self, msg: str) -> None:
        """Print *msg* to stderr unless quiet mode is enabled."""
        if not self.quiet:
            print(msg, file=sys.stderr)

    # ------------------------------------------------------------------
    # Loading
    # ------------------------------------------------------------------

    @classmethod
    def load(
        cls,
        *,
        config_path: Path | None = None,
        overrides: dict[str, Any] | None = None,
        verbose: bool = False,
        quiet: bool = False,
        require_credentials: bool = True,
    ) -> Config:
        """Build a Config by layering sources (highest-priority first):

        1. Explicit CLI flag overrides (``overrides`` dict)
        2. Environment variables (``ORRO_*``)
        3. YAML config file
        4. 1Password vault
        5. Built-in defaults
        """
        cfg = cls(verbose=verbose, quiet=quiet)

        # --- layer 5: defaults are already set in the dataclass ---

        # --- layer 4: 1Password ---
        op_values: dict[str, str | None] = {}
        for op_field, attr in _OP_MAP.items():
            required = attr in {"endpoint", "access_id", "access_secret", "device_id"}
            op_values[attr] = op_read(TUYA_ITEM, op_field, required=False)
        for attr, val in op_values.items():
            if val:
                setattr(cfg, attr, val)

        # 1Password JSON blobs
        raw_map = op_read(TUYA_ITEM, "LAN DP Map", required=False)
        if raw_map:
            cfg.lan_dps.update({k: int(v) for k, v in json.loads(raw_map).items()})
        raw_presets = op_read(TUYA_ITEM, "Presets", required=False)
        if raw_presets:
            cfg.presets.update({k: str(v) for k, v in json.loads(raw_presets).items()})

        # --- layer 3: config file ---
        file_values = _read_config_file(config_path)
        cfg.debug(f"config file: {file_values}")
        for key, val in file_values.items():
            if key == "presets" and isinstance(val, dict):
                cfg.presets.update({k: str(v) for k, v in val.items()})
            elif key == "lan_dps" and isinstance(val, dict):
                cfg.lan_dps.update({k: int(v) for k, v in val.items()})
            elif hasattr(cfg, key) and key not in {"verbose", "quiet", "lan_dps", "presets"}:
                setattr(cfg, key, val)

        # --- layer 2: environment variables ---
        for env_key, attr in _ENV_MAP.items():
            val = os.environ.get(env_key)
            if val:
                setattr(cfg, attr, val)
        raw_map = os.environ.get("ORRO_LAN_DP_MAP")
        if raw_map:
            cfg.lan_dps.update({k: int(v) for k, v in json.loads(raw_map).items()})
        raw_presets = os.environ.get("ORRO_PRESETS")
        if raw_presets:
            cfg.presets.update({k: str(v) for k, v in json.loads(raw_presets).items()})

        # --- layer 1: explicit overrides ---
        if overrides:
            for key, val in overrides.items():
                if val is not None and hasattr(cfg, key):
                    setattr(cfg, key, val)

        # Validate required fields.
        if require_credentials:
            required = {"endpoint", "access_id", "access_secret", "device_id"}
            missing = [name for name in sorted(required) if not getattr(cfg, name, None)]
            if missing:
                raise RuntimeError(
                    f"Missing required configuration: {', '.join(missing)}. "
                    "Set environment variables, create a config file, "
                    "or install 1Password CLI (op)."
                )

        return cfg

    # ------------------------------------------------------------------
    # Serialisation helpers
    # ------------------------------------------------------------------

    def to_dict(self, *, redact: bool = False) -> dict[str, Any]:
        """Return a plain dict of config values.

        When *redact* is ``True``, secret fields are replaced with ``***``.
        """
        out: dict[str, Any] = {}
        for f in fields(self):
            if f.name in {"verbose", "quiet"}:
                continue
            val = getattr(self, f.name)
            if redact and f.name in SECRET_FIELDS and val:
                val = "***"
            out[f.name] = val
        return out
