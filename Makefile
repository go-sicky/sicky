# sicky verification gate.
#
#   make verify   every commit — no network, fails fast (~19s warm)
#   make full     periodically — adds tidy, vuln, cover
#
# There is no CI in this repository. The pre-commit hook runs `make verify`,
# so these targets are the single source of truth for both a human running
# them by hand and the hook.

GO           ?= go
GOLANGCI     ?= golangci-lint
GOVULNCHECK  ?= govulncheck
COVERPROFILE ?= coverage.out
TIMEOUT      ?= 5m

.PHONY: verify full fmt fmt-check vet lint build test race cover vuln tidy-check hook-install clean help

# The per-commit gate. Deliberately excludes vuln (needs network) and
# tidy-check, so a commit never fails for a reason unrelated to the diff.
## verify: fmt + vet + lint + build + race. The per-commit gate.
verify: fmt-check vet lint build race
	@echo "verify: OK"

## full: everything. Run before releasing.
full: verify tidy-check vuln cover
	@echo "full: OK"

## fmt: apply the configured formatters in place.
fmt:
	$(GOLANGCI) fmt

# golangci-lint fmt, not `gofmt -l`: .golangci.yml enables gofumpt and
# goimports (local-prefixes github.com/go-sicky/sicky) on top of gofmt, so
# the formatter gate is strictly wider than `gofmt -l`.
## fmt-check: fail if formatting drifts.
fmt-check:
	@out="$$($(GOLANGCI) fmt --diff 2>&1)"; \
	if [ -n "$$out" ]; then echo "format drift:"; echo "$$out"; exit 1; fi

## vet: go vet.
vet:
	$(GO) vet ./...

## lint: golangci-lint with .golangci.yml.
lint:
	$(GOLANGCI) run --timeout $(TIMEOUT)

## build: compile every package.
build:
	$(GO) build ./...

## test: unit tests (cached).
test:
	$(GO) test ./...

# -count=1 defeats the test cache; without it the gate proves nothing.
## race: tests under the race detector, cache disabled.
race:
	$(GO) test -race -count=1 ./...

## cover: write coverage.out and print the total.
cover:
	$(GO) test -coverprofile=$(COVERPROFILE) ./...
	@$(GO) tool cover -func=$(COVERPROFILE) | tail -1

# govulncheck has no allowlist flag, so the wrapper diffs its findings
# against .vuln-allow (see scripts/vuln-allow.sh).
## vuln: reachability-aware CVE scan, minus .vuln-allow.
vuln:
	@scripts/vuln-allow.sh

# -diff reports without writing. `go mod tidy && git diff --exit-code`
# mutates first and leaves a dirty tree when it fails.
## tidy-check: fail if go.mod/go.sum are not tidy.
tidy-check:
	$(GO) mod tidy -diff

# core.hooksPath is local git config and cannot be committed, so the hook
# script is versioned under .githooks/ and enabled with this one command.
## hook-install: enable the pre-commit hook (run once per clone).
hook-install:
	@git config core.hooksPath .githooks
	@echo "installed. core.hooksPath is local git config, not versioned."

## clean: remove coverage.out.
clean:
	rm -f $(COVERPROFILE)

## help: list targets.
help:
	@grep -E '^## [a-z-]+: ' $(MAKEFILE_LIST) | sed 's/^## /  /'