## Tala WTE — Wireless Training Environment
## Usage: make <target>

BINARY     := tala-wte
GO_MODULE  := github.com/vtemlabs/tala-wte
CMD_PATH   := ./cmd/server
WEB_DIR    := web
BUILD_DIR  := $(WEB_DIR)/build
DIST       := dist

# Detect Go toolchain (local)
GO := $(shell which go 2>/dev/null)

# Version stamped into the binary (internal/version.Version). Derived from the
# nearest git tag so local builds report something meaningful; CI overrides this
# with the exact tag (see .github/workflows/release.yml). Falls back to "dev".
VERSION   := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo dev)
LDFLAGS   := -X $(GO_MODULE)/internal/version.Version=$(VERSION)

# Vendor the latest e-terminal into the embed assets (pulled fresh every build,
# trimmed to the Linux bits). go:embed bakes it into the binary so the installer
# sets it up for the operator account WITHOUT cloning at install time. If the
# pull fails, an existing vendored copy is kept; if none exists the build omits
# e-terminal (HasInstaller() returns false, bootstrap skips).
ETERM_VENDOR := internal/eterminal/assets/e-terminal
ETERM_REPO   := https://github.com/tweathers-sec/e-terminal.git

define require_go
	@if [ -z "$(GO)" ]; then \
		echo ""; \
		echo "  ERROR: go toolchain not found in PATH"; \
		echo "  Install Go 1.24+ from https://go.dev/dl/"; \
		echo ""; \
		exit 1; \
	fi
endef

