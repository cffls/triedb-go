package triedb

import (
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/holiman/uint256"
)

// Helper functions for tests
func randomAddress() Address {
	var addr Address
	rand.Read(addr[:])
	return addr
}

func randomHash() Hash {
	var hash Hash
	rand.Read(hash[:])
	return hash
}

func emptyStorageRoot() Hash {
	h, _ := HashFromHex("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
	return h
}

func emptyCodeHash() []byte {
	h, _ := HashFromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	return h[:]
}

func createTestAccount(nonce uint64) *Account {
	balance := new(uint256.Int).SetUint64(nonce)
	weiMultiplier := new(uint256.Int).SetUint64(1_000_000_000_000_000_000)
	balance.Mul(balance, weiMultiplier)

	return &Account{
		Nonce:       nonce,
		Balance:     balance,
		StorageRoot: emptyStorageRoot(),
		CodeHash:    emptyCodeHash(),
	}
}

// Basic functionality tests
func TestDatabaseOpenClose(t *testing.T) {
	tmpFile := "test_open_close.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Reopen
	db, err = Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open existing database: %v", err)
	}
	defer db.Close()
}

func TestAccountSetGet(t *testing.T) {
	tmpFile := "test_account.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	addr := randomAddress()
	account := createTestAccount(42)

	// Write
	tx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}

	if err := tx.SetAccount(addr, account); err != nil {
		t.Fatalf("Failed to set account: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Read
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}

	readAccount, err := roTx.GetAccount(addr)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}

	if readAccount == nil {
		t.Fatal("Account not found")
	}

	if readAccount.Nonce != account.Nonce {
		t.Errorf("Nonce mismatch: got %d, want %d", readAccount.Nonce, account.Nonce)
	}

	if readAccount.Balance.Cmp(account.Balance) != 0 {
		t.Errorf("Balance mismatch: got %s, want %s", readAccount.Balance, account.Balance)
	}

	if err := roTx.Commit(); err != nil {
		t.Fatalf("Failed to commit RO transaction: %v", err)
	}
}

func TestStorageSetGet(t *testing.T) {
	tmpFile := "test_storage.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	addr := randomAddress()
	account := createTestAccount(1)

	// Set account first
	tx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	tx.SetAccount(addr, account)

	// Set storage
	slot := randomHash()
	value := randomHash()
	if err := tx.SetStorage(addr, slot, &value); err != nil {
		t.Fatalf("Failed to set storage: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Read storage
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}

	readValue, err := roTx.GetStorage(addr, slot)
	if err != nil {
		t.Fatalf("Failed to get storage: %v", err)
	}

	if readValue == nil {
		t.Fatal("Storage value not found")
	}

	if *readValue != value {
		t.Errorf("Value mismatch: got %s, want %s", readValue.Hex(), value.Hex())
	}

	if err := roTx.Commit(); err != nil {
		t.Fatalf("Failed to commit RO transaction: %v", err)
	}
}

func TestAccountDelete(t *testing.T) {
	tmpFile := "test_delete.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	addr := randomAddress()
	account := createTestAccount(1)

	// Set account
	tx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	tx.SetAccount(addr, account)
	tx.Commit()

	// Delete account
	tx, err = db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	if err := tx.SetAccount(addr, nil); err != nil {
		t.Fatalf("Failed to delete account: %v", err)
	}
	tx.Commit()

	// Verify deleted
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}

	readAccount, err := roTx.GetAccount(addr)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}

	if readAccount != nil {
		t.Error("Account should be deleted")
	}

	roTx.Commit()
}

func TestTransactionRollback(t *testing.T) {
	tmpFile := "test_rollback.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	addr := randomAddress()
	account := createTestAccount(1)

	// Set account but rollback
	tx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	tx.SetAccount(addr, account)
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// Verify not committed
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}

	readAccount, err := roTx.GetAccount(addr)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}

	if readAccount != nil {
		t.Error("Account should not exist after rollback")
	}

	roTx.Commit()
}

func TestStateRoot(t *testing.T) {
	tmpFile := "test_root.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	root1, _ := db.StateRoot()

	// Add account
	addr := randomAddress()
	tx, _ := db.BeginRW()
	tx.SetAccount(addr, createTestAccount(1))
	tx.Commit()

	root2, _ := db.StateRoot()

	if root1 == root2 {
		t.Error("State root should change after modification")
	}
}

