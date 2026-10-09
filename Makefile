# AI-KBS root Makefile (T0.5 + T0.8).
#
# Every target is phony; no file is produced by `make` itself.
#
# Conventions:
#   - Each module ships its own lint config (backend/.golangci.yml,
#     */pyproject.toml, frontend/.eslintrc.cjs).
#   - Aggregate targets (lint) delegate to module-local targets so a single
#     failing step does not mask the rest.
#   - Targets that need tools absent on the build host (helm, npm) are
#     guarded with a "command -v" check that prints an actionable hint
#     instead of failing the build mysteriously.
#
# T0.9 will extend this with: test, test-unit, test-integration, bdd,
# bdd-mvp, cli-build, perf, dev-up, dev-down, helm-install, helm-uninstall,
# scrub-audit, docs-build, docs-dev.

SHELL := /usr/bin/env bash

# ---- tool paths -----------------------------------------------------------
GOLANGCI_LINT ?= $(shell go env GOPATH 2>/dev/null)/bin/golangci-lint
ifeq ($(strip $(GOLANGCI_LINT)),)
GOLANGCI_LINT := golangci-lint
endif
RUFF          ?= ruff

# ---- repo paths -----------------------------------------------------------
BACKEND_DIR   := backend
MCP_DIR       := mcp-server
SAG_DIR       := sag-bridge
DOCREADER_DIR := docreader

# ---- default --------------------------------------------------------------
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "AI-KBS root Makefile (T0.5 + T0.8)"
	@echo ""
	@echo "Quality gates:"
	@echo "  lint                    Run all configured linters"
	@echo "  lint-backend            golangci-lint on backend/"
	@echo "  lint-python             ruff across Python modules"
	@echo "  lint-frontend           Placeholder; frontend lint lands in T0.9"
	@echo ""
	@echo "Database migrations:"
	@echo "  migrate-up              Apply all pending migrations (uses AIKBS_DB_URL)"
	@echo "  migrate-down            Roll back N steps (default 1; set steps=N)"
	@echo "  migrate-status          Show current version + dirty flag"
	@echo "  migrate-force           Force schema_migrations to a version (set version=N)"
	@echo ""
	@echo "Misc:"
	@echo "  clean                   Remove cached artifacts"

# ============================================================================
# Quality gates
# ============================================================================

.PHONY: lint lint-backend lint-python lint-frontend
lint: lint-backend lint-python
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
	@echo ">> frontend lint is dormant in T0.5/T0.8; wired in T0.9"

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
	@echo "clean: build/test caches removed"