# Colors
CYAN  := \033[0;36m
GREEN := \033[0;32m
RESET := \033[0m

.DEFAULT_GOAL := help

# ─── Help ─────────────────────────────────────────────────────────────────────
.PHONY: help
help:
	@echo "$(CYAN)Tala WTE — Wireless Training Environment$(RESET)"
	@echo ""
	@echo "$(GREEN)Usage:$(RESET) make <target>"
	@echo ""
	@echo "  $(GREEN)dev$(RESET)              Start backend + frontend dev servers"
	@echo "  $(GREEN)dev-backend$(RESET)      Start Go backend only (hot reload via air)"
	@echo "  $(GREEN)dev-frontend$(RESET)     Start SvelteKit dev server only"
	@echo ""
	@echo "  $(GREEN)build$(RESET)            Build everything (web + Go binary)"
	@echo "  $(GREEN)build-web$(RESET)        Build SvelteKit frontend"
	@echo "  $(GREEN)build-go$(RESET)         Build Go binary"
	@echo "  $(GREEN)build-all$(RESET)        Alias for build"
	@echo ""
	@echo "  $(GREEN)linux$(RESET)            Cross-compile Linux amd64 + arm64 (Docker)"
	@echo "  $(GREEN)linux-amd64$(RESET)      Cross-compile Linux amd64 only (Docker)"
	@echo "  $(GREEN)linux-arm64$(RESET)      Cross-compile Linux arm64 only (Docker)"
	@echo "  $(GREEN)release$(RESET)          Frontend + Linux binaries"
	@echo ""
	@echo "  $(GREEN)run$(RESET)              Run the built binary"
	@echo ""
	@echo "  $(GREEN)go-tidy$(RESET)          Run go mod tidy"
	@echo "  $(GREEN)go-fmt$(RESET)           Run gofmt on all Go files"
	@echo "  $(GREEN)go-vet$(RESET)           Run go vet"
	@echo "  $(GREEN)check$(RESET)            Run svelte-check on frontend"
	@echo "  $(GREEN)test$(RESET)             Run Go tests"
	@echo ""
	@echo "  $(GREEN)clean$(RESET)            Remove build artifacts"
	@echo "  $(GREEN)clean-all$(RESET)        Remove all generated files including pb_data"

# ─── Development ──────────────────────────────────────────────────────────────
.PHONY: dev
dev:
	@echo "$(CYAN)Starting Tala WTE dev servers...$(RESET)"
	@$(MAKE) -j2 dev-backend dev-frontend

.PHONY: dev-backend
dev-backend:
	$(call require_go)
	@if command -v air > /dev/null; then \
		air -c .air.toml; \
	else \
		$(GO) run $(CMD_PATH); \
	fi

.PHONY: dev-frontend
dev-frontend:
	cd $(WEB_DIR) && pnpm dev

# ─── Build ────────────────────────────────────────────────────────────────────
.PHONY: build
build: build-web build-go
	@echo "$(GREEN)Build complete: $(DIST)/$(BINARY)$(RESET)"

.PHONY: build-all
build-all: build

.PHONY: linux
linux: linux-amd64 linux-arm64

.PHONY: linux-amd64
linux-amd64: build-web eterminal
	$(call require_go)
	@echo "$(CYAN)Cross-compiling Linux amd64...$(RESET)"
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64 $(CMD_PATH)
	@echo "$(GREEN)-> $(DIST)/$(BINARY)-linux-amd64$(RESET)"

.PHONY: linux-arm64
linux-arm64: build-web eterminal
	$(call require_go)
	@echo "$(CYAN)Cross-compiling Linux arm64...$(RESET)"
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64 $(CMD_PATH)
	@echo "$(GREEN)-> $(DIST)/$(BINARY)-linux-arm64$(RESET)"

.PHONY: release
release: build-web linux
	@echo "$(GREEN)Release build complete$(RESET)"

.PHONY: build-web
build-web:
	@echo "$(CYAN)Building SvelteKit frontend...$(RESET)"
	cd $(WEB_DIR) && pnpm install && pnpm build
	@echo "$(GREEN)Frontend built: $(BUILD_DIR)/$(RESET)"

.PHONY: eterminal
eterminal:
	@echo "$(CYAN)Vendoring latest e-terminal...$(RESET)"
	@rm -rf $(ETERM_VENDOR).tmp
	@if git clone --depth 1 -q $(ETERM_REPO) $(ETERM_VENDOR).tmp 2>/dev/null; then \
		rm -rf $(ETERM_VENDOR).tmp/.git $(ETERM_VENDOR).tmp/.gitignore $(ETERM_VENDOR).tmp/images $(ETERM_VENDOR).tmp/README.md; \
		rm -f  $(ETERM_VENDOR).tmp/config/bin/*-darwin-*; \
		rm -rf $(ETERM_VENDOR); mv $(ETERM_VENDOR).tmp $(ETERM_VENDOR); \
		echo "  -> $(ETERM_VENDOR) ($$(du -sh $(ETERM_VENDOR) | cut -f1), latest upstream)"; \
	elif [ -d $(ETERM_VENDOR) ]; then \
		rm -rf $(ETERM_VENDOR).tmp; echo "  [!] git clone failed; keeping existing vendored copy"; \
	else \
		rm -rf $(ETERM_VENDOR).tmp; echo "  [!] git clone failed and nothing vendored; build will omit e-terminal"; \
	fi

.PHONY: build-go
build-go: go-tidy eterminal
	$(call require_go)
	@echo "$(CYAN)Building Go binary...$(RESET)"
	mkdir -p $(DIST)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(CMD_PATH)
	@echo "$(GREEN)Binary built: $(DIST)/$(BINARY)$(RESET)"

# ─── Run ──────────────────────────────────────────────────────────────────────
.PHONY: run
run:
	@if [ ! -f "$(DIST)/$(BINARY)" ]; then $(MAKE) build; fi
	sudo $(DIST)/$(BINARY) serve --http=0.0.0.0:8090

# ─── Dependencies ─────────────────────────────────────────────────────────────
# System dependencies and host routing are installed by the binary's own
# `install` command (see cmd/server/install.go and internal/deps); there is no
# separate shell-script step.
.PHONY: install-web-deps
install-web-deps:
	cd $(WEB_DIR) && pnpm install

# ─── Go tools ─────────────────────────────────────────────────────────────────
.PHONY: go-tidy
go-tidy:
	$(call require_go)
	$(GO) mod tidy

.PHONY: go-fmt
go-fmt:
	$(call require_go)
	gofmt -w -s .

.PHONY: go-vet
go-vet:
	$(call require_go)
	$(GO) vet ./...

.PHONY: test
test:
	$(call require_go)
	$(GO) test -race ./...

.PHONY: check
check:
	cd $(WEB_DIR) && pnpm check

# ─── Clean ────────────────────────────────────────────────────────────────────
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -f server
	rm -rf $(DIST)
	rm -rf $(BUILD_DIR)
	rm -f /tmp/hostapd-*.conf
	rm -f /tmp/dnsmasq-*.conf

.PHONY: clean-all
clean-all: clean
	rm -rf $(WEB_DIR)/node_modules
	rm -rf $(WEB_DIR)/.svelte-kit
	@echo "$(GREEN)Clean complete$(RESET)"
