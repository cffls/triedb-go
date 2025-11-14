use std::ffi::{c_char, CStr};
use std::sync::Arc;

use alloy_primitives::{Address, StorageKey, B256, U256};
use triedb::{
    account::Account,
    path::{AddressPath, StoragePath},
    transaction::{Transaction, RO, RW},
    Database,
};

// ============================================================================
// Error Handling
// ============================================================================

#[repr(C)]
#[derive(Debug, Copy, Clone, PartialEq, Eq)]
pub enum TrieDBError {
    Success = 0,
    InvalidPath = 1,
    InvalidAddress = 2,
    DatabaseOpenFailed = 3,
    TransactionFailed = 4,
    NullPointer = 5,
    Utf8Error = 6,
    AccountNotFound = 7,
    StorageNotFound = 8,
}

// ============================================================================
// Opaque Types (Hide Rust internals from C)
// ============================================================================

pub struct TrieDB {
    inner: Arc<Database>,
}

pub struct TrieDBTransactionRO {
    inner: Option<Transaction<Arc<Database>, RO>>,
}

pub struct TrieDBTransactionRW {
    inner: Option<Transaction<Arc<Database>, RW>>,
}

// ============================================================================
// C-Compatible Data Structures
// ============================================================================

#[repr(C)]
pub struct CAccount {
    pub nonce: u64,
    pub balance: [u8; 32],
    pub storage_root: [u8; 32],
    pub code_hash: [u8; 32],
}

#[repr(C)]
pub struct CAddress {
    pub bytes: [u8; 20],
}

#[repr(C)]
pub struct CStorageKey {
    pub bytes: [u8; 32],
}

#[repr(C)]
pub struct CStorageValue {
    pub bytes: [u8; 32],
}

#[repr(C)]
pub struct CHash {
    pub bytes: [u8; 32],
}

// ============================================================================
// Database Operations
// ============================================================================

/// Opens an existing database at the given path.
///
/// # Safety
/// - `path` must be a valid null-terminated UTF-8 string
/// - `out_db` must be a valid pointer
/// - Caller must call `triedb_close` to free resources
#[no_mangle]
pub unsafe extern "C" fn triedb_open(path: *const c_char, out_db: *mut *mut TrieDB) -> TrieDBError {
    if path.is_null() || out_db.is_null() {
        return TrieDBError::NullPointer;
    }

    let path_str = match CStr::from_ptr(path).to_str() {
        Ok(s) => s,
        Err(_) => return TrieDBError::Utf8Error,
    };

    match Database::open(path_str) {
        Ok(db) => {
            let boxed = Box::new(TrieDB {
                inner: Arc::new(db),
            });
            *out_db = Box::into_raw(boxed);
            TrieDBError::Success
        }
        Err(_) => TrieDBError::DatabaseOpenFailed,
    }
}

/// Creates a new database at the given path (fails if exists).
///
/// # Safety
/// - `path` must be a valid null-terminated UTF-8 string
/// - `out_db` must be a valid pointer
/// - Caller must call `triedb_close` to free resources
#[no_mangle]
pub unsafe extern "C" fn triedb_create_new(
    path: *const c_char,
    out_db: *mut *mut TrieDB,
) -> TrieDBError {
    if path.is_null() || out_db.is_null() {
        return TrieDBError::NullPointer;
    }

    let path_str = match CStr::from_ptr(path).to_str() {
        Ok(s) => s,
        Err(_) => return TrieDBError::Utf8Error,
    };

    match Database::create_new(path_str) {
        Ok(db) => {
            let boxed = Box::new(TrieDB {
                inner: Arc::new(db),
            });
            *out_db = Box::into_raw(boxed);
            TrieDBError::Success
        }
        Err(_) => TrieDBError::DatabaseOpenFailed,
    }
}

/// Closes the database and frees resources.
///
/// # Safety
/// - `db` must be a valid pointer from `triedb_open` or `triedb_create_new`
/// - `db` must not be used after this call
#[no_mangle]
pub unsafe extern "C" fn triedb_close(db: *mut TrieDB) -> TrieDBError {
    if db.is_null() {
        return TrieDBError::NullPointer;
    }

    // Drop the Box, which will drop the Arc<Database>
    let _ = Box::from_raw(db);
    TrieDBError::Success
}

