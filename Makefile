# Self-documented Makefile — run `make` or `make help`.
# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

.DEFAULT_GOAL := help

# Parent otfabric/go.work must not pull this module in until it is listed there.
export GOWORK ?= off

GO         ?= go
BIN_DIR    := bin
FIXTURECTL := $(BIN_DIR)/fixturectl
SCHEMA     := fixtures/schema/fixture.schema.json
CANONICAL  := $(wildcard fixtures/canonical/*.json)
INVALID    := $(wildcard fixtures/invalid/*.json)

PROBE      := $(BIN_DIR)/interop-probe
DOCKER    ?= docker
NATIVE_IMAGE ?= snap-interop-native:local
PYTHON_IMAGE ?= snap-interop-python:local
PROBE_ADDR ?= 127.0.0.1:1102
PYTHON_DIR := servers/python

.PHONY: help fmt lint test ci bootstrap-check build fixturectl probe-bin \
	fixtures fixtures-check fixtures-update \
	native python images up down probe smoke-native smoke-python \
	python-test _smoke-run interop report sbom scan

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go and other available trees
	@echo "Running gofmt"
	@$(GO) fmt ./...
	@chmod +x scripts/bootstrap-check.sh
	@if command -v ruff >/dev/null 2>&1; then \
		echo "Running ruff format"; \
		ruff format $(PYTHON_DIR); \
	fi

lint: ## Lint bootstrap contracts, shell scripts and Go packages
	@echo "Running bootstrap-check"
	@./scripts/bootstrap-check.sh
	@if command -v shellcheck >/dev/null 2>&1; then \
		echo "Running shellcheck"; \
		shellcheck scripts/*.sh; \
	else \
		echo "shellcheck not installed — skipping"; \
	fi
	@echo "Running go vet"
	@$(GO) vet ./...

test: ## Run unit tests, fixture validation and golden checks
	@echo "Running go test"
	@$(GO) test ./...
	@$(MAKE) fixturectl
	@echo "Validating canonical fixtures"
	@$(FIXTURECTL) validate --schema $(SCHEMA) $(CANONICAL)
	@echo "Expecting invalid fixtures to fail"
	@failed=0; \
	for f in $(INVALID); do \
		if $(FIXTURECTL) validate --schema $(SCHEMA) "$$f" >/dev/null 2>&1; then \
			echo "FAIL: $$f should be invalid"; \
			failed=1; \
		else \
			echo "OK  invalid as expected: $$f"; \
		fi; \
	done; \
	if [ "$$failed" -ne 0 ]; then exit 1; fi
	@$(MAKE) fixtures-check
	@$(MAKE) python-test
	@echo "OK: tests passed"

python-test: ## Run Python adapter unit tests in an isolated venv
	@echo "Running Python adapter tests"
	@python3 -m venv $(PYTHON_DIR)/.venv
	@$(PYTHON_DIR)/.venv/bin/pip -q install --upgrade pip
	@$(PYTHON_DIR)/.venv/bin/pip -q install -r $(PYTHON_DIR)/requirements-dev.txt
	@$(PYTHON_DIR)/.venv/bin/pip -q install -e $(PYTHON_DIR)
	@cd $(PYTHON_DIR) && .venv/bin/ruff check .
	@cd $(PYTHON_DIR) && .venv/bin/mypy
	@cd $(PYTHON_DIR) && .venv/bin/pytest

bootstrap-check: ## Validate Milestone 0 repository skeleton
	@./scripts/bootstrap-check.sh

build: fixturectl probe-bin ## Build tooling binaries

fixturectl: ## Build fixturectl into bin/
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(FIXTURECTL) ./tooling/fixturectl

probe-bin: ## Build interop-probe into bin/
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(PROBE) ./tooling/interop-probe

ci: ## Non-interactive CI entrypoint
	@echo "Running make ci"
	@$(MAKE) lint
	@$(MAKE) test
	@$(MAKE) build
	@echo "ci passed"

# --- fixture compilation ---

fixtures: fixturectl ## Compile canonical fixtures into fixtures/compiled/
	@$(FIXTURECTL) compile --schema $(SCHEMA) --out fixtures/compiled $(CANONICAL)

fixtures-check: fixturectl ## Verify compiled goldens match compiler output
	@echo "Checking compiled goldens"
	@tmpdir=$$(mktemp -d); \
	$(FIXTURECTL) compile --schema $(SCHEMA) --out "$$tmpdir" $(CANONICAL); \
	failed=0; \
	for src in $(CANONICAL); do \
		base=$$(basename "$$src" .json); \
		got="$$tmpdir/$$base.compiled.json"; \
		want="fixtures/compiled/$$base.compiled.json"; \
		if ! cmp -s "$$got" "$$want"; then \
			echo "FAIL golden drift: $$want"; \
			failed=1; \
		else \
			echo "OK  $$want"; \
		fi; \
	done; \
	rm -rf "$$tmpdir"; \
	if [ "$$failed" -ne 0 ]; then \
		echo "Update goldens only via: make fixtures-update"; \
		exit 1; \
	fi

fixtures-update: fixturectl ## Explicitly regenerate compiled golden fixtures
	@echo "Updating compiled goldens"
	@$(FIXTURECTL) compile --schema $(SCHEMA) --out fixtures/compiled $(CANONICAL)



native: ## Build native Snap7 server image (linux/amd64)
	@$(DOCKER) build --platform=linux/amd64 \
		-f servers/native/Dockerfile \
		-t $(NATIVE_IMAGE) .

python: ## Build pure-Python server image
	@$(DOCKER) build \
		-f servers/python/Dockerfile \
		-t $(PYTHON_IMAGE) .

images: native python ## Build both server images
	@echo "images: native and python built"

up: ## Start both compose server profiles
	@docker compose --profile native --profile python up -d --build

down: ## Stop compose services
	@docker compose --profile native --profile python --profile servers down --remove-orphans 2>/dev/null || true

probe: probe-bin ## Smoke-probe an S7 server (PROBE_ADDR, default localhost:1102)
	@$(PROBE) -addr $(PROBE_ADDR) -db 1 -offset 0 -length 8 -expect-hex 0102030405060708
	@$(PROBE) -addr $(PROBE_ADDR) -db 1 -offset 8 -length 4 -write-hex deadbeef

smoke-native: native probe-bin ## Build native image, run it, probe read/write, tear down
	@$(MAKE) _smoke-run IMAGE=$(NATIVE_IMAGE) HOST_PORT=1102 PLATFORM=--platform=linux/amd64 LABEL=native

smoke-python: python probe-bin ## Build Python image, run it, probe read/write, tear down
	@$(MAKE) _smoke-run IMAGE=$(PYTHON_IMAGE) HOST_PORT=2102 PLATFORM= LABEL=python

_smoke-run:
	@cid=$$($(DOCKER) run -d --rm $(PLATFORM) \
		-p $(HOST_PORT):102 \
		-v "$(CURDIR)/fixtures/compiled:/fixtures:ro" \
		--read-only \
		--tmpfs /tmp \
		--tmpfs /run/snap-interop:uid=10001,gid=10001,mode=0755 \
		--cap-drop ALL --cap-add NET_BIND_SERVICE \
		-e SNAP_INTEROP_FIXTURE=/fixtures/basic-db.compiled.json \
		$(IMAGE)); \
	echo "started $$cid"; \
	ok=0; \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
		if $(PROBE) -addr 127.0.0.1:$(HOST_PORT) -db 1 -offset 0 -length 8 -expect-hex 0102030405060708 >/tmp/snap-interop-probe.out 2>&1; then \
			ok=1; break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ok" -ne 1 ]; then \
		echo "FAIL: $(LABEL) smoke read"; \
		$(DOCKER) logs "$$cid" || true; \
		cat /tmp/snap-interop-probe.out || true; \
		$(DOCKER) stop "$$cid" >/dev/null || true; \
		exit 1; \
	fi; \
	cat /tmp/snap-interop-probe.out; \
	$(PROBE) -addr 127.0.0.1:$(HOST_PORT) -db 1 -offset 8 -length 4 -write-hex deadbeef; \
	$(DOCKER) stop "$$cid" >/dev/null; \
	echo "smoke-$(LABEL) passed"

interop: ## Full interop gate (Milestone 4)
	@echo "interop: not implemented until Milestone 4"; exit 1

report: ## Generate interop reports (Milestone 4)
	@echo "report: not implemented until Milestone 4"; exit 1

sbom: ## Generate SBOMs (Milestone 6)
	@echo "sbom: not implemented until Milestone 6"; exit 1

scan: ## Vulnerability scan (Milestone 6)
	@echo "scan: not implemented until Milestone 6"; exit 1
