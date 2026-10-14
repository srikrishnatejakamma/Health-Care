package tenancy

import (
	"context"
	"healthcare/internal/domain"
	"slices"
)

type Scope struct {
	TenantID      string
	Actor         string
	Roles         []string
	Facilities    []string
	CorrelationID string
}
type key struct{}

func With(ctx context.Context, s Scope) context.Context { return context.WithValue(ctx, key{}, s) }
func From(ctx context.Context) (Scope, error) {
	s, ok := ctx.Value(key{}).(Scope)
	if !ok || s.TenantID == "" || s.Actor == "" {
		return Scope{}, domain.ErrForbidden
	}
	return s, nil
}
func (s Scope) Has(roles ...string) bool {
	for _, r := range roles {
		if slices.Contains(s.Roles, r) {
			return true
		}
	}
	return false
}
func (s Scope) Facility(id string) bool {
	return s.Has("tenant_admin", "worker") || slices.Contains(s.Facilities, "*") || slices.Contains(s.Facilities, id)
}
func Worker(ctx context.Context, tenantID string) context.Context {
	return With(ctx, Scope{TenantID: tenantID, Actor: "workflow-worker", Roles: []string{"worker"}, CorrelationID: domain.ID()})
}
