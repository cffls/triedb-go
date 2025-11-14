SHELL := /bin/bash

# Detect OS
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    LIB_EXT := so
    LIB_PATH_VAR := LD_LIBRARY_PATH
endif
ifeq ($(UNAME_S),Darwin)
    LIB_EXT := dylib
    LIB_PATH_VAR := DYLD_LIBRARY_PATH
endif

# Paths
FFI_DIR := triedb-ffi
GO_DIR := triedb-go
TARGET_DIR := $(FFI_DIR)/target/release
LIB_NAME := libtriedb_ffi.$(LIB_EXT)
STATIC_LIB := libtriedb_ffi.a
HEADER := triedb.h

.PHONY: all build-ffi build-go test-go example install clean help bench-go

all: build-ffi build-go ## Build everything

help: ## Show this help message
	@echo "TrieDB FFI Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build-ffi: ## Build Rust FFI library (release mode)
	@echo "Building Rust FFI library..."
	cd $(FFI_DIR) && cargo build --release
	@echo "Generating C header..."
	cd $(FFI_DIR) && cbindgen --config cbindgen.toml --crate triedb-ffi --output $(HEADER) 2>/dev/null || \
		echo "Warning: cbindgen not found. Run: cargo install cbindgen"
	@echo "FFI library built: $(TARGET_DIR)/$(LIB_NAME)"

build-go: build-ffi ## Build Go bindings
	@echo "Building Go bindings..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		go build -v
	@echo "Go bindings built successfully"

test-go: build-ffi ## Run Go example
	@echo "Running Go example..."
	cd $(GO_DIR)/example && \
		CGO_LDFLAGS="-L../../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go run main.go

example: test-go ## Alias for test-go

install: build-ffi ## Install library and header system-wide (requires sudo)
	@echo "Installing library system-wide..."
	sudo cp $(TARGET_DIR)/$(LIB_NAME) /usr/local/lib/
	sudo cp $(TARGET_DIR)/$(STATIC_LIB) /usr/local/lib/
	sudo cp $(FFI_DIR)/$(HEADER) /usr/local/include/
ifeq ($(UNAME_S),Linux)
	sudo ldconfig
endif
	@echo "Installation complete"
	@echo "You can now build Go projects without setting CGO_LDFLAGS"

uninstall: ## Uninstall library and header from system
	@echo "Uninstalling..."
	sudo rm -f /usr/local/lib/$(LIB_NAME)
	sudo rm -f /usr/local/lib/$(STATIC_LIB)
	sudo rm -f /usr/local/include/$(HEADER)
ifeq ($(UNAME_S),Linux)
	sudo ldconfig
endif
	@echo "Uninstallation complete"

clean: ## Clean all build artifacts
	@echo "Cleaning build artifacts..."
	cd $(FFI_DIR) && cargo clean
	cd $(GO_DIR) && go clean
	rm -f $(FFI_DIR)/$(HEADER)
	rm -f $(GO_DIR)/example/*.db
	rm -f $(GO_DIR)/example/*.db.meta
	@echo "Clean complete"

check-deps: ## Check if all dependencies are installed
	@echo "Checking dependencies..."
	@command -v cargo >/dev/null 2>&1 || { echo "Error: cargo not found. Install Rust from https://rustup.rs"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "Error: go not found. Install Go from https://go.dev"; exit 1; }
	@command -v cbindgen >/dev/null 2>&1 || { echo "Warning: cbindgen not found. Install with: cargo install cbindgen"; }
	@echo "All required dependencies are installed"

bench-ffi: ## Run Rust benchmarks (requires triedb core)
	@echo "Note: This requires the core triedb crate to be available"
	@echo "Run from the core triedb repository instead"

bench-go: build-ffi ## Run Go benchmarks
	@echo "Running Go benchmarks..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=. -benchmem -benchtime=5s

bench-go-quick: build-ffi ## Run quick Go benchmarks (1s each)
	@echo "Running quick Go benchmarks..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=. -benchmem -benchtime=1s

bench-go-compare: build-ffi ## Run comparison benchmarks (TrieDB vs in-memory)
	@echo "Running comparison benchmarks..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=BenchmarkCompare -benchmem -benchtime=5s

bench-go-concurrent: build-ffi ## Run concurrent read benchmarks
	@echo "Running concurrent benchmarks..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=BenchmarkConcurrent -benchmem -cpu=1,2,4,8

bench-go-profile: build-ffi ## Run benchmarks with CPU profiling
	@echo "Running benchmarks with CPU profiling..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=BenchmarkAccountBatchWrite -benchmem -cpuprofile=cpu.prof -benchtime=30s
	@echo ""
	@echo "CPU profile saved to $(GO_DIR)/cpu.prof"
	@echo "View with: go tool pprof -http=:8080 $(GO_DIR)/cpu.prof"

bench-go-mem-profile: build-ffi ## Run benchmarks with memory profiling
	@echo "Running benchmarks with memory profiling..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -bench=BenchmarkAccountBatchWrite -benchmem -memprofile=mem.prof -benchtime=30s
	@echo ""
	@echo "Memory profile saved to $(GO_DIR)/mem.prof"
	@echo "View with: go tool pprof -http=:8080 $(GO_DIR)/mem.prof"

test-go-unit: build-ffi ## Run Go unit tests
	@echo "Running Go unit tests..."
	cd $(GO_DIR) && \
		CGO_LDFLAGS="-L../$(TARGET_DIR) -ltriedb_ffi" \
		$(LIB_PATH_VAR)="../$(TARGET_DIR):$$$(LIB_PATH_VAR)" \
		go test -v -count=1

format-ffi: ## Format Rust code
	cd $(FFI_DIR) && cargo fmt

format-go: ## Format Go code
	cd $(GO_DIR) && go fmt ./...

format: format-ffi format-go ## Format all code

lint-ffi: ## Lint Rust code
	cd $(FFI_DIR) && cargo clippy -- -D warnings

lint-go: ## Lint Go code
	cd $(GO_DIR) && go vet ./...

lint: lint-ffi lint-go ## Lint all code

.PHONY: build-static
build-static: ## Build static library for embedding
	@echo "Building static library..."
	cd $(FFI_DIR) && cargo build --release
	@echo "Static library: $(TARGET_DIR)/$(STATIC_LIB)"

