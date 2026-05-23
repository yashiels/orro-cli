"""Allow running as ``python -m orro``."""

from orro.cli import main

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
    except Exception as exc:
        import sys

        print(f"orro: {exc}", file=sys.stderr)
        raise SystemExit(1)
