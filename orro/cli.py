"""Argument parsing and command dispatch for the orro CLI."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any

from orro import __version__
from orro.config import Config, default_config_path, write_config_file
from orro.desk import (
    get_presets,
    get_status,
    go_to_height,
    lan_then_cloud,
    movement_commands,
)
from orro.output import print_result

# ---------------------------------------------------------------------------
# Parser
# ---------------------------------------------------------------------------


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="orro",
        description="Control your Tuya-based standing desk from the terminal.",
    )
    parser.add_argument("--version", action="version", version=f"orro {__version__}")
    parser.add_argument(
        "--cloud",
        action="store_true",
        help="force Tuya Cloud API instead of LAN-first",
    )
    parser.add_argument(
        "--output",
        choices=["table", "json"],
        default="table",
        help="output format (default: table)",
    )
    parser.add_argument(
        "--json",
        action="store_const",
        const="json",
        dest="output",
        help=argparse.SUPPRESS,  # hidden alias for --output json
    )
    parser.add_argument("--verbose", "-v", action="store_true", help="print debug information")
    parser.add_argument("--quiet", "-q", action="store_true", help="suppress info banners")
    parser.add_argument(
        "--config",
        metavar="PATH",
        help="path to config file (default: ~/.config/orro/config.yaml)",
    )

    sub = parser.add_subparsers(dest="command")

    # Movement commands
    sub.add_parser("status", help="Show desk status (height, connection, state)")
    sub.add_parser("up", help="Move desk up (hold until stop)")
    sub.add_parser("down", help="Move desk down (hold until stop)")
    sub.add_parser("stop", help="Stop desk movement")
    sub.add_parser("sit", help="Move to sit preset")
    sub.add_parser("stand", help="Move to stand preset")
    sub.add_parser("presets", help="Show preset configuration and device memory slots")

    # goto
    goto = sub.add_parser("goto", help="Move to a specific memory preset (mem1-mem4)")
    goto.add_argument(
        "preset",
        choices=["mem1", "mem2", "mem3", "mem4"],
        help="memory slot name",
    )

    # height
    height = sub.add_parser("height", help="Move to a specific height in cm")
    height.add_argument("cm", type=float, help="target height in centimetres")

    # config
    config_parser = sub.add_parser("config", help="Manage configuration")
    config_sub = config_parser.add_subparsers(dest="config_action")

    config_sub.add_parser("show", help="Print current config (secrets redacted)")
    config_sub.add_parser("path", help="Print config file path")

    config_set = config_sub.add_parser("set", help="Set config values")
    config_set.add_argument("--endpoint", help="Tuya API endpoint URL")
    config_set.add_argument("--access-id", help="Tuya Access ID")
    config_set.add_argument("--access-secret", help="Tuya Access Secret")
    config_set.add_argument("--device-id", help="Tuya device ID")
    config_set.add_argument("--local-key", help="device local encryption key")
    config_set.add_argument("--lan-ip", help="device LAN IP address")
    config_set.add_argument("--lan-version", help="Tuya LAN protocol version")

    config_sub.add_parser("init", help="Interactive config setup (prints template if not a TTY)")

    return parser


# ---------------------------------------------------------------------------
# Config subcommand handlers
# ---------------------------------------------------------------------------

_CONFIG_TEMPLATE = """\
# orro config — ~/.config/orro/config.yaml
# Uncomment and fill in your values.

endpoint: https://openapi.tuyaeu.com
access_id: your_access_id
access_secret: your_access_secret
device_id: your_device_id
# local_key: your_local_key
# lan_ip: 192.168.10.95
# lan_version: "3.4"

# presets:
#   sit: mem1
#   stand: mem3
"""


def _cmd_config(args: argparse.Namespace) -> int:
    config_path = Path(args.config) if args.config else None

    if args.config_action == "path":
        print(config_path or default_config_path())
        return 0

    if args.config_action == "show":
        cfg = Config.load(
            config_path=config_path,
            verbose=args.verbose,
            quiet=args.quiet,
            require_credentials=False,
        )
        print_result(cfg.to_dict(redact=True), args.output)
        return 0

    if args.config_action == "set":
        values: dict[str, Any] = {}
        field_map = {
            "endpoint": "endpoint",
            "access_id": "access_id",
            "access_secret": "access_secret",
            "device_id": "device_id",
            "local_key": "local_key",
            "lan_ip": "lan_ip",
            "lan_version": "lan_version",
        }
        for arg_name, cfg_key in field_map.items():
            val = getattr(args, arg_name, None)
            if val is not None:
                values[cfg_key] = val
        if not values:
            print("orro: no values specified for config set", file=sys.stderr)
            return 1
        path = write_config_file(values, config_path)
        print(f"wrote {path}")
        return 0

    if args.config_action == "init":
        if sys.stdin.isatty():
            return _interactive_config_init(config_path)
        print(_CONFIG_TEMPLATE)
        return 0

    # No subcommand — show help.
    build_parser().parse_args(["config", "--help"])
    return 1


def _interactive_config_init(config_path: Path | None) -> int:
    """Walk the user through initial config creation."""
    print("orro config init — interactive setup\n")
    values: dict[str, str] = {}

    prompts = [
        ("endpoint", "Tuya API endpoint", "https://openapi.tuyaeu.com"),
        ("access_id", "Access ID", ""),
        ("access_secret", "Access Secret", ""),
        ("device_id", "Device ID", ""),
        ("local_key", "Local Key (optional, press Enter to skip)", ""),
        ("lan_ip", "LAN IP (optional, press Enter to skip)", ""),
        ("lan_version", "LAN protocol version (optional)", "3.4"),
    ]

    for key, label, default in prompts:
        suffix = f" [{default}]" if default else ""
        answer = input(f"  {label}{suffix}: ").strip()
        if answer:
            values[key] = answer
        elif default:
            values[key] = default

    # Remove empty optional values.
    values = {k: v for k, v in values.items() if v}

    path = write_config_file(values, config_path)
    print(f"\nConfig written to {path}")
    return 0


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = build_parser()
    args = parser.parse_args(argv)

    if not args.command:
        parser.print_help()
        return 1

    # Config subcommand does not need full Config.load() for all actions.
    if args.command == "config":
        return _cmd_config(args)

    config_path = Path(args.config) if args.config else None
    config = Config.load(
        config_path=config_path,
        verbose=args.verbose,
        quiet=args.quiet,
    )

    if args.command == "status":
        print_result(get_status(config, args.cloud), args.output)
        return 0

    if args.command == "presets":
        print_result(get_presets(config), args.output)
        return 0

    if args.command == "height":
        print_result(go_to_height(config, args.cm, args.cloud), args.output)
        return 0

    if args.command == "goto":
        commands = movement_commands("goto", config, target=args.preset)
        print_result(lan_then_cloud(config, commands, args.cloud), args.output)
        return 0

    # Simple movement commands: up, down, stop, sit, stand
    commands = movement_commands(args.command, config)
    print_result(lan_then_cloud(config, commands, args.cloud), args.output)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130) from None
    except Exception as exc:
        print(f"orro: {exc}", file=sys.stderr)
        raise SystemExit(1) from None