// Benchmarks
func BenchmarkAccountWrite(b *testing.B) {
	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_write_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			accounts := make([]Address, b.N)
			for i := range accounts {
				accounts[i] = randomAddress()
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				tx      *TransactionRW
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					tx, err = db.BeginRW()
					if err != nil {
						b.Fatalf("Failed to begin RW transaction: %v", err)
					}
				}

				if err := tx.SetAccount(accounts[i], createTestAccount(uint64(i))); err != nil {
					b.Fatalf("Failed to set account: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := tx.Commit(); err != nil {
						b.Fatalf("Failed to commit transaction: %v", err)
					}
					tx = nil
					pending = 0
				}
			}

			if tx != nil && pending > 0 {
				if err := tx.Commit(); err != nil {
					b.Fatalf("Failed to commit final transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}
}

func BenchmarkAccountRead(b *testing.B) {
	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_read_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			const numAccounts = 10000
			addresses := make([]Address, numAccounts)
			tx, err := db.BeginRW()
			if err != nil {
				b.Fatalf("Failed to begin RW transaction: %v", err)
			}
			for i := 0; i < numAccounts; i++ {
				addresses[i] = randomAddress()
				if err := tx.SetAccount(addresses[i], createTestAccount(uint64(i))); err != nil {
					b.Fatalf("Failed to pre-populate account: %v", err)
				}
			}
			if err := tx.Commit(); err != nil {
				b.Fatalf("Failed to commit pre-population: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				roTx    *TransactionRO
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					roTx, err = db.BeginRO()
					if err != nil {
						b.Fatalf("Failed to begin RO transaction: %v", err)
					}
				}

				if _, err := roTx.GetAccount(addresses[i%numAccounts]); err != nil {
					b.Fatalf("Failed to get account: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := roTx.Commit(); err != nil {
						b.Fatalf("Failed to commit RO transaction: %v", err)
					}
					roTx = nil
					pending = 0
				}
			}

			if roTx != nil {
				if err := roTx.Commit(); err != nil {
					b.Fatalf("Failed to commit final RO transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}
}

func BenchmarkAccountBatchWrite(b *testing.B) {
	tmpFile := "bench_batch.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	batchSizes := []int{10, 100, 1000}
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				tx, _ := db.BeginRW()
				for j := 0; j < batchSize; j++ {
					addr := randomAddress()
					tx.SetAccount(addr, createTestAccount(uint64(j)))
				}
				tx.Commit()
			}

			b.ReportMetric(float64(batchSize), "accounts/op")
		})
	}
}

func BenchmarkStorageWrite(b *testing.B) {
	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_storage_write_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			addr := randomAddress()
			tx, err := db.BeginRW()
			if err != nil {
				b.Fatalf("Failed to begin RW transaction: %v", err)
			}
			if err := tx.SetAccount(addr, createTestAccount(1)); err != nil {
				b.Fatalf("Failed to create account: %v", err)
			}
			if err := tx.Commit(); err != nil {
				b.Fatalf("Failed to commit account creation: %v", err)
			}

			slots := make([]Hash, b.N)
			values := make([]Hash, b.N)
			for i := range slots {
				slots[i] = randomHash()
				values[i] = randomHash()
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				writeTx *TransactionRW
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					writeTx, err = db.BeginRW()
					if err != nil {
						b.Fatalf("Failed to begin RW transaction: %v", err)
					}
				}

				if err := writeTx.SetStorage(addr, slots[i], &values[i]); err != nil {
					b.Fatalf("Failed to set storage: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := writeTx.Commit(); err != nil {
						b.Fatalf("Failed to commit storage transaction: %v", err)
					}
					writeTx = nil
					pending = 0
				}
			}

			if writeTx != nil {
				if err := writeTx.Commit(); err != nil {
					b.Fatalf("Failed to commit final storage transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}
}

func BenchmarkStorageRead(b *testing.B) {
	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_storage_read_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			addr := randomAddress()
			const numSlots = 1000
			slots := make([]Hash, numSlots)

			tx, err := db.BeginRW()
			if err != nil {
				b.Fatalf("Failed to begin RW transaction: %v", err)
			}
			if err := tx.SetAccount(addr, createTestAccount(1)); err != nil {
				b.Fatalf("Failed to create account: %v", err)
			}
			for i := 0; i < numSlots; i++ {
				slots[i] = randomHash()
				value := randomHash()
				if err := tx.SetStorage(addr, slots[i], &value); err != nil {
					b.Fatalf("Failed to populate storage: %v", err)
				}
			}
			if err := tx.Commit(); err != nil {
				b.Fatalf("Failed to commit population transaction: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				roTx    *TransactionRO
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					roTx, err = db.BeginRO()
					if err != nil {
						b.Fatalf("Failed to begin RO transaction: %v", err)
					}
				}

				if _, err := roTx.GetStorage(addr, slots[i%numSlots]); err != nil {
					b.Fatalf("Failed to get storage: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := roTx.Commit(); err != nil {
						b.Fatalf("Failed to commit RO transaction: %v", err)
					}
					roTx = nil
					pending = 0
				}
			}

			if roTx != nil {
				if err := roTx.Commit(); err != nil {
					b.Fatalf("Failed to commit final RO transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}
}

func BenchmarkConcurrentReads(b *testing.B) {
	tmpFile := "bench_concurrent.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Populate database
	const numAccounts = 10000
	addresses := make([]Address, numAccounts)
	tx, _ := db.BeginRW()
	for i := 0; i < numAccounts; i++ {
		addresses[i] = randomAddress()
		tx.SetAccount(addresses[i], createTestAccount(uint64(i)))
	}
	tx.Commit()

	const opsPerTx = 256

	b.RunParallel(func(pb *testing.PB) {
		var (
			i     int
			roTx  *TransactionRO
			opCnt int
		)

		for pb.Next() {
			if opCnt == 0 {
				var err error
				roTx, err = db.BeginRO()
				if err != nil {
					panic(fmt.Sprintf("failed to begin RO transaction: %v", err))
				}
			}

			if _, err := roTx.GetAccount(addresses[i%numAccounts]); err != nil {
				panic(fmt.Sprintf("failed to get account: %v", err))
			}

			i++
			opCnt++
			if opCnt == opsPerTx {
				if err := roTx.Commit(); err != nil {
					panic(fmt.Sprintf("failed to commit RO transaction: %v", err))
				}
				roTx = nil
				opCnt = 0
			}
		}

		if roTx != nil {
			if err := roTx.Commit(); err != nil {
				panic(fmt.Sprintf("failed to commit final RO transaction: %v", err))
			}
		}
	})
}

func BenchmarkStateRootComputation(b *testing.B) {
	tmpFile := "bench_root.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Add some accounts
	tx, _ := db.BeginRW()
	for i := 0; i < 100; i++ {
		tx.SetAccount(randomAddress(), createTestAccount(uint64(i)))
	}
	tx.Commit()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		db.StateRoot()
	}
}

func BenchmarkMixedWorkload(b *testing.B) {
	tmpFile := "bench_mixed.db"
	defer os.RemoveAll(tmpFile)
	defer os.RemoveAll(tmpFile + ".meta")

	db, err := CreateNew(tmpFile)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Pre-populate
	addresses := make([]Address, 1000)
	tx, _ := db.BeginRW()
	for i := range addresses {
		addresses[i] = randomAddress()
		tx.SetAccount(addresses[i], createTestAccount(uint64(i)))
	}
	tx.Commit()

	const (
		writeOpsPerTx = 64
		readOpsPerTx  = 256
	)

	var (
		writeTx      *TransactionRW
		readTx       *TransactionRO
		pendingWrite int
		pendingRead  int
	)

	for i := 0; i < b.N; i++ {
		if i%5 == 0 {
			if pendingWrite == 0 {
				writeTx, err = db.BeginRW()
				if err != nil {
					b.Fatalf("Failed to begin RW transaction: %v", err)
				}
			}

			if err := writeTx.SetAccount(randomAddress(), createTestAccount(uint64(i))); err != nil {
				b.Fatalf("Failed to set account in mixed workload: %v", err)
			}

			pendingWrite++
			if pendingWrite == writeOpsPerTx {
				if err := writeTx.Commit(); err != nil {
					b.Fatalf("Failed to commit write transaction: %v", err)
				}
				writeTx = nil
				pendingWrite = 0
			}
		} else {
			if pendingRead == 0 {
				readTx, err = db.BeginRO()
				if err != nil {
					b.Fatalf("Failed to begin RO transaction: %v", err)
				}
			}

			if _, err := readTx.GetAccount(addresses[i%len(addresses)]); err != nil {
				b.Fatalf("Failed to get account in mixed workload: %v", err)
			}

			pendingRead++
			if pendingRead == readOpsPerTx {
				if err := readTx.Commit(); err != nil {
					b.Fatalf("Failed to commit read transaction: %v", err)
				}
				readTx = nil
				pendingRead = 0
			}
		}
	}

	if writeTx != nil {
		if err := writeTx.Commit(); err != nil {
			b.Fatalf("Failed to commit final write transaction: %v", err)
		}
	}

	if readTx != nil {
		if err := readTx.Commit(); err != nil {
			b.Fatalf("Failed to commit final read transaction: %v", err)
		}
	}
}

// Comparison benchmark with in-memory map
type InMemoryDB struct {
	accounts map[Address]*Account
	storage  map[Address]map[Hash]Hash
}

func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		accounts: make(map[Address]*Account),
		storage:  make(map[Address]map[Hash]Hash),
	}
}

func (db *InMemoryDB) SetAccount(addr Address, acc *Account) {
	if acc == nil {
		delete(db.accounts, addr)
	} else {
		db.accounts[addr] = acc
	}
}

func (db *InMemoryDB) GetAccount(addr Address) *Account {
	return db.accounts[addr]
}

func (db *InMemoryDB) SetStorage(addr Address, slot Hash, value *Hash) {
	if db.storage[addr] == nil {
		db.storage[addr] = make(map[Hash]Hash)
	}
	if value == nil {
		delete(db.storage[addr], slot)
	} else {
		db.storage[addr][slot] = *value
	}
}

func (db *InMemoryDB) GetStorage(addr Address, slot Hash) *Hash {
	if storage, ok := db.storage[addr]; ok {
		if val, ok := storage[slot]; ok {
			return &val
		}
	}
	return nil
}

func BenchmarkCompareInMemoryWrite(b *testing.B) {
	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("TrieDB_OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_compare_triedb_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			addresses := make([]Address, b.N)
			for i := range addresses {
				addresses[i] = randomAddress()
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				tx      *TransactionRW
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					tx, err = db.BeginRW()
					if err != nil {
						b.Fatalf("Failed to begin RW transaction: %v", err)
					}
				}

				if err := tx.SetAccount(addresses[i], createTestAccount(uint64(i))); err != nil {
					b.Fatalf("Failed to set account: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := tx.Commit(); err != nil {
						b.Fatalf("Failed to commit transaction: %v", err)
					}
					tx = nil
					pending = 0
				}
			}

			if tx != nil {
				if err := tx.Commit(); err != nil {
					b.Fatalf("Failed to commit final transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}

	b.Run("InMemoryMap", func(b *testing.B) {
		db := NewInMemoryDB()

		addresses := make([]Address, b.N)
		for i := range addresses {
			addresses[i] = randomAddress()
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			db.SetAccount(addresses[i], createTestAccount(uint64(i)))
		}
	})
}

func BenchmarkCompareInMemoryRead(b *testing.B) {
	const numAccounts = 10000

	opsPerTxCases := []int{1, 16, 256, 4096, 65536}

	for _, opsPerTx := range opsPerTxCases {
		b.Run(fmt.Sprintf("TrieDB_OpsPerTx_%d", opsPerTx), func(b *testing.B) {
			tmpFile := fmt.Sprintf("bench_compare_triedb_read_ops_%d.db", opsPerTx)
			defer os.RemoveAll(tmpFile)
			defer os.RemoveAll(tmpFile + ".meta")

			db, err := CreateNew(tmpFile)
			if err != nil {
				b.Fatalf("Failed to create database: %v", err)
			}
			defer db.Close()

			addresses := make([]Address, numAccounts)
			tx, err := db.BeginRW()
			if err != nil {
				b.Fatalf("Failed to begin RW transaction: %v", err)
			}
			for i := 0; i < numAccounts; i++ {
				addresses[i] = randomAddress()
				if err := tx.SetAccount(addresses[i], createTestAccount(uint64(i))); err != nil {
					b.Fatalf("Failed to seed account: %v", err)
				}
			}
			if err := tx.Commit(); err != nil {
				b.Fatalf("Failed to commit seed transaction: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			var (
				roTx    *TransactionRO
				pending int
			)

			for i := 0; i < b.N; i++ {
				if pending == 0 {
					roTx, err = db.BeginRO()
					if err != nil {
						b.Fatalf("Failed to begin RO transaction: %v", err)
					}
				}

				if _, err := roTx.GetAccount(addresses[i%numAccounts]); err != nil {
					b.Fatalf("Failed to get account: %v", err)
				}

				pending++
				if pending == opsPerTx {
					if err := roTx.Commit(); err != nil {
						b.Fatalf("Failed to commit RO transaction: %v", err)
					}
					roTx = nil
					pending = 0
				}
			}

			if roTx != nil {
				if err := roTx.Commit(); err != nil {
					b.Fatalf("Failed to commit final RO transaction: %v", err)
				}
			}

			b.ReportMetric(float64(opsPerTx), "ops/tx")
		})
	}

	b.Run("InMemoryMap", func(b *testing.B) {
		db := NewInMemoryDB()

		addresses := make([]Address, numAccounts)
		for i := 0; i < numAccounts; i++ {
			addresses[i] = randomAddress()
			db.SetAccount(addresses[i], createTestAccount(uint64(i)))
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			db.GetAccount(addresses[i%numAccounts])
		}
	})
}

// ============================================================================
// Overlay State Tests
// ============================================================================

func TestOverlayStateMut_Basic(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	// Check initial state
	isEmpty, err := overlay.IsEmpty()
	if err != nil {
		t.Fatalf("Failed to check if empty: %v", err)
	}
	if !isEmpty {
		t.Error("Expected overlay to be empty initially")
	}

	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected length 0, got %d", length)
	}

	// Insert an account
	addr := randomAddress()
	account := createTestAccount(1)
	err = overlay.InsertAccount(addr, account)
	if err != nil {
		t.Fatalf("Failed to insert account: %v", err)
	}

	// Check state after insertion
	isEmpty, err = overlay.IsEmpty()
	if err != nil {
		t.Fatalf("Failed to check if empty: %v", err)
	}
	if isEmpty {
		t.Error("Expected overlay to not be empty after insertion")
	}

	length, err = overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected length 1, got %d", length)
	}
}

func TestOverlayStateMut_WithCapacity(t *testing.T) {
	overlay, err := NewOverlayStateWithCapacity(100)
	if err != nil {
		t.Fatalf("Failed to create overlay with capacity: %v", err)
	}
	defer overlay.Close()

	isEmpty, err := overlay.IsEmpty()
	if err != nil {
		t.Fatalf("Failed to check if empty: %v", err)
	}
	if !isEmpty {
		t.Error("Expected overlay to be empty initially")
	}
}

func TestOverlayStateMut_InsertMultipleAccounts(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	numAccounts := 10
	for i := 0; i < numAccounts; i++ {
		addr := randomAddress()
		account := createTestAccount(uint64(i))
		err = overlay.InsertAccount(addr, account)
		if err != nil {
			t.Fatalf("Failed to insert account %d: %v", i, err)
		}
	}

	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != numAccounts {
		t.Errorf("Expected length %d, got %d", numAccounts, length)
	}
}

func TestOverlayStateMut_InsertStorage(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	addr := randomAddress()
	slot := randomHash()
	value := uint256.NewInt(12345)

	err = overlay.InsertStorage(addr, slot, value)
	if err != nil {
		t.Fatalf("Failed to insert storage: %v", err)
	}

	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 1 {
		t.Errorf("Expected length 1, got %d", length)
	}
}

func TestOverlayStateMut_InsertTombstones(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	addr := randomAddress()
	slot := randomHash()

	// Insert account tombstone
	err = overlay.InsertAccount(addr, nil)
	if err != nil {
		t.Fatalf("Failed to insert account tombstone: %v", err)
	}

	// Insert storage tombstone
	err = overlay.InsertStorage(addr, slot, nil)
	if err != nil {
		t.Fatalf("Failed to insert storage tombstone: %v", err)
	}

	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 2 {
		t.Errorf("Expected length 2, got %d", length)
	}
}

func TestOverlayStateMut_Freeze(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	// Insert some changes
	for i := 0; i < 5; i++ {
		addr := randomAddress()
		account := createTestAccount(uint64(i))
		err = overlay.InsertAccount(addr, account)
		if err != nil {
			t.Fatalf("Failed to insert account %d: %v", i, err)
		}
	}

	// Freeze the overlay
	err = overlay.Freeze()
	if err != nil {
		t.Fatalf("Failed to freeze overlay: %v", err)
	}

	// Check that the mutable pointer is consumed
	if overlay.mutPtr != nil {
		t.Error("Expected mutPtr to be nil after freeze")
	}
	if overlay.frozenPtr == nil {
		t.Error("Expected frozenPtr to be set after freeze")
	}

	// Check frozen overlay state
	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get frozen length: %v", err)
	}
	if length != 5 {
		t.Errorf("Expected frozen length 5, got %d", length)
	}

	isEmpty, err := overlay.IsEmpty()
	if err != nil {
		t.Fatalf("Failed to check if frozen is empty: %v", err)
	}
	if isEmpty {
		t.Error("Expected frozen overlay to not be empty")
	}
}

func TestOverlayState_Empty(t *testing.T) {
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	err = overlay.Freeze()
	if err != nil {
		t.Fatalf("Failed to freeze overlay: %v", err)
	}

	isEmpty, err := overlay.IsEmpty()
	if err != nil {
		t.Fatalf("Failed to check if empty: %v", err)
	}
	if !isEmpty {
		t.Error("Expected frozen overlay to be empty")
	}

	length, err := overlay.Len()
	if err != nil {
		t.Fatalf("Failed to get length: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected length 0, got %d", length)
	}
}

func TestComputeRootWithOverlay_EmptyOverlay(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := CreateNew(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get initial state root
	initialRoot, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get initial state root: %v", err)
	}

	// Create empty overlay
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	// Begin read-only transaction
	tx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}
	defer tx.Commit()

	// Compute root with empty overlay (will auto-freeze)
	result, err := tx.ComputeRootWithOverlay(overlay)
	if err != nil {
		t.Fatalf("Failed to compute root with overlay: %v", err)
	}
	defer result.Close()

	newRoot, err := result.Root()
	if err != nil {
		t.Fatalf("Failed to get root: %v", err)
	}

	// Empty overlay should not change the root
	if initialRoot != newRoot {
		t.Errorf("Expected root %x, got %x", initialRoot, newRoot)
	}
}

func TestComputeRootWithOverlay_WithChanges(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := CreateNew(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add initial account to database
	addr1 := randomAddress()
	account1 := createTestAccount(1)

	tx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	err = tx.SetAccount(addr1, account1)
	if err != nil {
		t.Fatalf("Failed to set account: %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Get root after initial commit
	rootAfterCommit, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get state root: %v", err)
	}

	// Create overlay with new account
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	addr2 := randomAddress()
	account2 := createTestAccount(2)
	err = overlay.InsertAccount(addr2, account2)
	if err != nil {
		t.Fatalf("Failed to insert account: %v", err)
	}

	// Begin read-only transaction
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}
	defer roTx.Commit()

	// Compute root with overlay (will auto-freeze)
	result, err := roTx.ComputeRootWithOverlay(overlay)
	if err != nil {
		t.Fatalf("Failed to compute root with overlay: %v", err)
	}
	defer result.Close()

	overlayRoot, err := result.Root()
	if err != nil {
		t.Fatalf("Failed to get overlay root: %v", err)
	}

	// Overlay root should be different from current root
	if rootAfterCommit == overlayRoot {
		t.Error("Expected overlay root to be different from committed root")
	}

	// Now actually commit the same change and verify roots match
	rwTx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	err = rwTx.SetAccount(addr2, account2)
	if err != nil {
		t.Fatalf("Failed to set account: %v", err)
	}
	err = rwTx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	actualRoot, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get actual root: %v", err)
	}

	// The overlay root should match the actual committed root
	if overlayRoot != actualRoot {
		t.Errorf("Expected overlay root %x to match actual root %x", overlayRoot, actualRoot)
	}
}

func TestComputeRootWithOverlay_MultipleChanges(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := CreateNew(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create overlay with multiple accounts
	overlay, err := NewOverlayStateWithCapacity(10)
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	numAccounts := 5
	addresses := make([]Address, numAccounts)
	accounts := make([]*Account, numAccounts)

	for i := 0; i < numAccounts; i++ {
		addresses[i] = randomAddress()
		accounts[i] = createTestAccount(uint64(i + 1))
		err = overlay.InsertAccount(addresses[i], accounts[i])
		if err != nil {
			t.Fatalf("Failed to insert account %d: %v", i, err)
		}
	}

	// Compute overlay root
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}
	defer roTx.Commit()

	result, err := roTx.ComputeRootWithOverlay(overlay)
	if err != nil {
		t.Fatalf("Failed to compute root with overlay: %v", err)
	}
	defer result.Close()

	overlayRoot, err := result.Root()
	if err != nil {
		t.Fatalf("Failed to get overlay root: %v", err)
	}

	// Commit the same changes
	rwTx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	for i := 0; i < numAccounts; i++ {
		err = rwTx.SetAccount(addresses[i], accounts[i])
		if err != nil {
			t.Fatalf("Failed to set account %d: %v", i, err)
		}
	}
	err = rwTx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	actualRoot, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get actual root: %v", err)
	}

	// Verify roots match
	if overlayRoot != actualRoot {
		t.Errorf("Expected overlay root %x to match actual root %x", overlayRoot, actualRoot)
	}
}

func TestComputeRootWithOverlay_WithStorage(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := CreateNew(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	addr := randomAddress()
	account := createTestAccount(1)

	// Create account with storage
	rwTx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	err = rwTx.SetAccount(addr, account)
	if err != nil {
		t.Fatalf("Failed to set account: %v", err)
	}

	slot1 := randomHash()
	value1Uint := uint256.NewInt(100)
	value1Bytes := value1Uint.Bytes32()
	value1 := Hash(value1Bytes)
	err = rwTx.SetStorage(addr, slot1, &value1)
	if err != nil {
		t.Fatalf("Failed to set storage: %v", err)
	}

	err = rwTx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create overlay with new storage slot
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	slot2 := randomHash()
	value2Uint := uint256.NewInt(200)
	err = overlay.InsertStorage(addr, slot2, value2Uint)
	if err != nil {
		t.Fatalf("Failed to insert storage: %v", err)
	}

	// Compute overlay root
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}
	defer roTx.Commit()

	result, err := roTx.ComputeRootWithOverlay(overlay)
	if err != nil {
		t.Fatalf("Failed to compute root with overlay: %v", err)
	}
	defer result.Close()

	overlayRoot, err := result.Root()
	if err != nil {
		t.Fatalf("Failed to get overlay root: %v", err)
	}

	// Commit the storage change
	rwTx2, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	value2Bytes := value2Uint.Bytes32()
	value2Hash := Hash(value2Bytes)
	err = rwTx2.SetStorage(addr, slot2, &value2Hash)
	if err != nil {
		t.Fatalf("Failed to set storage: %v", err)
	}
	err = rwTx2.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	actualRoot, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get actual root: %v", err)
	}

	// Verify roots match
	if overlayRoot != actualRoot {
		t.Errorf("Expected overlay root %x to match actual root %x", overlayRoot, actualRoot)
	}
}

func TestComputeRootWithOverlay_Deletions(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := CreateNew(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add initial accounts
	addr1 := randomAddress()
	addr2 := randomAddress()
	account1 := createTestAccount(1)
	account2 := createTestAccount(2)

	rwTx, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	err = rwTx.SetAccount(addr1, account1)
	if err != nil {
		t.Fatalf("Failed to set account1: %v", err)
	}
	err = rwTx.SetAccount(addr2, account2)
	if err != nil {
		t.Fatalf("Failed to set account2: %v", err)
	}
	err = rwTx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create overlay with deletion (tombstone)
	overlay, err := NewOverlayState()
	if err != nil {
		t.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Close()

	err = overlay.InsertAccount(addr1, nil) // Delete addr1
	if err != nil {
		t.Fatalf("Failed to insert tombstone: %v", err)
	}

	// Compute overlay root
	roTx, err := db.BeginRO()
	if err != nil {
		t.Fatalf("Failed to begin RO transaction: %v", err)
	}
	defer roTx.Commit()

	result, err := roTx.ComputeRootWithOverlay(overlay)
	if err != nil {
		t.Fatalf("Failed to compute root with overlay: %v", err)
	}
	defer result.Close()

	overlayRoot, err := result.Root()
	if err != nil {
		t.Fatalf("Failed to get overlay root: %v", err)
	}

	// Actually delete the account
	rwTx2, err := db.BeginRW()
	if err != nil {
		t.Fatalf("Failed to begin RW transaction: %v", err)
	}
	err = rwTx2.SetAccount(addr1, nil)
	if err != nil {
		t.Fatalf("Failed to delete account: %v", err)
	}
	err = rwTx2.Commit()
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	actualRoot, err := db.StateRoot()
	if err != nil {
		t.Fatalf("Failed to get actual root: %v", err)
	}

	// Verify roots match
	if overlayRoot != actualRoot {
		t.Errorf("Expected overlay root %x to match actual root %x", overlayRoot, actualRoot)
	}
}
