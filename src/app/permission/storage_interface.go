package permission

import (
	"context"
	"errors"
)

// Sentinel errors returned by the storage layer so the service can decide
// intent without parsing driver errors.
var (
	// ErrRoleNameTaken is returned by CreateRole when a role with the same
	// name already exists (roles.name is UNIQUE).
	ErrRoleNameTaken = errors.New("role name already taken")

	// ErrRoleNotFound is returned by GetRoleById when no role has the id.
	ErrRoleNotFound = errors.New("role not found")
)

type Storage interface {
	BeginTx(ctx context.Context) (StorageTx, error)
}

type StorageTx interface {
	// Direct per-user permissions (user_permissions).
	AddPermission(ctx context.Context, userID int, permission string) error
	HasPermission(ctx context.Context, userID int, permission string) (bool, error)
	RemovePermission(ctx context.Context, userID int, permission string) error

	// Roles + role->permission mapping.
	CreateRole(ctx context.Context, name, description string) (int, error)
	GetRoleById(ctx context.Context, roleID int) (*Role, error)
	DeleteRole(ctx context.Context, roleID int) error
	AddRolePermission(ctx context.Context, roleID int, permission string) error
	RemoveRolePermission(ctx context.Context, roleID int, permission string) error

	// User->role assignment (user_roles).
	AssignRole(ctx context.Context, userID, roleID int) error
	UnassignRole(ctx context.Context, userID, roleID int) error

	// HasRolePermission reports whether any role assigned to userID grants the
	// bare permission, including the "super_user" wildcard role permission.
	HasRolePermission(ctx context.Context, userID int, permission string) (bool, error)

	// ListUserRoles returns every role assigned to userID.
	ListUserRoles(ctx context.Context, userID int) ([]Role, error)

	// ListRolePermissions returns the bare permissions a role holds.
	ListRolePermissions(ctx context.Context, roleID int) ([]string, error)

	// ListDirectPermissions returns the user's direct permissions with the
	// "<user_id>:" key prefix stripped, sorted.
	ListDirectPermissions(ctx context.Context, userID int) ([]string, error)

	// ListUserPermissions returns the user's effective permissions: direct
	// grants (key prefix stripped) plus every permission granted by the
	// user's roles, deduplicated and sorted.
	ListUserPermissions(ctx context.Context, userID int) ([]string, error)

	Commit() error
	Rollback() error
}