/// Gets the current state root hash.
///
/// # Safety
/// - `db` must be a valid pointer
/// - `out_root` must be a valid pointer to a 32-byte array
#[no_mangle]
pub unsafe extern "C" fn triedb_state_root(db: *const TrieDB, out_root: *mut CHash) -> TrieDBError {
    if db.is_null() || out_root.is_null() {
        return TrieDBError::NullPointer;
    }

    let db = &*db;
    let root = db.inner.state_root();
    (*out_root).bytes.copy_from_slice(root.as_slice());
    TrieDBError::Success
}

/// Gets the database size in pages.
///
/// # Safety
/// - `db` must be a valid pointer
/// - `out_size` must be a valid pointer
#[no_mangle]
pub unsafe extern "C" fn triedb_size(db: *const TrieDB, out_size: *mut u32) -> TrieDBError {
    if db.is_null() || out_size.is_null() {
        return TrieDBError::NullPointer;
    }

    let db = &*db;
    *out_size = db.inner.size();
    TrieDBError::Success
}

// ============================================================================
// Read-Only Transactions
// ============================================================================

/// Begins a read-only transaction.
///
/// # Safety
/// - `db` must be a valid pointer
/// - `out_tx` must be a valid pointer
/// - Caller must call `triedb_ro_commit` or `triedb_ro_free` to free resources
#[no_mangle]
pub unsafe extern "C" fn triedb_begin_ro(
    db: *const TrieDB,
    out_tx: *mut *mut TrieDBTransactionRO,
) -> TrieDBError {
    if db.is_null() || out_tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let db = &*db;
    match triedb::database::begin_ro(db.inner.clone()) {
        Ok(tx) => {
            let boxed = Box::new(TrieDBTransactionRO { inner: Some(tx) });
            *out_tx = Box::into_raw(boxed);
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Gets an account from a read-only transaction.
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `address` must be a valid pointer to a 20-byte array
/// - `out_account` must be a valid pointer
/// - `out_exists` must be a valid pointer
#[no_mangle]
pub unsafe extern "C" fn triedb_ro_get_account(
    tx: *mut TrieDBTransactionRO,
    address: *const CAddress,
    out_account: *mut CAccount,
    out_exists: *mut bool,
) -> TrieDBError {
    if tx.is_null() || address.is_null() || out_account.is_null() || out_exists.is_null() {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let path = AddressPath::for_address(addr);

    match tx_inner.get_account(&path) {
        Ok(Some(account)) => {
            (*out_account).nonce = account.nonce;
            account
                .balance
                .to_be_bytes_vec()
                .as_slice()
                .iter()
                .enumerate()
                .for_each(|(i, &b)| {
                    (*out_account).balance[i] = b;
                });
            (*out_account)
                .storage_root
                .copy_from_slice(account.storage_root.as_slice());
            (*out_account)
                .code_hash
                .copy_from_slice(account.code_hash.as_slice());
            *out_exists = true;
            TrieDBError::Success
        }
        Ok(None) => {
            *out_exists = false;
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Gets a storage slot from a read-only transaction.
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `address` must be a valid pointer to a 20-byte array
/// - `slot` must be a valid pointer to a 32-byte array
/// - `out_value` must be a valid pointer
/// - `out_exists` must be a valid pointer
#[no_mangle]
pub unsafe extern "C" fn triedb_ro_get_storage(
    tx: *mut TrieDBTransactionRO,
    address: *const CAddress,
    slot: *const CStorageKey,
    out_value: *mut CStorageValue,
    out_exists: *mut bool,
) -> TrieDBError {
    if tx.is_null()
        || address.is_null()
        || slot.is_null()
        || out_value.is_null()
        || out_exists.is_null()
    {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let slot_key = StorageKey::from_slice(&(*slot).bytes);
    let path = StoragePath::for_address_and_slot(addr, slot_key);

    match tx_inner.get_storage_slot(&path) {
        Ok(Some(value)) => {
            (*out_value)
                .bytes
                .copy_from_slice(&value.to_be_bytes::<32>());
            *out_exists = true;
            TrieDBError::Success
        }
        Ok(None) => {
            *out_exists = false;
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Commits a read-only transaction (releases snapshot).
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `tx` must not be used after this call
#[no_mangle]
pub unsafe extern "C" fn triedb_ro_commit(tx: *mut TrieDBTransactionRO) -> TrieDBError {
    if tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let mut tx_box = Box::from_raw(tx);
    if let Some(tx_inner) = tx_box.inner.take() {
        match tx_inner.commit() {
            Ok(_) => TrieDBError::Success,
            Err(_) => TrieDBError::TransactionFailed,
        }
    } else {
        TrieDBError::TransactionFailed
    }
}

/// Frees a read-only transaction without committing.
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `tx` must not be used after this call
#[no_mangle]
pub unsafe extern "C" fn triedb_ro_free(tx: *mut TrieDBTransactionRO) -> TrieDBError {
    if tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let _ = Box::from_raw(tx);
    TrieDBError::Success
}

// ============================================================================
// Read-Write Transactions
// ============================================================================

/// Begins a read-write transaction (blocks if another RW transaction exists).
///
/// # Safety
/// - `db` must be a valid pointer
/// - `out_tx` must be a valid pointer
/// - Caller must call `triedb_rw_commit` or `triedb_rw_rollback` to free resources
#[no_mangle]
pub unsafe extern "C" fn triedb_begin_rw(
    db: *const TrieDB,
    out_tx: *mut *mut TrieDBTransactionRW,
) -> TrieDBError {
    if db.is_null() || out_tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let db = &*db;
    match triedb::database::begin_rw(db.inner.clone()) {
        Ok(tx) => {
            let boxed = Box::new(TrieDBTransactionRW { inner: Some(tx) });
            *out_tx = Box::into_raw(boxed);
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Gets an account from a read-write transaction.
///
/// # Safety
/// - Same safety requirements as `triedb_ro_get_account`
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_get_account(
    tx: *mut TrieDBTransactionRW,
    address: *const CAddress,
    out_account: *mut CAccount,
    out_exists: *mut bool,
) -> TrieDBError {
    if tx.is_null() || address.is_null() || out_account.is_null() || out_exists.is_null() {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let path = AddressPath::for_address(addr);

    match tx_inner.get_account(&path) {
        Ok(Some(account)) => {
            (*out_account).nonce = account.nonce;
            account
                .balance
                .to_be_bytes_vec()
                .as_slice()
                .iter()
                .enumerate()
                .for_each(|(i, &b)| {
                    (*out_account).balance[i] = b;
                });
            (*out_account)
                .storage_root
                .copy_from_slice(account.storage_root.as_slice());
            (*out_account)
                .code_hash
                .copy_from_slice(account.code_hash.as_slice());
            *out_exists = true;
            TrieDBError::Success
        }
        Ok(None) => {
            *out_exists = false;
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Sets an account in a read-write transaction.
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `address` must be a valid pointer to a 20-byte array
/// - `account` must be a valid pointer (or NULL to delete)
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_set_account(
    tx: *mut TrieDBTransactionRW,
    address: *const CAddress,
    account: *const CAccount,
) -> TrieDBError {
    if tx.is_null() || address.is_null() {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let path = AddressPath::for_address(addr);

    let account_opt = if account.is_null() {
        None
    } else {
        let balance = U256::from_be_slice(&(*account).balance);
        let storage_root = B256::from_slice(&(*account).storage_root);
        let code_hash = B256::from_slice(&(*account).code_hash);
        Some(Account::new(
            (*account).nonce,
            balance,
            storage_root,
            code_hash,
        ))
    };

    match tx_inner.set_account(path, account_opt) {
        Ok(_) => TrieDBError::Success,
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Gets a storage slot from a read-write transaction.
///
/// # Safety
/// - Same safety requirements as `triedb_ro_get_storage`
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_get_storage(
    tx: *mut TrieDBTransactionRW,
    address: *const CAddress,
    slot: *const CStorageKey,
    out_value: *mut CStorageValue,
    out_exists: *mut bool,
) -> TrieDBError {
    if tx.is_null()
        || address.is_null()
        || slot.is_null()
        || out_value.is_null()
        || out_exists.is_null()
    {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let slot_key = StorageKey::from_slice(&(*slot).bytes);
    let path = StoragePath::for_address_and_slot(addr, slot_key);

    match tx_inner.get_storage_slot(&path) {
        Ok(Some(value)) => {
            (*out_value)
                .bytes
                .copy_from_slice(&value.to_be_bytes::<32>());
            *out_exists = true;
            TrieDBError::Success
        }
        Ok(None) => {
            *out_exists = false;
            TrieDBError::Success
        }
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Sets a storage slot in a read-write transaction.
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `address` must be a valid pointer to a 20-byte array
/// - `slot` must be a valid pointer to a 32-byte array
/// - `value` must be a valid pointer (or NULL to delete)
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_set_storage(
    tx: *mut TrieDBTransactionRW,
    address: *const CAddress,
    slot: *const CStorageKey,
    value: *const CStorageValue,
) -> TrieDBError {
    if tx.is_null() || address.is_null() || slot.is_null() {
        return TrieDBError::NullPointer;
    }

    let tx_ref = &mut *tx;
    let tx_inner = match tx_ref.inner.as_mut() {
        Some(t) => t,
        None => return TrieDBError::TransactionFailed,
    };

    let addr = Address::from_slice(&(*address).bytes);
    let slot_key = StorageKey::from_slice(&(*slot).bytes);
    let path = StoragePath::for_address_and_slot(addr, slot_key);

    let value_opt = if value.is_null() {
        None
    } else {
        Some(U256::from_be_bytes((*value).bytes))
    };

    match tx_inner.set_storage_slot(path, value_opt) {
        Ok(_) => TrieDBError::Success,
        Err(_) => TrieDBError::TransactionFailed,
    }
}

/// Commits a read-write transaction (persists changes).
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `tx` must not be used after this call
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_commit(tx: *mut TrieDBTransactionRW) -> TrieDBError {
    if tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let mut tx_box = Box::from_raw(tx);
    if let Some(tx_inner) = tx_box.inner.take() {
        match tx_inner.commit() {
            Ok(_) => TrieDBError::Success,
            Err(_) => TrieDBError::TransactionFailed,
        }
    } else {
        TrieDBError::TransactionFailed
    }
}

/// Rolls back a read-write transaction (discards changes).
///
/// # Safety
/// - `tx` must be a valid pointer
/// - `tx` must not be used after this call
#[no_mangle]
pub unsafe extern "C" fn triedb_rw_rollback(tx: *mut TrieDBTransactionRW) -> TrieDBError {
    if tx.is_null() {
        return TrieDBError::NullPointer;
    }

    let mut tx_box = Box::from_raw(tx);
    if let Some(tx_inner) = tx_box.inner.take() {
        match tx_inner.rollback() {
            Ok(_) => TrieDBError::Success,
            Err(_) => TrieDBError::TransactionFailed,
        }
    } else {
        TrieDBError::TransactionFailed
    }
}

// ============================================================================
// Utility Functions
// ============================================================================

/// Returns a human-readable error message for an error code.
///
/// # Safety
/// - Returns a static string, no need to free
#[no_mangle]
pub extern "C" fn triedb_error_message(error: TrieDBError) -> *const c_char {
    let msg = match error {
        TrieDBError::Success => "Success\0",
        TrieDBError::InvalidPath => "Invalid path\0",
        TrieDBError::InvalidAddress => "Invalid address\0",
        TrieDBError::DatabaseOpenFailed => "Failed to open database\0",
        TrieDBError::TransactionFailed => "Transaction operation failed\0",
        TrieDBError::NullPointer => "Null pointer provided\0",
        TrieDBError::Utf8Error => "UTF-8 conversion error\0",
        TrieDBError::AccountNotFound => "Account not found\0",
        TrieDBError::StorageNotFound => "Storage slot not found\0",
    };
    msg.as_ptr() as *const c_char
}
