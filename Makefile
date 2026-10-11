# AI-KBS root Makefile (T0.5 + T0.8 + T0.9b).
#
# Every target is phony; no file is produced by `make` itself.
#
# Conventions:
#   - Each module ships its own lint/test config (backend/.golangci.yml,
#     */pyproject.toml, frontend/.eslintrc.cjs).
#   - Aggregate targets (lint, test) delegate to module-local targets so a
#     single failing step does not mask the rest.
#   - Targets that need tools absent on the build host (helm, npm) are
#     guarded with a "command -v" check that prints an actionable hint
#     instead of failing the build mysteriously.

SHELL := /usr/bin/env bash

# ---- tool paths -----------------------------------------------------------
GOLANGCI_LINT  ?= $(shell go env GOPATH 2>/dev/null)/bin/golangci-lint
ifeq ($(strip $(GOLANGCI_LINT)),)
GOLANGCI_LINT  := golangci-lint
endif
RUFF           ?= ruff
HELM           ?= helm
DOCKER_COMPOSE ?= docker compose
NPM            ?= npm

# ---- repo paths -----------------------------------------------------------
BACKEND_DIR    := backend
MCP_DIR        := mcp-server
SAG_DIR        := sag-bridge
DOCREADER_DIR  := docreader
FRONTEND_DIR   := frontend
HELM_DIR       := helm
DOCS_DIR       := docs
DOCKER_DEV     := docker-compose.dev.yml

# ---- default --------------------------------------------------------------
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "AI-KBS root Makefile"
	@echo ""
	@echo "Quality gates:"
	@echo "  lint                    Run all configured linters"
	@echo "  lint-backend            golangci-lint on backend/"
	@echo "  lint-python             ruff across Python modules"
	@echo "  lint-frontend           eslint on frontend/"
	@echo "  test                    Aggregate tests (unit + integration)"
	@echo "  test-unit               Backend go test ./... (no integration)"
	@echo "  test-integration        docker compose up + backend integration tests"
	@echo "  bdd                     All Godog BDD specs"
	@echo "  bdd-mvp                 MVP-critical BDD specs only"
	@echo "  scrub-audit             Grep for forbidden vendor / internal strings"
	@echo ""
	@echo "Database migrations:"
	@echo "  migrate-up              Apply all pending migrations (uses AIKBS_DB_URL)"
	@echo "  migrate-down            Roll back N steps (default 1; set steps=N)"
	@echo "  migrate-status          Show current version + dirty flag"
	@echo "  migrate-force           Force schema_migrations to a version (set version=N)"
	@echo ""
	@echo "Build & run:"
	@echo "  cli-build               Build backend/cmd/cli binary to ./bin/clai"
	@echo "  perf                    Run performance benchmarks (M2/M3 fixture)"
	@echo ""
	@echo "Dev stack (docker compose):"
	@echo "  dev-up                  Start the 9-service dev stack in background"
	@echo "  dev-down                Stop + remove containers + volumes"
	@echo ""
	@echo "Kubernetes:"
	@echo "  helm-install            helm install ai-kbs ./helm (set KUBECONFIG=...)"
	@echo "  helm-uninstall          helm uninstall ai-kbs"
	@echo ""
	@echo "Docs (VitePress):"
	@echo "  docs-build              Production build to docs/.vitepress/dist"
	@echo "  docs-dev                Live preview on http://localhost:5174"
	@echo ""
	@echo "Misc:"
	@echo "  clean                   Remove build/test caches"

# ============================================================================
# Quality gates
# ============================================================================

.PHONY: lint lint-backend lint-python lint-frontend
lint: lint-backend lint-python lint-frontend
	@echo "lint: aggregate pass complete"

lint-backend:
	@echo ">> backend/.golangci.yml"
	@cd $(BACKEND_DIR) && $(GOLANGCI_LINT) run --timeout 5m

lint-python:
	@echo ">> ruff check mcp-server/ sag-bridge/ docreader/"
	@cd $(MCP_DIR)       && $(RUFF) check .
	@cd $(SAG_DIR)       && $(RUFF) check .
	@cd $(DOCREADER_DIR) && $(RUFF) check .

