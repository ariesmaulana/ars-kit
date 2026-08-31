# Permission Module

Role-based access control. Users hold **roles**, roles carry **permissions**, checks resolve through `user_roles` → `role_permissions`. Permissions are bare strings (`super_user`, `user:profile_update`, `default`) — the permission module never stores a `<user_id>:<permission>` key. The old `user_permissions` table was dropped in `20260128` and replaced by RBAC.

## When Is a Permission Check Allowed?

`CheckPermission(userID, permission)` returns `true` only if **one** holds:

1. One of the user's roles carries the permission via `role_permissions`
2. The user holds the `super_user` role (wildcard — passes every check)

Otherwise `false`.

Example — user `5` asking for `user:profile_update`:

| User 5 roles | Check `user:profile_update` |
|---|---|
| `member` where `member` carries `user:profile_update` | ✅ allowed |
| (no role with that permission) | ❌ denied |
| `super_user` | ✅ allowed (wildcard) |

## Roles

Seeded in `20260128_000004_role_based_access.sql`:

* `super_user` — **wildcard + bootstrap-only**. Never assigned at runtime (`AssignRole` refuses it). No rows in `role_permissions` by design — wildcard is implemented in `service.CheckPermission` fallback. Managed via SOP/direct DB if needed.
* `member` — default role for self-registered users, initially carries `default` permission.

Additional roles are inserted via SOP/direct DB (no API yet). `RoleExists` guards all mutations.

## Permission String Format

Bare strings, no user ID embedded:

```
<module>:<action>  e.g. user:profile_update, user:password_update
super_user         e.g. super_user (wildcard)
default            e.g. default (demo workflow)
```

Each module declares its permissions as constants in `const.go` (e.g. `src/app/user/const.go`). The permission catalog (`permissions` table) is the allowlist — `AssignPermissionToRole` rejects any permission not in the catalog.

## Catalog

`permissions` table is seeded manually via SOP when a new feature ships:

```sql
INSERT INTO permissions (permission) VALUES ('user:profile_update') ON CONFLICT DO NOTHING;
```

Source of truth for valid strings is each module's `const.go`.

## API

| Operation | Method | Gating |
|---|---|---|
| `CheckPermission` | `permission.Service.CheckPermission` | — |
| `AssignRole` | `permission.Service.AssignRole` | refuses `super_user`, requires known role, audited |
| `UnassignRole` | `permission.Service.UnassignRole` | protects last `super_user` holder, audited |
| `AssignPermissionToRole` | `permission.Service.AssignPermissionToRole` | permission must be in catalog, refuses `super_user` role, audited |
| `RemovePermissionFromRole` | `permission.Service.RemovePermissionFromRole` | same as above, audited |

All mutations are transactional with `permission_audit` insert and use `ON CONFLICT DO NOTHING` (idempotent grant) / `DELETE` (idempotent revoke).

Admin HTTP is via `user` module: `POST /api/v1/users/roles/assign`, `/unassign`, `/roles/permissions/grant`, `/revoke` — all require actor holds `super_user`.

## How Permissions Are Checked

User module enforces (see `src/app/user/const.go`):

| Operation | Permission required |
|---|---|
| `UpdateUsername` | `user:profile_update` or `super_user` wildcard |
| `UpdatePassword` | `user:password_update` or `super_user` wildcard |
| `AssignRole` / `UnassignRole` / `AssignPermissionToRole` / `RemovePermissionFromRole` / `ListUsers` / `GetUser` / `DeleteUser` / `UpdateUserStatus` | `super_user` |

## Storage

| Table | Columns | Notes |
|---|---|---|
| `roles` | `id, name UNIQUE, created_at` | seeded `super_user`, `member` |
| `user_roles` | `user_id, role_id UNIQUE(user_id,role_id)` | no cross-domain FK by design — domain isolation |
| `role_permissions` | `role_id, permission PK(role_id,permission)` | permission must exist in `permissions` catalog (app-level check) |
| `permissions` | `permission PK, created_at` | allowlist |
| `permission_audit` | `id, actor_id NULL, target_id, permission, action grant/revoke, created_at` | transactional audit, `target_id=0` for role-content changes |

Cross-domain FKs (e.g. `user_roles.user_id -> users.id`) are intentionally omitted — each domain owns its tables only. Orphan cleanup is handled in service layer.

## Direct SQL (manual/SOP)

```sql
-- grant permission to role (catalog must contain the permission)
INSERT INTO role_permissions (role_id, permission)
SELECT id, 'user:profile_update' FROM roles WHERE name = 'member'
ON CONFLICT DO NOTHING;

-- assign role to user
INSERT INTO user_roles (user_id, role_id)
SELECT 5, id FROM roles WHERE name = 'member'
ON CONFLICT DO NOTHING;
```
