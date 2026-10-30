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
NATIVE_IMAGE ?= snap7-interop-native:local
PYTHON_IMAGE ?= snap7-interop-python:local
PROBE_ADDR ?= 127.0.0.1:1102
PYTHON_DIR := servers/python

.PHONY: help fmt lint test ci bootstrap-check build fixturectl probe-bin \
	fixtures fixtures-check fixtures-update \
	native python images publish up down probe smoke-native smoke-python \
	candidate-native candidate-python python-test _smoke-run interop report sbom scan notices fuzz

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
	@chmod +x scripts/bootstrap-check.sh scripts/check-notices.sh scripts/sbom.sh scripts/scan.sh scripts/publish.sh scripts/release-notes.sh
	@./scripts/bootstrap-check.sh
	@scripts/release-notes.sh > /dev/null && echo "ok   RELEASE.md has the notes of interop_version"
	@./scripts/check-notices.sh
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
	@python3 -m unittest discover -q -s scripts -p 'test_*.py'
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

bootstrap-check: ## Validate repository skeleton and required docs
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



# BUILD_PLATFORM selects image arch (linux/amd64 or linux/arm64).
BUILD_PLATFORM ?= linux/amd64

native: ## Build native Snap7 server image (BUILD_PLATFORM=linux/amd64|linux/arm64)
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f servers/native/Dockerfile \
		-t $(NATIVE_IMAGE) .

python: ## Build pure-Python server image (BUILD_PLATFORM optional override)
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f servers/python/Dockerfile \
		-t $(PYTHON_IMAGE) .

images: native python ## Build both server images
	@echo "images: native and python built"

# IMAGE_TAG defaults to versions.yaml interop_version (e.g. 0.1.0 → tags v0.1.0 and 0.1.0).
# Requires: docker login ghcr.io (write:packages). Public package visibility set in GHCR UI.
publish: ## Multi-arch build + push both images to ghcr.io/otfabric
	@chmod +x scripts/publish.sh
	@./scripts/publish.sh

up: ## Start both compose server profiles
	@docker compose --profile native --profile python up -d --build

down: ## Stop compose services
	@docker compose --profile native --profile python --profile servers down --remove-orphans 2>/dev/null || true

probe: probe-bin ## Smoke-probe an S7 server (PROBE_ADDR, default localhost:1102)
	@$(PROBE) -addr $(PROBE_ADDR) -db 1 -offset 0 -length 8 -expect-hex 0102030405060708
	@$(PROBE) -addr $(PROBE_ADDR) -db 1 -offset 8 -length 4 -write-hex deadbeef

smoke-native: native probe-bin ## Build native image, run it, probe read/write, tear down
	@$(MAKE) _smoke-run IMAGE=$(NATIVE_IMAGE) HOST_PORT=1102 RUN_PLATFORM=--platform=$(BUILD_PLATFORM) LABEL=native

smoke-python: python probe-bin ## Build Python image, run it, probe read/write, tear down
	@$(MAKE) _smoke-run IMAGE=$(PYTHON_IMAGE) HOST_PORT=2102 RUN_PLATFORM=--platform=$(BUILD_PLATFORM) LABEL=python

# Candidates of the Upstream workflow (.github/workflows/upstream.yml): build an
# image at the latest upstream and smoke-probe it. Pins are not changed: the
# Python candidate swaps requirements.txt for the build only and restores it.
candidate-native: probe-bin ## Build + smoke the native image at a Snap7 commit (CANDIDATE_SHA=...)
	@test -n "$(CANDIDATE_SHA)" || { echo "CANDIDATE_SHA=<commit> is required" >&2; exit 1; }
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f servers/native/Dockerfile \
		--build-arg SNAP7_REVISION=$(CANDIDATE_SHA) \
		-t snap7-interop-native:candidate .
	@$(MAKE) _smoke-run IMAGE=snap7-interop-native:candidate HOST_PORT=1102 RUN_PLATFORM=--platform=$(BUILD_PLATFORM) LABEL=native

candidate-python: probe-bin ## Build + smoke the Python image at a python-snap7 release (CANDIDATE_VERSION=...)
	@test -n "$(CANDIDATE_VERSION)" || { echo "CANDIDATE_VERSION=<version> is required" >&2; exit 1; }
	@cp servers/python/requirements.txt $(BIN_DIR)/requirements.pinned; \
	sed -E 's/^python-snap7==.*/python-snap7==$(CANDIDATE_VERSION)/' $(BIN_DIR)/requirements.pinned > servers/python/requirements.txt; \
	rc=0; \
	$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f servers/python/Dockerfile \
		-t snap7-interop-python:candidate . || rc=$$?; \
	cp $(BIN_DIR)/requirements.pinned servers/python/requirements.txt; \
	exit $$rc
	@$(MAKE) _smoke-run IMAGE=snap7-interop-python:candidate HOST_PORT=2102 RUN_PLATFORM=--platform=$(BUILD_PLATFORM) LABEL=python

_smoke-run:
	@cid=$$($(DOCKER) run -d --rm $(RUN_PLATFORM) \
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
		if $(PROBE) -addr 127.0.0.1:$(HOST_PORT) -db 1 -offset 0 -length 8 -expect-hex 0102030405060708 >/tmp/snap7-interop-probe.out 2>&1; then \
			ok=1; break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ok" -ne 1 ]; then \
		echo "FAIL: $(LABEL) smoke read"; \
		$(DOCKER) logs "$$cid" || true; \
		cat /tmp/snap7-interop-probe.out || true; \
		$(DOCKER) stop "$$cid" >/dev/null || true; \
		exit 1; \
	fi; \
	cat /tmp/snap7-interop-probe.out; \
	$(PROBE) -addr 127.0.0.1:$(HOST_PORT) -db 1 -offset 8 -length 4 -write-hex deadbeef; \
	$(DOCKER) stop "$$cid" >/dev/null; \
	echo "smoke-$(LABEL) passed"

interop: ## Build both images, compare fixtures, write reports/ (FIXTURE=id optional)
	@chmod +x scripts/interop-run.sh
	@./scripts/interop-run.sh

report: ## Show latest interop markdown reports
	@ls -1 reports/*/report.md 2>/dev/null || echo "no reports yet; run make interop"

notices: ## Verify THIRD_PARTY_NOTICES.md and digest pins
	@chmod +x scripts/check-notices.sh
	@./scripts/check-notices.sh

fuzz: ## Short fixture parser/compiler fuzz smoke
	@$(GO) test ./internal/fixture/ -fuzz=FuzzValidate -fuzztime=20s
	@$(GO) test ./internal/fixture/ -fuzz=FuzzCompile -fuzztime=10s

sbom: ## Generate SPDX + CycloneDX SBOMs under dist/sbom/
	@chmod +x scripts/sbom.sh
	@./scripts/sbom.sh

scan: ## Trivy scan local images (SCAN_SEVERITY=CRITICAL,HIGH)
	@chmod +x scripts/scan.sh
	@./scripts/scan.sh
