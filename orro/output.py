"""Output formatting: table (human) and JSON (machine)."""

from __future__ import annotations

import json
from typing import Any


def _format_table_value(value: Any) -> str:
    """Format a single value for table display."""
    if isinstance(value, dict):
        return ", ".join(f"{k}={v}" for k, v in value.items())
    if isinstance(value, list):
        return ", ".join(str(v) for v in value)
    if isinstance(value, bool):
        return "yes" if value else "no"
    return str(value)


def print_result(payload: Any, output_format: str = "table") -> None:
    """Print *payload* in the requested format.

    *output_format* should be ``"json"`` or ``"table"`` (default).
    """
    if output_format == "json":
        print(json.dumps(payload, indent=2, sort_keys=True))
    elif isinstance(payload, dict):
        # Find the longest key for alignment.
        if not payload:
            return
        width = max(len(str(k)) for k in payload) + 1
        for key, value in payload.items():
            print(f"{key + ':':<{width}} {_format_table_value(value)}")
    else:
        print(payload)
