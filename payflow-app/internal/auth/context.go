package auth

import (
	"context"

	"github.com/google/uuid"
)

type principalKind string

const (
	PrincipalAPIKey    principalKind = "api_key"
	PrincipalDashboard principalKind = "dashboard"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxTenantID
	ctxPrincipalKind
	ctxSubjectID // api_keys.id or dashboard_users.id
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestID, id)
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

func WithTenantPrincipal(ctx context.Context, tenantID uuid.UUID, k principalKind, subjectID uuid.UUID) context.Context {
	ctx = context.WithValue(ctx, ctxTenantID, tenantID)
	ctx = context.WithValue(ctx, ctxPrincipalKind, k)
	ctx = context.WithValue(ctx, ctxSubjectID, subjectID)
	return ctx
}

func TenantID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxTenantID).(uuid.UUID)
	return v, ok
}

func Principal(ctx context.Context) (principalKind, bool) {
	v, ok := ctx.Value(ctxPrincipalKind).(principalKind)
	return v, ok
}

func SubjectID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxSubjectID).(uuid.UUID)
	return v, ok
}
