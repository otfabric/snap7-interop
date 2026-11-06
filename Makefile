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
PYTHON_DIR := adapters/python

# The adapters (adapters/<name>/) this repository builds and tests, and those a
# release publishes. own/ is apart: make -f own/go-s7comm/Makefile.
ADAPTERS ?= native python
PUBLISHED_ADAPTERS ?= native python
# local: the images `make images` builds; vX.Y.Z: the published images of a release.
VERSION ?= local
REGISTRY ?= ghcr.io/otfabric
REPORT_DIR ?= reports
PLATFORMS ?= linux/amd64,linux/arm64
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
RETRY := scripts/retry.sh

.PHONY: help all fmt lint test ci validate documents bootstrap-check build fixturectl probe-bin \
	fixtures fixtures-check fixtures-update capabilities print-adapters print-published-adapters \
	buildx-setup buildx publish-arch publish-manifest digests clean smoke \
	native python images publish up down probe smoke-native smoke-python \
	candidate-native candidate-python python-test interop report sbom scan notices fuzz

all: validate images ## Validate fixtures and versions and build the local images

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

ci: ## Non-interactive CI entrypoint (no Docker): lint, tests, validate, build tooling
	@echo "Running make ci"
	@$(MAKE) lint
	@$(MAKE) test
	@$(MAKE) validate
	@$(MAKE) build
	@echo "ci passed"

validate: ## Check tests/cases.tsv and known-bugs.tsv against the fixtures, the pins against versions.yaml, the RELEASE.md notes
	@python3 scripts/validate-fixtures.py
	@scripts/check-versions.sh
	@scripts/release-notes.sh > /dev/null && echo "ok   RELEASE.md has the notes of interop_version"

documents: ## Check $(REPORT_DIR)/results.json of the last interop run against schemas/ and the run rules
	@python3 scripts/interop_result.py $(REPORT_DIR)/results.json

capabilities: probe-bin ## Print the capability tokens every server is run with (docs/CAPABILITIES.md)
	@$(PROBE) capabilities

print-adapters: ## Print ADAPTERS
	@echo $(ADAPTERS)

print-published-adapters: ## Print PUBLISHED_ADAPTERS
	@echo $(PUBLISHED_ADAPTERS)

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
export VERSION REGISTRY REPORT_DIR BUILD_PLATFORM

native: ## Build native Snap7 server image (BUILD_PLATFORM=linux/amd64|linux/arm64)
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f adapters/native/Dockerfile \
		-t $(NATIVE_IMAGE) .

python: ## Build pure-Python server image (BUILD_PLATFORM optional override)
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f adapters/python/Dockerfile \
		-t $(PYTHON_IMAGE) .

images: $(ADAPTERS) ## Build every adapter image for BUILD_PLATFORM as snap7-interop-<adapter>:local
	@echo "images: $(ADAPTERS) built"

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

smoke: images probe-bin ## Container-contract smoke test of every adapter (scripts/smoke.sh)
	@for a in $(ADAPTERS); do scripts/smoke.sh $$a || exit 1; done

smoke-native: native probe-bin ## Build the native image and run its contract smoke test
	@scripts/smoke.sh native

smoke-python: python probe-bin ## Build the Python image and run its contract smoke test
	@HOST_PORT=2102 scripts/smoke.sh python

# Candidates of the Upstream workflow (.github/workflows/upstream.yml): build an
# image at the latest upstream and smoke-probe it. Pins are not changed: the
# Python candidate swaps requirements.txt for the build only and restores it.
candidate-native: probe-bin ## Build + smoke the native image at a Snap7 commit (CANDIDATE_SHA=...)
	@test -n "$(CANDIDATE_SHA)" || { echo "CANDIDATE_SHA=<commit> is required" >&2; exit 1; }
	@$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f adapters/native/Dockerfile \
		--build-arg SNAP7_REVISION=$(CANDIDATE_SHA) \
		-t snap7-interop-native:candidate .
	@ADAPTER_IMAGE_native=snap7-interop-native:candidate scripts/smoke.sh native

