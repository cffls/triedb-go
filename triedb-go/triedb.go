package triedb

// #cgo CFLAGS: -I${SRCDIR}/../triedb-ffi
// #cgo LDFLAGS: ${SRCDIR}/../target/release/libtriedb_ffi.a -ldl -lm -lpthread
// #include "triedb.h"
// #include <stdlib.h>
import "C"
import (
	"encoding/hex"
	"errors"
	"fmt"
	"unsafe"

	"github.com/holiman/uint256"
)

// Error codes matching the C enum
var (
	ErrInvalidPath        = errors.New("invalid path")
	ErrInvalidAddress     = errors.New("invalid address")
	ErrDatabaseOpenFailed = errors.New("failed to open database")
	ErrTransactionFailed  = errors.New("transaction operation failed")
	ErrNullPointer        = errors.New("null pointer provided")
	ErrUtf8Error          = errors.New("UTF-8 conversion error")
	ErrAccountNotFound    = errors.New("account not found")
	ErrStorageNotFound    = errors.New("storage slot not found")
)

const (
	AddressLength = 20
	HashLength    = 32
)

// mapError converts C error codes to Go errors
func mapError(code uint32) error {
	switch code {
	case uint32(C.Success):
		return nil
	case uint32(C.InvalidPath):
		return ErrInvalidPath
	case uint32(C.InvalidAddress):
		return ErrInvalidAddress
	case uint32(C.DatabaseOpenFailed):
		return ErrDatabaseOpenFailed
	case uint32(C.TransactionFailed):
		return ErrTransactionFailed
	case uint32(C.NullPointer):
		return ErrNullPointer
	case uint32(C.Utf8Error):
		return ErrUtf8Error
	case uint32(C.AccountNotFound):
		return ErrAccountNotFound
	case uint32(C.StorageNotFound):
		return ErrStorageNotFound
	default:
		return fmt.Errorf("unknown error code: %d", code)
	}
}

// Address represents a 20-byte Ethereum address
type Address [AddressLength]byte

// Hash represents a 32-byte hash
type Hash [HashLength]byte

// Account represents an Ethereum account
type Account struct {
	Nonce       uint64
	Balance     *uint256.Int
	StorageRoot Hash
	CodeHash    []byte
}

// Database represents a TrieDB instance
type Database struct {
	ptr *C.TrieDB
}

// TransactionRO represents a read-only transaction
type TransactionRO struct {
	ptr *C.TrieDBTransactionRO
}

// TransactionRW represents a read-write transaction
type TransactionRW struct {
	ptr *C.TrieDBTransactionRW
}

// Open opens an existing database at the given path
func Open(path string) (*Database, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var dbPtr *C.TrieDB
	result := C.triedb_open(cPath, &dbPtr)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	return &Database{ptr: dbPtr}, nil
}

// CreateNew creates a new database at the given path (fails if exists)
func CreateNew(path string) (*Database, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var dbPtr *C.TrieDB
	result := C.triedb_create_new(cPath, &dbPtr)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	return &Database{ptr: dbPtr}, nil
}

// Close closes the database and frees resources
func (db *Database) Close() error {
	if db.ptr == nil {
		return ErrNullPointer
	}
	result := C.triedb_close(db.ptr)
	db.ptr = nil
	return mapError(uint32(result))
}

// StateRoot returns the current state root hash
func (db *Database) StateRoot() (Hash, error) {
	if db.ptr == nil {
		return Hash{}, ErrNullPointer
	}

	var hash C.CHash
	result := C.triedb_state_root(db.ptr, &hash)
	if err := mapError(uint32(result)); err != nil {
		return Hash{}, err
	}

	var root Hash
	copy(root[:], C.GoBytes(unsafe.Pointer(&hash.bytes[0]), C.int(HashLength)))
	return root, nil
}

// Size returns the database size in pages
func (db *Database) Size() (uint32, error) {
	if db.ptr == nil {
		return 0, ErrNullPointer
	}

	var size C.uint32_t
	result := C.triedb_size(db.ptr, &size)
	if err := mapError(uint32(result)); err != nil {
		return 0, err
	}

	return uint32(size), nil
}

