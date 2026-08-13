package permission

import "time"

type UserPermission struct {
	Id         int
	UserId     int
	Permission string
	CreatedAt  time.Time
}

// Role is a named group of permissions. A role holds bare permission strings
// (e.g. "user:profile_update" or "super_user") shared across every user
// assigned to it; CheckPermission treats a role permission like a direct
// grant, including the "super_user" wildcard.
type Role struct {
	Id          int
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