candidate-python: probe-bin ## Build + smoke the Python image at a python-snap7 release (CANDIDATE_VERSION=...)
	@test -n "$(CANDIDATE_VERSION)" || { echo "CANDIDATE_VERSION=<version> is required" >&2; exit 1; }
	@cp adapters/python/requirements.txt $(BIN_DIR)/requirements.pinned; \
	sed -E 's/^python-snap7==.*/python-snap7==$(CANDIDATE_VERSION)/' $(BIN_DIR)/requirements.pinned > adapters/python/requirements.txt; \
	rc=0; \
	$(DOCKER) build --platform=$(BUILD_PLATFORM) \
		-f adapters/python/Dockerfile \
		-t snap7-interop-python:candidate . || rc=$$?; \
	cp $(BIN_DIR)/requirements.pinned adapters/python/requirements.txt; \
	exit $$rc
	@ADAPTER_IMAGE_python=snap7-interop-python:candidate HOST_PORT=2102 scripts/smoke.sh python

interop: images probe-bin ## Cross-stack self-test: every case of tests/cases.tsv on the two ADAPTERS (FIXTURE=id optional)
	@scripts/interop.sh $(ADAPTERS)

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

# --- multi-arch builds and publishing (the Release workflow builds natively per architecture) ---

buildx-setup: ## Prepare multi-arch builds: a docker-container builder
	@$(DOCKER) buildx inspect snap7-interop-publisher >/dev/null 2>&1 || $(DOCKER) buildx create --name snap7-interop-publisher >/dev/null
	@$(DOCKER) buildx use snap7-interop-publisher

buildx: buildx-setup ## Build every published image for PLATFORMS, without pushing (local check)
	@for a in $(PUBLISHED_ADAPTERS); do \
		$(RETRY) $(DOCKER) buildx build --platform $(PLATFORMS) -f adapters/$$a/Dockerfile -t snap7-interop-$$a:buildx . || exit 1; \
	done

publish-arch: ## Push the local images of this machine as <image>:$(VERSION)-$(ARCH) (VERSION=vX.Y.Z; build and test them first)
	@case "$(VERSION)" in v*.*.*) ;; *) echo "VERSION=vX.Y.Z is required" >&2; exit 1;; esac
	@for a in $(PUBLISHED_ADAPTERS); do \
		$(DOCKER) tag snap7-interop-$$a:local $(REGISTRY)/snap7-interop-$$a:$(VERSION)-$(ARCH) && \
		$(RETRY) $(DOCKER) push --quiet $(REGISTRY)/snap7-interop-$$a:$(VERSION)-$(ARCH) || exit 1; \
	done

publish-manifest: ## Join the per-architecture images of VERSION into <image>:vX.Y.Z and :X.Y.Z
	@case "$(VERSION)" in v*.*.*) ;; *) echo "VERSION=vX.Y.Z is required" >&2; exit 1;; esac
	@for a in $(PUBLISHED_ADAPTERS); do \
		i=$(REGISTRY)/snap7-interop-$$a; \
		$(RETRY) $(DOCKER) buildx imagetools create --tag $$i:$(VERSION) --tag $$i:$(patsubst v%,%,$(VERSION)) $$i:$(VERSION)-amd64 $$i:$(VERSION)-arm64 || exit 1; \
	done

digests: ## Print "<adapter> <digest>" of the published multi-arch images of VERSION
	@case "$(VERSION)" in v*.*.*) ;; *) echo "VERSION=vX.Y.Z is required" >&2; exit 1;; esac
	@for a in $(PUBLISHED_ADAPTERS); do \
		echo "$$a $$($(DOCKER) buildx imagetools inspect $(REGISTRY)/snap7-interop-$$a:$(VERSION) --format '{{.Manifest.Digest}}')"; \
	done

clean: ## Remove test containers, the local images, reports/ results and bin/
	@$(DOCKER) ps -aq --filter "label=snap7-interop.run" | xargs -r $(DOCKER) rm -f >/dev/null 2>&1 || true
	@$(DOCKER) compose down --remove-orphans 2>/dev/null || true
	@for a in $(ADAPTERS); do $(DOCKER) rmi snap7-interop-$$a:local 2>/dev/null || true; done
	@find reports -mindepth 1 -not -name .gitkeep -delete 2>/dev/null || true
	@find bin -mindepth 1 -delete 2>/dev/null || true
