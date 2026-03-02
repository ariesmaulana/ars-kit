# User Domain Lock Rules

This document defines the locking strategy for the user domain to prevent deadlocks.

## General Rules

1. **All update operations MUST use pessimistic row locking** (`SELECT ... FOR UPDATE`)
2. **Locks are only available within transaction writer** (StorageTx interface)
3. **Lock ordering must be strictly followed** to prevent deadlocks

## Lock Hierarchy

Locks must be acquired in this order:

```
1. users table (higher priority)
2. members table (lower priority)
```

## Lock Ordering Rules

### Rule 1: Lock by Table Hierarchy
When operations span multiple tables, always lock in this order:
1. Lock `users` first
2. Lock `members` second

Example: When updating a member, lock the user row first, then the member row.

### Rule 2: Lock by Ascending ID
When locking multiple rows from the same table, lock by **ascending ID order**.

Example: If updating members with IDs [5, 2, 8], lock them in order: 2, 5, 8.

### Rule 3: Single Entity Updates
For single-entity updates within one table, lock the target row before any UPDATE/DELETE.

## Operation-Specific Lock Requirements

### UpdateUsername (users table only)
1. Lock user by ID using `LockUserById(ctx, userId)`
2. Perform update

### UpdatePassword (users table only)
1. Lock user by ID using `LockUserById(ctx, userId)`
2. Perform update

### UpdateMemberInfo (members table, validates against users)
1. Lock user by ID using `LockUserById(ctx, userId)` - owner validation
2. Lock member by ID using `LockMemberById(ctx, memberId)`
3. Perform update

### DeleteMember (members table, validates against users)
1. Lock user by ID using `LockUserById(ctx, userId)` - owner validation
2. Lock member by ID using `LockMemberById(ctx, memberId)`
3. Perform delete

### Bulk Operations (future)
If multiple members need updating:
1. Lock the user first
2. Lock all members in ascending ID order
3. Perform updates

## Implementation Notes

- All lock methods return the locked entity to avoid additional SELECT queries
- Lock methods are only available on `StorageTx` interface, not on `Storage`
- Locks are automatically released when transaction commits or rolls back
- If a row doesn't exist, lock methods return `ErrTypeNotFound`

## Deadlock Prevention Checklist

Before implementing any update operation, verify:
- [ ] Are multiple tables involved? Lock by hierarchy (users → members)
- [ ] Are multiple rows from the same table involved? Lock by ascending ID
- [ ] Is the lock acquired within a transaction?
- [ ] Is the lock acquired before any UPDATE/DELETE?

## Example Code

### Correct: Update Member with Proper Lock Ordering

```go
tx, _ := storage.BeginTx(ctx)
defer tx.Rollback()

// 1. Lock user first (hierarchy)
user, err := tx.LockUserById(ctx, userId)
if err != nil {
    return err
}

// 2. Lock member second
member, err := tx.LockMemberById(ctx, memberId)
if err != nil {
    return err
}

// 3. Validate ownership
if member.UserId != user.Id {
    return errors.New("unauthorized")
}

// 4. Perform update
err = tx.UpdateMemberInfo(ctx, memberId, newName, newIncome)
tx.Commit()
```

### Incorrect: Wrong Lock Ordering (DEADLOCK RISK!)

```go
// ❌ BAD: Locking member before user violates hierarchy
tx, _ := storage.BeginTx(ctx)
member, _ := tx.LockMemberById(ctx, memberId) // Wrong order!
user, _ := tx.LockUserById(ctx, userId)       // Should be first!
```
