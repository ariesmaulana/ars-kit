package user

// Available permission strings for the user module.
//
// The permission module stores and checks each permission as
// "<user_id>:<permission>". Other modules follow the same convention: a
// const.go listing every permission the module checks, so callers and readers
// (humans or LLMs) can see at a glance what exists without grepping strings.
const (
	// PermissionUpdateProfile gates UpdateUsername.
	PermissionUpdateProfile = "user:profile_update"

	// PermissionUpdatePassword gates UpdatePassword.
	PermissionUpdatePassword = "user:password_update"

	// PermissionSuperUser grants access to every action (wildcard) and the
	// right to manage other users' permissions.
	// Keep in sync with permission.PermissionSuperUser (the permission module
	// implements the wildcard check with it).
	PermissionSuperUser = "super_user"
)
