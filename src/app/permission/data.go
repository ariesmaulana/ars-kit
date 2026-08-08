package permission

import "time"

type UserPermission struct {
	Id         int
	UserId     int
	Permission string
	CreatedAt  time.Time
}
