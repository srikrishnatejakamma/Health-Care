package rest

import (
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"net/http"
)

// Tenant administrators may lower assigned budgets. Raising ceilings remains a
// platform-control-plane operation, and the version CAS fences suspension races.
func (s *Server) tenantConfiguration(w http.ResponseWriter, r *http.Request) {
	scope, _ := tenancy.From(r.Context())
	if !scope.Has("tenant_admin") {
		s.fail(w, r, domain.ErrForbidden)
		return
	}
	var in struct {
		Version int           `json:"version"`
		Quotas  domain.Quotas `json:"quotas"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	tenant, err := s.Service.Store.Tenant(r.Context(), scope.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if tenant.Version != in.Version {
		s.fail(w, r, domain.ErrConflict)
		return
	}
	q, old := in.Quotas, tenant.Quotas
	if !q.Valid() || q.ConcurrentJobs > old.ConcurrentJobs || q.QueuedJobs > old.QueuedJobs || q.MemoryMB > old.MemoryMB || q.RequestsPerMinute > old.RequestsPerMinute {
		s.fail(w, r, domain.ErrInvalid)
		return
	}
	tenant.Quotas = q
	tenant.Version++
	if err = s.Service.Store.UpdateTenant(r.Context(), tenant); err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, tenant)
}