lint-frontend:
	@if [ -d "$(FRONTEND_DIR)" ]; then \
	  echo ">> frontend/.eslintrc.cjs"; \
	  cd $(FRONTEND_DIR) && npx --no-install eslint . ; \
	else \
	  echo ">> frontend/ not yet scaffolded (M1)"; \
	fi

# ---- tests ----------------------------------------------------------------

.PHONY: test test-unit test-integration bdd bdd-mvp
test: test-unit
	@echo "test: unit pass complete (integration requires dev-up)"

test-unit:
	@echo ">> go test ./..."
	@cd $(BACKEND_DIR) && go test -count=1 -short ./...

# Integration tests need the dev stack; we start it, run, and tear down.
# `INTEGRATION=1` is the env opt-in so accidental runs on a workstation do
# not silently spin up Postgres / Redis / etc.
test-integration:
	@if [ "$(INTEGRATION)" != "1" ]; then \
	  echo "INTEGRATION=1 required (test-integration boots the dev stack)"; \
	  exit 1; \
	fi
	@$(MAKE) dev-up
	@trap '$(MAKE) dev-down' EXIT; \
	 cd $(BACKEND_DIR) && go test -count=1 -tags=integration ./test/integration/...

.PHONY: bdd bdd-mvp
bdd:
	@if [ -d "$(BACKEND_DIR)/test/bdd" ]; then \
	  cd $(BACKEND_DIR) && go test -count=1 ./test/bdd/... ; \
	else \
	  echo ">> bdd specs land in M1" ; \
	fi

bdd-mvp:
	@if [ -d "$(BACKEND_DIR)/test/bdd" ]; then \
	  cd $(BACKEND_DIR) && go test -count=1 -tags=mvp ./test/bdd/... ; \
	else \
	  echo ">> bdd-mvp specs land in M1" ; \
	fi

# ============================================================================
# Build & run
# ============================================================================

.PHONY: cli-build perf
cli-build:
	@mkdir -p bin
	@cd $(BACKEND_DIR) && go build -o ../bin/clai ./cmd/cli
	@echo "built ./bin/clai"

# Performance fixture — wired but no benchmarks until M2/M3 land.
# NFR targets (see docker/langfuse/dashboards/ai-kbs-default.json):
#   p50 < 200ms, p95 < 800ms, p99 < 2s on hybrid search.
perf:
	@if [ -d "$(BACKEND_DIR)/test/perf" ]; then \
	  cd $(BACKEND_DIR) && go test -count=1 -tags=perf -bench=. -benchmem ./test/perf/... ; \
	else \
	  echo ">> perf fixtures land in M2 (no benchmarks yet)" ; \
	fi

# ============================================================================
# Dev stack
# ============================================================================

.PHONY: dev-up dev-down
dev-up:
	@if [ ! -f .env ]; then \
	  echo "WARN: .env missing; copying from .env.example"; \
	  cp -n .env.example .env ; \
	fi
	@$(DOCKER_COMPOSE) -f $(DOCKER_DEV) up -d
	@echo "dev stack up. Check: docker compose -f $(DOCKER_DEV) ps"

dev-down:
	@$(DOCKER_COMPOSE) -f $(DOCKER_DEV) down -v
	@echo "dev stack down (volumes removed)"

# ============================================================================
# Helm
# ============================================================================

.PHONY: helm-install helm-uninstall
helm-install:
	@command -v $(HELM) >/dev/null || { echo "helm not on PATH"; exit 1; }
	@$(HELM) upgrade --install ai-kbs $(HELM_DIR) --create-namespace --namespace ai-kbs

helm-uninstall:
	@command -v $(HELM) >/dev/null || { echo "helm not on PATH"; exit 1; }
	@$(HELM) uninstall ai-kbs --namespace ai-kbs

# ============================================================================
# Docs (VitePress)
# ============================================================================

.PHONY: docs-build docs-dev
docs-build:
	@command -v $(NPM) >/dev/null || { echo "npm not on PATH"; exit 1; }
	@cd $(DOCS_DIR) && $(NPM) install --no-audit --no-fund
	@cd $(DOCS_DIR) && $(NPM) run docs:build

