# User Domain Lock Rules

This document defines the locking strategy for the user domain to prevent deadlocks.

## General Rules

1. **All update operations MUST use pessimistic row locking** (`SELECT ... FOR UPDATE`)
2. **Locks are only available within transaction writer** (StorageTx interface)
3. **Locks are strictly ordered** to prevent deadlocks — see below

## Lock Ordering

All current user-domain operations lock only `users`. When multiple rows of the same table are locked, they **must** be locked in ascending ID order.

Future tables in the user domain (if added) will be documented here with their hierarchy. Cross-domain tables are never locked from the user domain — each domain owns its own locks.

## Rule 1: Single Entity Updates
For single-entity updates, lock the target row before any UPDATE/DELETE.

## Rule 2: Multiple Rows Same Table
When locking multiple rows from the same table, lock by **ascending ID order**.

Example: If updating users with IDs [5, 2, 8], lock them in order: 2, 5, 8.

## Operation-Specific Lock Requirements

### UpdateUsername (users table only)
1. Lock user by ID using `LockUserById(ctx, userId)`
2. Perform update

### UpdatePassword (users table only)
1. Lock user by ID using `LockUserById(ctx, userId)`
2. Perform update

### UpdateUserStatus / DeleteUser (users table only)
1. Lock user by ID using `LockUserById(ctx, userId)`
2. Perform update/delete

### Login (users table, throttle state)
1. `GetUserByUsername` (no lock) to resolve ID
2. Lock row via `LockUserLoginState(ctx, userId)` — serializes concurrent logins for the same account
3. `RecordFailedLogin` or `ResetLoginState` + `UpdateLastLogin` within same transaction

## Implementation Notes

- All lock methods return the locked entity to avoid additional SELECT queries
- Lock methods are only available on `StorageTx` interface, not on `Storage`
- Locks are automatically released when transaction commits or rolls back
- If a row doesn't exist, lock methods return `ErrTypeNotFound`

## Deadlock Prevention Checklist

Before implementing any update operation, verify:
- [ ] Are multiple rows from the same table involved? Lock by ascending ID
- [ ] Is the lock acquired within a transaction?
- [ ] Is the lock acquired before any UPDATE/DELETE?

## Example Code

### Correct: Update Username with Proper Lock

```go
tx, _ := storage.BeginTx(ctx)
defer tx.Rollback()

user, errType, err := tx.LockUserById(ctx, userId)
if err != nil {
    return err
}

// Perform update while holding the lock
err = tx.UpdateUsername(ctx, userId, newUsername)
tx.Commit()
```

### Incorrect: Update Without Lock (RACE CONDITION)

```go
// ❌ BAD: No lock, concurrent updates can overwrite each other
tx, _ := storage.BeginTx(ctx)
err = tx.UpdateUsername(ctx, userId, newUsername) // Missing LockUserById!
tx.Commit()
```
