# Permission Module

Manages per-user permissions stored in the `user_permissions` table. Each
permission grants one user the right to perform an action in a module.

## When Is a Permission Check Allowed?

A permission check for a user and an action returns `true` **only if at least
one** of the following holds:

1. The user holds the exact permission for that function: `<user_id>:<module>:<action>`
2. The user holds the super user permission: `<user_id>:super_user` (wildcard —
   overrides every action check)

Otherwise the check returns `false` and the operation is denied.

Example — user `5` asking to update their username (needs `user:profile_update`):

| User 5 holds                       | Check result |
|------------------------------------|--------------|
| `5:user:profile_update`            | ✅ allowed   |
| (nothing)                          | ❌ denied    |
| `5:super_user`                     | ✅ allowed (wildcard) |
| `5:super_user` + (nothing else)    | ✅ allowed (wildcard) |

## Permission String Format

A permission is a single string with three segments separated by `:`:

```
<user_id>:<module>:<action>
```

| Segment  | Description                                                       | Example              |
|----------|-------------------------------------------------------------------|----------------------|
| user_id  | Database id of the user the permission belongs to                 | `5`                  |
| module   | The module that owns the action, e.g. `user`                      | `user`               |
| action   | The action within the module, e.g. `profile_update`               | `profile_update`     |

Full example: `5:user:profile_update` — user **5** may run the **profile_update**
action of the **user** module.

> **Important:** the permission string **must include the owning user's id**.
> The user id is part of the key itself, so the string is self-contained and
> only meaningful for that user. A string without the id (e.g.
> `user:profile_update`) will never match a permission check.

## Super User Permission

The super user permission grants the right to manage other users' permissions
(grant/revoke). It also acts as a **wildcard**: a user holding
`<user_id>:super_user` passes **any** permission check, even for actions they
were never explicitly granted. It has no module/action segments:

```
<user_id>:super_user
```

Example: `7:super_user` — user **7** may grant/revoke permissions **and** passes
every other permission check.

## Setting a Permission

### Via the grant API

`POST /api/v1/users/permissions/grant` (actor must hold `<actor_id>:super_user`):

```json
{
  "user_id": 5,
  "permission": "5:user:profile_update"
}
```

The `permission` field must be the **full key including the target user's id**.

### Directly in SQL

```sql
INSERT INTO user_permissions (user_id, permission)
VALUES (5, '5:user:profile_update')
ON CONFLICT DO NOTHING;
```

Granting is idempotent — re-granting an existing permission is a no-op.
Revoking (`POST /api/v1/users/permissions/revoke`) deletes the row.

## How Permissions Are Checked

The user module enforces these keys (see `src/app/user/service_interface.go`
for the module/action constants):

| Operation                | Permission required          |
|--------------------------|------------------------------|
| `UpdateUsername`         | `<user_id>:user:profile_update` or `<user_id>:super_user` |
| `UpdatePassword`         | `<user_id>:user:password_update` or `<user_id>:super_user` |
| `GrantPermission`        | `<actor_id>:super_user`      |
| `RevokePermission`       | `<actor_id>:super_user`      |

New modules should follow the same convention: define a module name and action
constants, and check with `<user_id>:<module>:<action>`.

## Storage

| Column      | Type          | Notes                                    |
|-------------|---------------|------------------------------------------|
| id          | SERIAL        | primary key                              |
| user_id     | INTEGER       | the user the permission belongs to       |
| permission  | VARCHAR(255)  | the permission string (format above)     |
| created_at  | TIMESTAMP     | default `NOW()`                          |

Constraints:

- `UNIQUE (user_id, permission)` — one row per (user, permission)
- `permission` is limited to 255 characters