docs-dev:
	@command -v $(NPM) >/dev/null || { echo "npm not on PATH"; exit 1; }
	@cd $(DOCS_DIR) && $(NPM) install --no-audit --no-fund
	@cd $(DOCS_DIR) && $(NPM) run docs:dev -- --port 5174

# ============================================================================
# Scrub audit (CI gate)
# ============================================================================
#
# Per ADR-007: AI-KBS-authored infrastructure files must not contain vendor
# or internal-platform names (内部 Wiki 系统, 钉钉, 飞书, 企微, 咕噜,
# upstream-fork internal network names, etc.). Forked source under backend/,
# mcp-server/, sag-bridge/, docreader/, frontend/ is exempt — those names
# legitimately appear in upstream code we have not rewritten.

SCRUB_PATTERNS := 内部wiki|钉钉|飞书|企微|咕噜|legacy.weknora|wiki内部|wiki system
# Scope: only AI-KBS-authored infra / docs surface. Fork code is excluded.
SCRUB_ROOTS    := Makefile \
                  .golangci.yml .eslintrc.cjs .pre-commit-config.yaml \
                  .env.example docker-compose.dev.yml \
                  docker helm .github
# Common noise filters
SCRUB_EXCLUDES := --exclude-dir=node_modules --exclude-dir=.git \
                  --exclude-dir=dist --exclude-dir=.vitepress \
                  --exclude-dir=bin --exclude=*.lock

.PHONY: scrub-audit
scrub-audit:
	@echo "scrub-audit: scanning for $(SCRUB_PATTERNS)"
	@echo "             in: $(SCRUB_ROOTS)"
	@if grep -RInE "$(SCRUB_PATTERNS)" $(SCRUB_EXCLUDES) --exclude=Makefile $(SCRUB_ROOTS); then \
	  echo ""; \
	  echo "FAIL: scrub-audit found forbidden strings above."; \
	  echo "These are vendor / internal-platform names that must not appear"; \
	  echo "in the public codebase (ADR-007). Remove them and re-run."; \
	  exit 1; \
	fi
	@echo "scrub-audit: clean"

# ============================================================================
# Migrations (T0.8)
# ============================================================================
#
# Wraps backend/cmd/migrate, which uses the same golang-migrate library the
# server uses at startup. AIKBS_DB_URL must point at the target database.

MIGRATE_BIN   := $(BACKEND_DIR)/bin/migrate
MIGRATE_PKG   := ./cmd/migrate
MIGRATE_FLAGS ?= -dsn "$(AIKBS_DB_URL)"

.PHONY: migrate-up migrate-down migrate-status migrate-force
migrate-up: $(MIGRATE_BIN)
	@cd $(BACKEND_DIR) && $(MIGRATE_BIN) $(MIGRATE_FLAGS) -action up

migrate-down: $(MIGRATE_BIN)
	@cd $(BACKEND_DIR) && $(MIGRATE_BIN) $(MIGRATE_FLAGS) -action down -steps $(or $(steps),1)

migrate-status: $(MIGRATE_BIN)
	@cd $(BACKEND_DIR) && $(MIGRATE_BIN) $(MIGRATE_FLAGS) -action status

migrate-force: $(MIGRATE_BIN)
	@if [ -z "$(version)" ]; then \
	  echo "ERROR: migrate-force requires version=N"; exit 1; \
	fi
	@cd $(BACKEND_DIR) && $(MIGRATE_BIN) $(MIGRATE_FLAGS) -action force -version $(version)

$(MIGRATE_BIN):
	@mkdir -p $(BACKEND_DIR)/bin
	@cd $(BACKEND_DIR) && go build -o bin/migrate $(MIGRATE_PKG)

# ============================================================================
# Cleanup
# ============================================================================

.PHONY: clean
clean:
	@rm -rf $(BACKEND_DIR)/bin bin
	@$(DOCKER_COMPOSE) -f $(DOCKER_DEV) down -v --remove-orphans 2>/dev/null || true
	@echo "clean: build/test caches removed"