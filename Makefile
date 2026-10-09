# AI-KBS root Makefile (T0.5 foundation; T0.9 will replace/extend).
#
# This file seeds the canonical entry points that T0.9 will surface; T0.5
# delivers the configuration files (.golangci.yml, pyproject.toml,
# .eslintrc.cjs) and a minimal `make lint` aggregator so that T0.9 can
# wire the rest of the targets without inventing new conventions.
#
# Convention conventions:
#   - Every module ships its own lint config (see backend/.golangci.yml,
#     */pyproject.toml [tool.ruff], frontend/.eslintrc.cjs).
#   - Each `lint-<module>` target delegates to the module-local tool, so a
#     single failure does not abort the aggregate `make lint` prematurely.
#   - Targets are .PHONY; never produce a file called `lint` etc.
#
# T0.9 will extend this Makefile with: test, test-unit, test-integration,
# bdd, bdd-mvp, cli-build, perf, helm-install, helm-uninstall, scrub-audit,
# migrate-up, migrate-down, dev-up, dev-down, docs-build.

SHELL := /usr/bin/env bash

# Tool paths. golangci-lint lives in $GOPATH/bin by default.
GOLANGCI_LINT ?= $(shell go env GOPATH 2>/dev/null)/bin/golangci-lint
ifeq ($(strip $(GOLANGCI_LINT)),)
GOLANGCI_LINT := golangci-lint
endif
RUFF ?= ruff

.PHONY: help lint lint-backend lint-python lint-frontend clean

help:
	@echo "AI-KBS root Makefile (T0.5 foundation)"
	@echo ""
	@echo "Targets:"
	@echo "  lint            Run all configured linters (Go + Python + frontend dry-run)"
	@echo "  lint-backend    Run golangci-lint on backend/"
	@echo "  lint-python     Run ruff check across Python modules"
	@echo "  lint-frontend   Placeholder; frontend lint lands in T0.9"
	@echo "  clean           Remove cached lint artifacts (none yet)"

lint: lint-backend lint-python
	@echo "lint: aggregate pass complete"

lint-backend:
	@echo ">> backend/.golangci.yml (go 1.26, golangci-lint v1.62.0+)"
	@cd backend && $(GOLANGCI_LINT) run --timeout 5m

lint-python:
	@echo ">> ruff check mcp-server/ sag-bridge/ docreader/"
	@cd mcp-server && $(RUFF) check .
	@cd sag-bridge && $(RUFF) check .
	@cd docreader && $(RUFF) check .

lint-frontend:
	@echo ">> frontend lint is dormant in T0.5; wired in T0.9"

clean:
	@echo "no cache yet"