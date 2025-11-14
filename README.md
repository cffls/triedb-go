# TrieDB Go Bindings via FFI

Complete guide for building and using TrieDB from Go through C FFI bindings.

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Building](#building)
- [API Reference](#api-reference)
- [Benchmarks](#benchmarks)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

### Run Benchmarks

```bash
# Quick benchmarks (1s each, ~30s total)
make bench-go-quick

# Full benchmarks (5s each, ~2min total)
make bench-go
```

### Key Features

- 🔗 **Static Linking**: Rust code embedded in Go binaries (no runtime dependencies)
- 🚀 **Fast**: ~2µs reads, ~750µs writes, 50x speedup with batching
- 🔒 **Safe**: Zero-copy FFI with proper memory management
- 📦 **Simple**: Just build Rust library once, then use like any Go package

---

## Architecture

The FFI implementation consists of three layers:

```
┌─────────────────────────────────┐
│     Go Application              │
│  (Your blockchain/EVM code)     │
└────────────┬────────────────────┘
             │
┌────────────▼────────────────────┐
│   Go Bindings (triedb.go)       │
│  - Type conversions             │
│  - Error mapping                │
│  - Memory management            │
└────────────┬────────────────────┘
             │ CGO
┌────────────▼────────────────────┐
│   Rust FFI (triedb-ffi/lib.rs) │
│  - C-compatible API             │
│  - Opaque pointers              │
│  - Safe memory handling         │
└────────────┬────────────────────┘
             │
┌────────────▼────────────────────┐
│   TrieDB Core (Rust)            │
│  - Page-based storage           │
│  - MVCC snapshots               │
│  - Merkle trie operations       │
└─────────────────────────────────┘
```

### Components

- **`triedb-ffi/`** - Rust FFI layer exposing C-compatible API
- **`triedb-go/`** - Go wrapper with idiomatic API
- **`triedb-go/example/`** - Working example application

---

## Building

### Prerequisites

- Rust toolchain (1.70+)
- Go toolchain (1.21+)
- C compiler (gcc/clang)
- `cbindgen` for header generation: `cargo install cbindgen`

### Build Commands

```bash
# Build everything
make build-ffi build-go

# Just the FFI library
make build-ffi

# Run example
make example

# Run tests
make test-go-unit
```

### Installation

The library uses **static linking** by default, which means the Rust code is compiled directly into your Go binary. No runtime dependencies or environment variables needed!

**Build Steps:**

```bash
# 1. Build the Rust FFI library (creates libtriedb_ffi.a)
cargo build --release

# 2. Build your Go application
cd triedb-go
go build
```

**What happens behind the scenes:**
- The static library (`libtriedb_ffi.a`) is linked into your Go binary
- No `.so` or `.dylib` files needed at runtime
- Self-contained, portable executables
- No `LD_LIBRARY_PATH` configuration required

**Current cgo configuration (in `triedb.go`):**
```go
// #cgo CFLAGS: -I${SRCDIR}/../triedb-ffi
// #cgo LDFLAGS: ${SRCDIR}/../target/release/libtriedb_ffi.a -ldl -lm -lpthread
// #include "triedb.h"
```

**For library users:**
```bash
go get github.com/yourname/triedb-go/triedb-go
# Build the Rust library first
cd $GOPATH/pkg/mod/github.com/yourname/triedb-go@version
cargo build --release
# Now use in your Go code
```

---

## API Reference

### Database Operations

```go
import triedb "github.com/base/triedb-go"

// Open existing database
db, err := triedb.Open("state.db")

// Create new database (fails if exists)
db, err := triedb.CreateNew("state.db")

// Get state root
root, err := db.StateRoot()  // Returns Hash (32 bytes)

// Get size in pages
size, err := db.Size()

// Close database
err = db.Close()
```

### Read-Only Transactions

```go
// Begin RO transaction (multiple concurrent readers allowed)
tx, err := db.BeginRO()

// Get account
account, err := tx.GetAccount(address)  // nil if not found

// Get storage slot
value, err := tx.GetStorage(address, slot)  // nil if not found

// Commit (releases snapshot)
err = tx.Commit()
```

### Read-Write Transactions

```go
// Begin RW transaction (single writer, blocks other writers)
tx, err := db.BeginRW()

// Set account
err = tx.SetAccount(address, account)

// Delete account
err = tx.SetAccount(address, nil)

// Set storage
err = tx.SetStorage(address, slot, &value)

// Delete storage
err = tx.SetStorage(address, slot, nil)

// Commit changes
err = tx.Commit()

// Or rollback
err = tx.Rollback()
```

### Types

```go
type Address [20]byte
type Hash [32]byte

type Account struct {
    Nonce       uint64
    Balance     *uint256.Int
    StorageRoot Hash
    CodeHash    []byte
}
```

### Helper Functions

```go
// Parse hex strings
addr, err := triedb.AddressFromHex("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
hash, err := triedb.HashFromHex("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

// Convert to hex
hexStr := addr.Hex()  // Returns "0x..."
hexStr := hash.Hex()
```

### Complete Example

```go
package main

import (
    "log"

    "github.com/holiman/uint256"
    triedb "github.com/base/triedb-go"
)

func main() {
    // Create database
    db, err := triedb.CreateNew("state.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Parse address
    addr, _ := triedb.AddressFromHex("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

    // Create account
    emptyRoot, _ := triedb.HashFromHex("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
    emptyCode, _ := triedb.HashFromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")

    account := &triedb.Account{
        Nonce:       1,
        Balance:     uint256.NewInt(1_000_000_000_000_000_000), // 1 ETH
        StorageRoot: emptyRoot,
        CodeHash:    emptyCode[:],
    }

    // Write transaction
    tx, _ := db.BeginRW()
    tx.SetAccount(addr, account)
    tx.Commit()

    // Read transaction
    roTx, _ := db.BeginRO()
    readAccount, _ := roTx.GetAccount(addr)
    roTx.Commit()

    log.Printf("Balance: %s wei", readAccount.Balance.String())
}
```

---

## Benchmarks

### Quick Benchmarks

```bash
# All benchmarks, 1s each (~30s total)
make bench-go-quick

# Full benchmarks, 5s each (~2min)
make bench-go

# Compare TrieDB vs in-memory map
make bench-go-compare

# Test concurrent scaling (1/2/4/8 cores)
make bench-go-concurrent
```

### Results on AMD Ryzen 7 5800H + NVMe SSD

```
BenchmarkAccountWrite/OpsPerTx_1-16           746087 ns/op    1.000 ops/tx    424 B/op   9 allocs/op
BenchmarkAccountWrite/OpsPerTx_16-16           83678 ns/op     16.00 ops/tx    409 B/op   7 allocs/op
BenchmarkAccountWrite/OpsPerTx_256-16          19456 ns/op     256.0 ops/tx    408 B/op   7 allocs/op
BenchmarkAccountWrite/OpsPerTx_4096-16         11701 ns/op    4096 ops/tx     408 B/op   7 allocs/op
BenchmarkAccountWrite/OpsPerTx_65536-16         6102 ns/op   65536 ops/tx     408 B/op   7 allocs/op
BenchmarkAccountRead/OpsPerTx_1-16              2186 ns/op    1.000 ops/tx    368 B/op  10 allocs/op
BenchmarkAccountRead/OpsPerTx_16-16             1941 ns/op     16.00 ops/tx    346 B/op   8 allocs/op
BenchmarkAccountRead/OpsPerTx_256-16            1916 ns/op     256.0 ops/tx    345 B/op   8 allocs/op
BenchmarkAccountRead/OpsPerTx_4096-16           1912 ns/op    4096 ops/tx     345 B/op   8 allocs/op
BenchmarkAccountRead/OpsPerTx_65536-16          1908 ns/op   65536 ops/tx     345 B/op   8 allocs/op
BenchmarkStorageWrite/OpsPerTx_1-16            760010 ns/op   1.000 ops/tx    104 B/op   5 allocs/op
BenchmarkStorageWrite/OpsPerTx_16-16            82904 ns/op     16.00 ops/tx     89 B/op   3 allocs/op
BenchmarkStorageWrite/OpsPerTx_256-16           19723 ns/op     256.0 ops/tx     88 B/op   3 allocs/op
BenchmarkStorageWrite/OpsPerTx_4096-16          10646 ns/op    4096 ops/tx     88 B/op   3 allocs/op
BenchmarkStorageWrite/OpsPerTx_65536-16          6676 ns/op   65536 ops/tx     88 B/op   3 allocs/op
BenchmarkStorageRead/OpsPerTx_1-16              2190 ns/op    1.000 ops/tx    176 B/op   8 allocs/op
BenchmarkStorageRead/OpsPerTx_16-16             1974 ns/op     16.00 ops/tx    154 B/op   6 allocs/op
BenchmarkStorageRead/OpsPerTx_256-16            1945 ns/op     256.0 ops/tx    153 B/op   6 allocs/op
BenchmarkStorageRead/OpsPerTx_4096-16           1955 ns/op    4096 ops/tx     153 B/op   6 allocs/op
BenchmarkStorageRead/OpsPerTx_65536-16          1957 ns/op   65536 ops/tx     153 B/op   6 allocs/op
BenchmarkConcurrentReads-16                      315.4 ns/op                  346 B/op   8 allocs/op
BenchmarkStateRootComputation-16                 135.1 ns/op                   64 B/op   2 allocs/op
BenchmarkMixedWorkload-16                        8942 ns/op                    365 B/op   8 allocs/op
```

### Batch Operations

| Ops per Tx | Per Account | Speedup vs Single |
| ---------- | ----------- | ----------------- |
| 1          | 746 µs      | 1x                |
| 16         | 84 µs       | 8.9x              |
| 256        | 19 µs       | 38x               |
| 4096       | 12 µs       | 63x               |
| 65536      | 6 µs        | 122x              |

**Key Insight:** Batching provides massive speedup by amortizing transaction overhead.


### Profiling

```bash
# CPU profiling
make bench-go-profile
cd triedb-go && go tool pprof -http=:8080 cpu.prof

# Memory profiling
make bench-go-mem-profile
cd triedb-go && go tool pprof -http=:8080 mem.prof
```

### Performance Targets

| Operation     | Excellent | Good    | Acceptable |
| ------------- | --------- | ------- | ---------- |
| Account Write | < 20 µs   | < 50 µs | < 100 µs   |
| Account Read  | < 10 µs   | < 20 µs | < 50 µs    |
| Storage Write | < 40 µs   | < 80 µs | < 150 µs   |
| Storage Read  | < 15 µs   | < 30 µs | < 80 µs    |
| State Root    | < 5 µs    | < 10 µs | < 50 µs    |

### Optimization Tips

**1. Batch Writes**

❌ Bad (752 µs per account):
```go
for _, account := range accounts {
    tx, _ := db.BeginRW()
    tx.SetAccount(account.Address, account.Data)
    tx.Commit()
}
```

✅ Good (15 µs per account):
```go
tx, _ := db.BeginRW()
for _, account := range accounts {
    tx.SetAccount(account.Address, account.Data)
}
tx.Commit()
```

**2. Reuse Read Transactions**

❌ Bad:
```go
for _, addr := range addresses {
    tx, _ := db.BeginRO()
    account, _ := tx.GetAccount(addr)
    tx.Commit()
}
```

✅ Good:
```go
tx, _ := db.BeginRO()
for _, addr := range addresses {
    account, _ := tx.GetAccount(addr)
}
tx.Commit()
```

**3. Concurrent Reads**

```go
var wg sync.WaitGroup
for i := 0; i < runtime.NumCPU(); i++ {
    wg.Add(1)
    go func(batch []Address) {
        defer wg.Done()
        tx, _ := db.BeginRO()
        defer tx.Commit()
        for _, addr := range batch {
            tx.GetAccount(addr)
        }
    }(addressBatches[i])
}
wg.Wait()
```

### Continuous Benchmarking

```bash
# Save baseline
make bench-go > baseline.txt

# After changes
make bench-go > new.txt

# Compare
benchstat baseline.txt new.txt
```

Install `benchstat`:
```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

---

## Makefile Commands

### Building
```bash
make build-ffi          # Build Rust library
make build-go           # Build Go bindings
make install            # Install system-wide
make clean              # Clean artifacts
```

### Testing
```bash
make test-go-unit       # Run unit tests
make example            # Run example
```

### Benchmarking
```bash
make bench-go-quick           # Quick (1s each)
make bench-go                 # Full (5s each)
make bench-go-compare         # vs in-memory
make bench-go-concurrent      # Scalability
make bench-go-profile         # CPU profile
make bench-go-mem-profile     # Memory profile
```

### Code Quality
```bash
make format             # Format all code
make lint               # Lint all code
```

---

## Concurrency Model

- **Multiple concurrent readers**: Safe across goroutines
- **Single writer**: Only one RW transaction at a time (blocks)
- **Snapshot isolation**: Readers see consistent state from transaction start
- **MVCC**: Readers never block writers, writers never block readers

```go
// Multiple readers work concurrently
go func() {
    tx, _ := db.BeginRO()
    defer tx.Commit()
    // Read operations...
}()

go func() {
    tx, _ := db.BeginRO()
    defer tx.Commit()
    // Read operations...
}()

// Only one writer at a time
tx, _ := db.BeginRW()
tx.SetAccount(addr, account)
tx.Commit()
```
