# Permission Module

Manages per-user permissions stored in the `user_permissions` table and
role-based permissions in `roles` / `role_permissions` / `user_roles`. Each
direct permission grants one user the right to perform an action in a module;
a role grants the same right to every user assigned to it.

## When Is a Permission Check Allowed?

A permission check for a user and an action returns `true` **if at least one**
of the following holds:

1. The user holds the exact direct permission for that function: `<user_id>:<module>:<action>`
2. The user holds the direct super user permission: `<user_id>:super_user` (wildcard —
   overrides every action check)
3. **Any role assigned to the user** holds the bare permission `<module>:<action>`, or
   holds the bare `super_user` wildcard (B1 role fallback)

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

The grant/revoke API and the check path take the **bare permission** (e.g.
`user:profile_update` or `super_user`) plus a user id. The permission module
builds the `<user_id>:<permission>` key itself before storing or checking, so a
granted permission always matches a later check.

### Via the grant API

`POST /api/v1/users/permissions/grant` (actor must hold `<actor_id>:super_user`):

```json
{
  "user_id": 5,
  "permission": "user:profile_update"
}
```

The module stores it as `5:user:profile_update`. Do **not** include the user id
in the `permission` field — it would be prefixed again.

### Directly in SQL

```sql
INSERT INTO user_permissions (user_id, permission)
VALUES (5, '5:user:profile_update')
ON CONFLICT DO NOTHING;
```

When writing rows directly, store the **full key** (`<user_id>:<permission>`),
since checks always look it up in that form. Granting via the API is
idempotent — re-granting an existing permission is a no-op. Revoking
(`POST /api/v1/users/permissions/revoke`) deletes the row.

## Roles (B1)

A **role** is a named group of permissions shared across every user assigned
to it. Three tables implement the role model:

| Table               | Purpose                                            |
|---------------------|----------------------------------------------------|
| `roles`             | the role itself (`name` unique, `description`)     |
| `role_permissions`  | role → bare permission mapping (one row per pair)  |
| `user_roles`        | user → role assignment (one row per pair)          |

Unlike `user_permissions`, a role stores the **bare permission string**
(e.g. `user:profile_update` or `super_user`) — never the `<user_id>:...` key
form, because a role is shared across users and cannot embed one user's id.
The role wildcard is the bare `super_user`: any user assigned a role that
holds `super_user` passes every check, exactly like a direct `super_user`
grant.

Role management lives on the permission service (there is no HTTP surface for
it in this task — the admin track adds endpoints):

| Service method          | Effect                                        |
|-------------------------|-----------------------------------------------|
| `CreateRole`            | create a role (name is unique)                |
| `AddRolePermission`     | map a bare permission onto a role             |
| `AssignRole`            | assign a role to a user                       |
| `ListUserPermissions`   | effective listing (direct + roles)            |

### Listing a user's permissions

`GET /api/v1/users/permissions` lists the authenticated user's **effective**
permissions: their direct grants, each role they hold with that role's
permissions, and the deduplicated union of both. An optional `user_id` query
parameter lists another user and requires the actor to hold `super_user`.

```json
{
  "success": true,
  "message": "Permissions retrieved successfully",
  "data": {
    "user_id": 5,
    "direct_permissions": ["user:profile_update"],
    "roles": [
      { "id": 1, "name": "editor", "description": "", "permissions": ["report:view"] }
    ],
    "effective_permissions": ["report:view", "user:profile_update"]
  }
}
```

## How Permissions Are Checked

The user module enforces these keys (see `src/app/user/service_interface.go`
for the module/action constants):

| Operation                | Permission required          |
|--------------------------|------------------------------|
| `UpdateUsername`         | `<user_id>:user:profile_update` or `<user_id>:super_user` |
| `UpdatePassword`         | `<user_id>:user:password_update` or `<user_id>:super_user` |
| `GrantPermission`        | `<actor_id>:super_user`      |
| `RevokePermission`       | `<actor_id>:super_user`      |

New modules should follow the same convention: declare every permission the
module checks as a constant in the module's `const.go` (e.g.
`src/app/user/const.go`), so callers and readers see the full list in one
place without grepping string literals.

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

### Roles (`roles`, `role_permissions`, `user_roles`)

| Table | Column | Type | Notes |
|-------|--------|------|-------|
| `roles` | id | SERIAL | primary key |
| `roles` | name | VARCHAR(100) | `UNIQUE` |
| `roles` | description | VARCHAR(255) | default `''` |
| `roles` | created_at / updated_at | TIMESTAMP | default `NOW()` |
| `role_permissions` | role_id | INTEGER | FK → `roles.id`, `ON DELETE CASCADE` |
| `role_permissions` | permission | VARCHAR(255) | bare permission; `UNIQUE (role_id, permission)` |
| `user_roles` | user_id | INTEGER | no FK (matches `user_permissions` — the domain migrates standalone) |
| `user_roles` | role_id | INTEGER | FK → `roles.id`, `ON DELETE CASCADE`; `UNIQUE (user_id, role_id)` |