// BeginRO begins a read-only transaction
func (db *Database) BeginRO() (*TransactionRO, error) {
	if db.ptr == nil {
		return nil, ErrNullPointer
	}

	var txPtr *C.TrieDBTransactionRO
	result := C.triedb_begin_ro(db.ptr, &txPtr)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	return &TransactionRO{ptr: txPtr}, nil
}

// BeginRW begins a read-write transaction (blocks if another RW exists)
func (db *Database) BeginRW() (*TransactionRW, error) {
	if db.ptr == nil {
		return nil, ErrNullPointer
	}

	var txPtr *C.TrieDBTransactionRW
	result := C.triedb_begin_rw(db.ptr, &txPtr)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	return &TransactionRW{ptr: txPtr}, nil
}

// GetAccount retrieves an account from a read-only transaction
func (tx *TransactionRO) GetAccount(address Address) (*Account, error) {
	if tx.ptr == nil {
		return nil, ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	var cAccount C.CAccount
	var exists C.bool
	result := C.triedb_ro_get_account(tx.ptr, &cAddr, &cAccount, &exists)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	// Convert C account to Go account
	balance := new(uint256.Int)
	balance.SetBytes(C.GoBytes(unsafe.Pointer(&cAccount.balance[0]), C.int(HashLength)))

	var storageRoot Hash
	copy(storageRoot[:], C.GoBytes(unsafe.Pointer(&cAccount.storage_root[0]), C.int(HashLength)))

	codeHash := C.GoBytes(unsafe.Pointer(&cAccount.code_hash[0]), C.int(HashLength))

	return &Account{
		Nonce:       uint64(cAccount.nonce),
		Balance:     balance,
		StorageRoot: storageRoot,
		CodeHash:    codeHash,
	}, nil
}

// GetStorage retrieves a storage slot from a read-only transaction
func (tx *TransactionRO) GetStorage(address Address, slot Hash) (*Hash, error) {
	if tx.ptr == nil {
		return nil, ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	var cSlot C.CStorageKey
	copyBytesToC(&cSlot.bytes[0], slot[:], HashLength)

	var cValue C.CStorageValue
	var exists C.bool
	result := C.triedb_ro_get_storage(tx.ptr, &cAddr, &cSlot, &cValue, &exists)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	var value Hash
	copy(value[:], C.GoBytes(unsafe.Pointer(&cValue.bytes[0]), C.int(HashLength)))
	return &value, nil
}

// Commit commits a read-only transaction (releases snapshot)
func (tx *TransactionRO) Commit() error {
	if tx.ptr == nil {
		return ErrNullPointer
	}
	result := C.triedb_ro_commit(tx.ptr)
	tx.ptr = nil
	return mapError(uint32(result))
}

// GetAccount retrieves an account from a read-write transaction
func (tx *TransactionRW) GetAccount(address Address) (*Account, error) {
	if tx.ptr == nil {
		return nil, ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	var cAccount C.CAccount
	var exists C.bool
	result := C.triedb_rw_get_account(tx.ptr, &cAddr, &cAccount, &exists)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	balance := new(uint256.Int)
	balance.SetBytes(C.GoBytes(unsafe.Pointer(&cAccount.balance[0]), C.int(HashLength)))

	var storageRoot Hash
	copy(storageRoot[:], C.GoBytes(unsafe.Pointer(&cAccount.storage_root[0]), C.int(HashLength)))

	codeHash := C.GoBytes(unsafe.Pointer(&cAccount.code_hash[0]), C.int(HashLength))

	return &Account{
		Nonce:       uint64(cAccount.nonce),
		Balance:     balance,
		StorageRoot: storageRoot,
		CodeHash:    codeHash,
	}, nil
}

// SetAccount sets an account in a read-write transaction (nil to delete)
func (tx *TransactionRW) SetAccount(address Address, account *Account) error {
	if tx.ptr == nil {
		return ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	if account == nil {
		// Delete account
		result := C.triedb_rw_set_account(tx.ptr, &cAddr, nil)
		return mapError(uint32(result))
	}

	// Set account
	var cAccount C.CAccount
	cAccount.nonce = C.uint64_t(account.Nonce)

	// Convert uint256.Int to 32-byte big-endian
	balanceBytes := account.Balance.Bytes32()
	copyBytesToC(&cAccount.balance[0], balanceBytes[:], HashLength)

	copyBytesToC(&cAccount.storage_root[0], account.StorageRoot[:], HashLength)

	if len(account.CodeHash) != HashLength {
		return fmt.Errorf("account code hash must be %d bytes, got %d", HashLength, len(account.CodeHash))
	}
	copyBytesToC(&cAccount.code_hash[0], account.CodeHash, HashLength)

	result := C.triedb_rw_set_account(tx.ptr, &cAddr, &cAccount)
	return mapError(uint32(result))
}

// GetStorage retrieves a storage slot from a read-write transaction
func (tx *TransactionRW) GetStorage(address Address, slot Hash) (*Hash, error) {
	if tx.ptr == nil {
		return nil, ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	var cSlot C.CStorageKey
	copyBytesToC(&cSlot.bytes[0], slot[:], HashLength)

	var cValue C.CStorageValue
	var exists C.bool
	result := C.triedb_rw_get_storage(tx.ptr, &cAddr, &cSlot, &cValue, &exists)
	if err := mapError(uint32(result)); err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	var value Hash
	copy(value[:], C.GoBytes(unsafe.Pointer(&cValue.bytes[0]), C.int(HashLength)))
	return &value, nil
}

// SetStorage sets a storage slot in a read-write transaction (nil to delete)
func (tx *TransactionRW) SetStorage(address Address, slot Hash, value *Hash) error {
	if tx.ptr == nil {
		return ErrNullPointer
	}

	var cAddr C.CAddress
	copyBytesToC(&cAddr.bytes[0], address[:], AddressLength)

	var cSlot C.CStorageKey
	copyBytesToC(&cSlot.bytes[0], slot[:], HashLength)

	if value == nil {
		// Delete storage
		result := C.triedb_rw_set_storage(tx.ptr, &cAddr, &cSlot, nil)
		return mapError(uint32(result))
	}

	var cValue C.CStorageValue
	copyBytesToC(&cValue.bytes[0], (*value)[:], HashLength)

	result := C.triedb_rw_set_storage(tx.ptr, &cAddr, &cSlot, &cValue)
	return mapError(uint32(result))
}

// Commit commits a read-write transaction (persists changes)
func (tx *TransactionRW) Commit() error {
	if tx.ptr == nil {
		return ErrNullPointer
	}
	result := C.triedb_rw_commit(tx.ptr)
	tx.ptr = nil
	return mapError(uint32(result))
}

// Rollback rolls back a read-write transaction (discards changes)
func (tx *TransactionRW) Rollback() error {
	if tx.ptr == nil {
		return ErrNullPointer
	}
	result := C.triedb_rw_rollback(tx.ptr)
	tx.ptr = nil
	return mapError(uint32(result))
}

// Helper functions for common conversions

// AddressFromHex parses a hex string into an Address
func AddressFromHex(s string) (Address, error) {
	s = stripHexPrefix(s)
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, err
	}
	if len(bytes) != AddressLength {
		return Address{}, fmt.Errorf("address must be %d bytes", AddressLength)
	}
	var addr Address
	copy(addr[:], bytes)
	return addr, nil
}

// HashFromHex parses a hex string into a Hash
func HashFromHex(s string) (Hash, error) {
	s = stripHexPrefix(s)
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return Hash{}, err
	}
	if len(bytes) != HashLength {
		return Hash{}, fmt.Errorf("hash must be %d bytes", HashLength)
	}
	var hash Hash
	copy(hash[:], bytes)
	return hash, nil
}

// Hex returns the hex string representation
func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// Hex returns the hex string representation
func (h Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}

func stripHexPrefix(s string) string {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		return s[2:]
	}
	return s
}

// Helper function to copy Go bytes to C array
func copyBytesToC(dst *C.uint8_t, src []byte, n int) {
	for i := 0; i < n && i < len(src); i++ {
		*(*C.uint8_t)(unsafe.Pointer(uintptr(unsafe.Pointer(dst)) + uintptr(i))) = C.uint8_t(src[i])
	}
}
