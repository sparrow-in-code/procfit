# procfit — build & developer workflow
# All targets assume `go` is on PATH (see QUESTIONS.md for the nix provisioning note).

BINARY      := procfit
PKG         := github.com/netikras/procfit
CMD         := ./cmd/procfit
BINDIR      := bin
COVERPROFILE := coverage.out
COVER_MIN   := 80

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X $(PKG)/internal/meta.Version=$(VERSION) \
               -X $(PKG)/internal/meta.Commit=$(COMMIT) \
               -X $(PKG)/internal/meta.BuildDate=$(DATE)

# Tools are optional; targets degrade gracefully if a tool is absent.
STATICCHECK := $(shell command -v staticcheck 2>/dev/null)
GOLANGCILINT := $(shell command -v golangci-lint 2>/dev/null)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN{FS":.*##"} /^[a-zA-Z0-9_-]+:.*##/{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the binary into ./bin
	@mkdir -p $(BINDIR)
	go build -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINARY) $(CMD)

.PHONY: install
install: ## go install the binary
	go install -ldflags '$(LDFLAGS)' $(CMD)

.PHONY: run
run: ## Build and run (ARGS="ps ...")
	go run -ldflags '$(LDFLAGS)' $(CMD) $(ARGS)

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: race
race: ## Run tests with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests with coverage and enforce the >=80% floor
	# Coverage gate scopes to ./internal/... — cmd/procfit is a thin main that only
	# delegates to internal/app (which IS tested), an explicit exclusion per
	# DEVELOPMENT.md §3.2.
	go test -coverprofile=$(COVERPROFILE) -covermode=atomic ./internal/...
	@go tool cover -func=$(COVERPROFILE) | tail -1
	@total=$$(go tool cover -func=$(COVERPROFILE) | awk '/^total:/ {gsub(/%/,"",$$3); print $$3}'); \
	echo "total coverage: $$total% (floor $(COVER_MIN)%)"; \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN{ if (t+0 < m+0) { printf("FAIL: coverage %.1f%% < %d%%\n", t, m); exit 1 } else { print "coverage gate OK" } }'

.PHONY: cover-html
cover-html: cover ## Open an HTML coverage report
	go tool cover -html=$(COVERPROFILE)

.PHONY: fuzz-smoke
fuzz-smoke: ## Short run of every fuzz target
	@./scripts/fuzz-smoke.sh 2>/dev/null || echo "no fuzz targets yet (scripts/fuzz-smoke.sh missing)"

.PHONY: vet
vet: ## go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the code
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Fail if code is not gofmt-clean
	@out=$$(gofmt -s -l .); if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi

.PHONY: lint
lint: vet ## Run vet + staticcheck + golangci-lint if available
ifneq ($(STATICCHECK),)
	staticcheck ./...
else
	@echo "staticcheck not installed; skipping"
endif
ifneq ($(GOLANGCILINT),)
	golangci-lint run
else
	@echo "golangci-lint not installed; skipping (size-cap linters)"
endif

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: check
check: fmt-check vet lint test race cover ## Full local gate (what CI runs)

CLANG ?= clang

.PHONY: bpf
bpf: ## Recompile the embedded eBPF object (requires clang with a bpf target)
	$(CLANG) -O2 -target bpf -c internal/procfs/bpf/wakeups.bpf.c -o internal/procfs/bpf/wakeups.bpf.o
	@echo "regenerated internal/procfs/bpf/wakeups.bpf.o (commit it)"

.PHONY: clean
clean: ## Remove build/coverage artifacts
	rm -rf $(BINDIR) $(COVERPROFILE)
