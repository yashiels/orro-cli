"""Allow running as ``python -m orro``."""

import sys

from orro.cli import main

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130) from None
    except Exception as exc:
        print(f"orro: {exc}", file=sys.stderr)
        raise SystemExit(1) from None
