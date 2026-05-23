"""Basic tests for orro configuration."""

from orro.config import DEFAULT_LAN_DPS, DEFAULT_PRESETS, Config, default_config_path


def test_default_config():
    """Config dataclass has sensible defaults."""
    cfg = Config()
    assert cfg.endpoint == ""
    assert cfg.device_id == ""
    assert cfg.lan_dps == DEFAULT_LAN_DPS
    assert cfg.presets == DEFAULT_PRESETS


def test_default_config_path():
    """Config path resolves to XDG location."""
    path = default_config_path()
    assert path.name == "config.yaml"
    assert "orro" in str(path)


def test_config_to_dict_redact():
    """Secrets are redacted in to_dict output."""
    cfg = Config(
        endpoint="https://example.com",
        access_id="test_id",
        access_secret="super_secret",
        device_id="dev123",
        local_key="key123",
    )
    d = cfg.to_dict(redact=True)
    assert d["access_secret"] == "***"
    assert d["local_key"] == "***"
    assert d["access_id"] == "test_id"
    assert d["endpoint"] == "https://example.com"


def test_config_to_dict_no_redact():
    """Secrets are visible when redact=False."""
    cfg = Config(
        endpoint="https://example.com",
        access_id="test_id",
        access_secret="super_secret",
        device_id="dev123",
        local_key="key123",
    )
    d = cfg.to_dict(redact=False)
    assert d["access_secret"] == "super_secret"
    assert d["local_key"] == "key123"
