# TrieDB Go Bindings

Go bindings for TrieDB - a high-performance embedded database for Ethereum state trie.

## Features

- 🔗 **Static linking** - No runtime dependencies, self-contained binaries
- 🚀 **High performance** - ~2µs reads, ~750µs writes, 50x faster with batching  
- 🔒 **Memory safe** - Rust-powered with safe FFI layer
- 📦 **Easy to use** - Idiomatic Go API

## Quick Start

```bash
# 1. Build the Rust FFI library
cargo build --release

# 2. Run tests
cd triedb-go
go test -v
```

## Documentation

**See [../README.md](../README.md) for complete documentation including:**

- Building and installation
- Complete API reference
- Benchmark results and optimization tips
- Troubleshooting guide
- Cross-compilation

## Quick Example

```go
package main

import (
    "log"

    "github.com/holiman/uint256"
    triedb "github.com/base/triedb-go"
)

func main() {
    db, _ := triedb.CreateNew("state.db")
    defer db.Close()

    addr, _ := triedb.AddressFromHex("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
    emptyRoot, _ := triedb.HashFromHex("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
    emptyCode, _ := triedb.HashFromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")

    account := &triedb.Account{
        Nonce:       1,
        Balance:     uint256.NewInt(1_000_000_000_000_000_000),
        StorageRoot: emptyRoot,
        CodeHash:    emptyCode,
    }

    tx, _ := db.BeginRW()
    tx.SetAccount(addr, account)
    tx.Commit()

    log.Printf("State root: %s", db.StateRoot())
}
```

## Benchmarks

```bash
# From this directory
go test -bench=. -benchtime=1s
```

**Results on AMD Ryzen 7 5800H (Static Linking):**
- Account Write: 752 µs
- Account Read: 2.3 µs  
- Storage Write: 760 µs
- Storage Read: 2.2 µs
- Batch (100): 28 µs per account (27x faster!)
- Concurrent: 429 ns with 16 cores

Static linking adds no performance overhead compared to dynamic linking.

## Installation

```go
import triedb "github.com/base/triedb-go"
```

**Requirements:**
- Rust toolchain (to build the FFI library once)
- Go 1.21+
- CGO enabled (default)

**Build the FFI library:**
```bash
# From repository root
cargo build --release
```

After that, use it like any Go package - the Rust code is statically linked into your binary.

## API

### Database
```go
db, err := triedb.Open("state.db")
db, err := triedb.CreateNew("state.db")
root, err := db.StateRoot()
err = db.Close()
```

### Transactions
```go
// Read-only
tx, _ := db.BeginRO()
account, _ := tx.GetAccount(address)
tx.Commit()

// Read-write
tx, _ := db.BeginRW()
tx.SetAccount(address, account)
tx.Commit()
```

