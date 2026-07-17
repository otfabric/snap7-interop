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

.PHONY: help fmt lint test ci bootstrap-check build fixturectl \
	fixtures fixtures-check fixtures-update \
	native python images up down probe interop report sbom scan

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go and other available trees
	@echo "Running gofmt"
	@$(GO) fmt ./...
	@chmod +x scripts/bootstrap-check.sh
	@if command -v ruff >/dev/null 2>&1; then \
		echo "Running ruff format"; \
		ruff format servers/python || true; \
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
	@echo "OK: tests passed"

bootstrap-check: ## Validate Milestone 0 repository skeleton
	@./scripts/bootstrap-check.sh

build: fixturectl ## Build tooling binaries

fixturectl: ## Build fixturectl into bin/
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(FIXTURECTL) ./tooling/fixturectl

ci: ## Non-interactive CI entrypoint
	@echo "Running make ci"
	@$(MAKE) lint
	@$(MAKE) test
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



native: ## Build native server image (Milestone 2)
	@echo "native: not implemented until Milestone 2"; exit 1

python: ## Build Python server image (Milestone 3)
	@echo "python: not implemented until Milestone 3"; exit 1

images: ## Build both server images
	@echo "images: not implemented until Milestones 2 and 3"; exit 1

up: ## Start compose services (requires real images)
	@echo "up: placeholder images are not runnable"; exit 1

down: ## Stop compose services
	@docker compose --profile servers down --remove-orphans 2>/dev/null || true

probe: ## Run independent smoke probe (Milestone 2+)
	@echo "probe: not implemented until Milestone 2"; exit 1

interop: ## Full interop gate (Milestone 4)
	@echo "interop: not implemented until Milestone 4"; exit 1

report: ## Generate interop reports (Milestone 4)
	@echo "report: not implemented until Milestone 4"; exit 1

sbom: ## Generate SBOMs (Milestone 6)
	@echo "sbom: not implemented until Milestone 6"; exit 1

scan: ## Vulnerability scan (Milestone 6)
	@echo "scan: not implemented until Milestone 6"; exit 1
