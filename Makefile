.PHONY: install lint fmt test check ci clean

install:
	pip install -e .

install-dev:
	pip install -e ".[dev]"

lint:
	ruff check orro/

fmt:
	ruff format orro/

test:
	python -m pytest tests/ -v

check: lint test

ci: lint test

clean:
	rm -rf build/ dist/ *.egg-info .ruff_cache/
