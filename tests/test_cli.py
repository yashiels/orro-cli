"""Basic CLI tests."""

import subprocess
import sys


def test_version():
    """--version prints version string."""
    result = subprocess.run(
        [sys.executable, "-m", "orro", "--version"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0
    assert "orro" in result.stdout


def test_help():
    """--help exits cleanly."""
    result = subprocess.run(
        [sys.executable, "-m", "orro", "--help"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0
    assert "standing desk" in result.stdout.lower()


def test_config_path():
    """config path prints a path."""
    result = subprocess.run(
        [sys.executable, "-m", "orro", "config", "path"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0
    assert "config.yaml" in result.stdout
