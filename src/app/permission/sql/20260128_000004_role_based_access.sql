-- +goose Up
-- Role-based access control (P0-10, all-in): roles are the ONLY way access is
-- assigned. Permissions remain the unit being checked; a user holds whatever
-- permissions their roles carry.
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INTEGER NOT NULL,
    permission VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);

-- Seed the built-in roles.
--
-- WILDCARD: members of the super_user role pass EVERY permission check.
-- This is implemented directly in permission.Service.CheckPermission (it
-- falls back to "does the user hold the super_user role" when no role
-- carries the permission), so super_user deliberately has NO entries in
-- role_permissions — nothing to maintain when new permissions ship.
--
-- super_user itself is bootstrap-only: it is never assigned at runtime
-- (the service layer refuses), and even its contents are managed outside
-- the codebase via operational SOP / direct DB (see EXT-SOP task 2).
INSERT INTO roles (name) VALUES ('super_user'), ('member') ON CONFLICT DO NOTHING;

-- member starts with the demo/default permission so the registration
-- workflow keeps working out of the box.
INSERT INTO role_permissions (role_id, permission)
SELECT id, 'default' FROM roles WHERE name = 'member'
ON CONFLICT DO NOTHING;

INSERT INTO permissions (permission) VALUES ('default') ON CONFLICT DO NOTHING;

-- Direct per-user grants are replaced by role assignment.
DROP TABLE IF EXISTS user_permissions;

-- +goose Down
CREATE TABLE IF NOT EXISTS user_permissions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    permission VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (user_id, permission)
);
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
