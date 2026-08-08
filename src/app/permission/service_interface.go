package permission

import "context"

type Service interface {
	CheckPermission(ctx context.Context, input *CheckPermissionInput) *CheckPermissionOutput
	GrantPermission(ctx context.Context, input *GrantPermissionInput) *GrantPermissionOutput
	RevokePermission(ctx context.Context, input *RevokePermissionInput) *RevokePermissionOutput
}

type CheckPermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type CheckPermissionOutput struct {
	Success       bool
	Message       string
	TraceId       string
	HasPermission bool
}

type GrantPermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type GrantPermissionOutput struct {
	Success bool
	Message string
	TraceId string
}

type RevokePermissionInput struct {
	TraceId    string
	UserID     int
	Permission string
}

type RevokePermissionOutput struct {
	Success bool
	Message string
	TraceId string
}
