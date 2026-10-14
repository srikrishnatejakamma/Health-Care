package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/security"
	"healthcare/internal/framework/tenancy"
	"healthcare/internal/framework/workflow"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Server struct {
	Service  *application.Service
	Auth     *security.Auth
	Runner   *workflow.Runner
	Logger   *slog.Logger
	Requests atomic.Uint64
}

func New(s *application.Service, auth *security.Auth, runner *workflow.Runner) *Server {
	return &Server{Service: s, Auth: auth, Runner: runner, Logger: slog.Default()}
}

type claimsKey struct{}

func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"service": "healthcare-go-framework", "api": "/api/v1", "health": "/healthz"})
	})
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.Service.Store.Ping(ctx) != nil {
			write(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	m.HandleFunc("GET /metrics", s.metrics)
	platform := func(pattern string, h http.HandlerFunc) { m.Handle(pattern, s.authenticate(true, h)) }
	tenant := func(pattern string, h http.HandlerFunc) { m.Handle(pattern, s.authenticate(false, h)) }
	platform("POST /api/v1/platform/tenants", s.createTenant)
	platform("GET /api/v1/platform/tenants", s.listTenants)
	platform("GET /api/v1/platform/tenants/{id}", s.getTenant)
	platform("PATCH /api/v1/platform/tenants/{id}", s.updateTenant)
	tenant("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		scope, _ := tenancy.From(r.Context())
		t, err := s.Service.Store.Tenant(r.Context(), scope.TenantID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		write(w, 200, map[string]any{"tenant": t, "user_id": scope.Actor, "roles": scope.Roles, "facilities": scope.Facilities})
	})
	tenant("PUT /api/v1/memberships/{user}", s.putMembership)
	tenant("GET /api/v1/portal/overview", s.overview)
	tenant("GET /api/v1/operations", s.operations)
	tenant("GET /api/v1/stream", s.stream)
	tenant("POST /api/v1/facilities", create(s, s.Service.CreateFacility))
	tenant("POST /api/v1/staff", create(s, s.Service.CreateStaff))
	tenant("POST /api/v1/care-resources", create(s, s.Service.CreateResource))
	tenant("POST /api/v1/patients", create(s, s.Service.CreatePatient))
	tenant("PATCH /api/v1/patients/{id}",func(w http.ResponseWriter,r *http.Request){var in application.PatientInput;if !s.decode(w,r,&in){return};b,err:=s.Service.UpdatePatient(r.Context(),r.Header.Get("Idempotency-Key"),r.PathValue("id"),in);s.result(w,r,b,err,200)})
	tenant("POST /api/v1/orders", create(s, s.Service.CreateOrder))
	tenant("POST /api/v1/appointments", create(s, s.Service.CreateAppointment))
	tenant("POST /api/v1/appointments/{id}/cancel",func(w http.ResponseWriter,r *http.Request){b,err:=s.Service.CancelAppointment(r.Context(),r.Header.Get("Idempotency-Key"),r.PathValue("id"));s.result(w,r,b,err,200)})
	tenant("POST /api/v1/schedules", create(s, s.Service.CreateSchedule))
	tenant("POST /api/v1/workflows/{id}/outcomes", func(w http.ResponseWriter, r *http.Request) {
		var in application.OutcomeInput
		if !s.decode(w, r, &in) {
			return
		}
		b, err := s.Service.Outcome(r.Context(), r.Header.Get("Idempotency-Key"), r.PathValue("id"), in)
		s.result(w, r, b, err, 200)
	})
	tenant("POST /api/v1/workflows/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		b, err := s.Service.CancelWorkflow(r.Context(), r.Header.Get("Idempotency-Key"), r.PathValue("id"))
		s.result(w, r, b, err, 200)
	})
	tenant("POST /api/v1/schedules/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		var in application.ScheduleAction
		if !s.decode(w, r, &in) {
			return
		}
		b, err := s.Service.ScheduleAction(r.Context(), r.Header.Get("Idempotency-Key"), r.PathValue("id"), in)
		s.result(w, r, b, err, 200)
	})
	tenant("POST /api/v1/jobs/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		var in application.ScheduleAction
		if !s.decode(w, r, &in) {
			return
		}
		b, err := s.Service.JobAction(r.Context(), r.Header.Get("Idempotency-Key"), r.PathValue("id"), in)
		s.result(w, r, b, err, 200)
	})
	for path, kind := range map[string]string{"facilities": "facilities", "staff": "staff", "care-resources": "care_resources", "patients": "patients", "orders": "orders", "workflows": "workflows", "appointments": "appointments", "jobs": "jobs", "schedules": "schedules", "audit": "audit", "events": "outbox"} {
		tenant("GET /api/v1/"+path, func(w http.ResponseWriter, r *http.Request) {
			limit, err := pageLimit(r)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			p, err := s.Service.List(r.Context(), kind, r.URL.Query().Get("after"), limit)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			write(w, 200, p)
		})
		tenant("GET /api/v1/"+path+"/{id}", func(w http.ResponseWriter, r *http.Request) {
			b, err := s.Service.Get(r.Context(), kind, r.PathValue("id"))
			s.result(w, r, b, err, 200)
		})
	}
	tenant("GET /fhir/Patient/{id}", s.fhirPatient)
	tenant("GET /fhir/Patient", s.fhirPatients)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Requests.Add(1)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		id := domain.ID()
		w.Header().Set("X-Correlation-ID", id)
		defer func() {
			if recover() != nil {
				s.Logger.Error("request panic", "correlation_id", id)
				write(w, 500, map[string]string{"error": "internal_error", "correlation_id": id})
			}
		}()
		m.ServeHTTP(w, r)
	})
}
func create[T any](s *Server, fn func(context.Context, string, T) (json.RawMessage, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in T
		if !s.decode(w, r, &in) {
			return
		}
		b, err := fn(r.Context(), r.Header.Get("Idempotency-Key"), in)
		s.result(w, r, b, err, 201)
	}
}
func (s *Server) authenticate(platform bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			write(w, 401, map[string]string{"error": "authentication_required"})
			return
		}
		c, err := s.Auth.Validate(parts[1])
		if err != nil {
			write(w, 401, map[string]string{"error": "invalid_token"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey{}, c)
		ctx = database.ControlActor(ctx, c.Subject)
		if platform {
			if !c.PlatformAdmin {
				write(w, 403, map[string]string{"error": "access_denied"})
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		selected := r.Header.Get("X-Tenant-ID")
		if len(r.Header.Values("X-Tenant-ID")) > 1 {
			write(w, 403, map[string]string{"error": "conflicting_tenant_selectors"})
			return
		}
		if values, ok := r.URL.Query()["tenant_id"]; ok {
			if len(values) != 1 || values[0] == "" || (selected != "" && selected != values[0]) {
				write(w, 403, map[string]string{"error": "conflicting_tenant_selectors"})
				return
			}
			selected = values[0]
		}
		ctx, err = s.Auth.Resolve(ctx, c, selected, w.Header().Get("X-Correlation-ID"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if err = s.admit(ctx); err != nil {
			if errors.Is(err, domain.ErrQuota) {
				w.Header().Set("Retry-After", "60")
			}
			s.fail(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Server) admit(ctx context.Context) error {
	return s.Service.AdmitRequest(ctx)
}
func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		s.fail(w, r, fmt.Errorf("%w: invalid JSON or unknown fields", domain.ErrInvalid))
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		s.fail(w, r, fmt.Errorf("%w: one JSON object required", domain.ErrInvalid))
		return false
	}
	return true
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) result(w http.ResponseWriter, r *http.Request, b json.RawMessage, err error, status int) {
	if err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, status, b)
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	message := "internal_error"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status = 400
		message = err.Error()
	case errors.Is(err, domain.ErrNotFound):
		status = 404
		message = "not_found"
	case errors.Is(err, domain.ErrConflict):
		status = 409
		message = "resource_conflict"
	case errors.Is(err, domain.ErrForbidden):
		status = 403
		message = "access_denied"
	case errors.Is(err, domain.ErrInactive):
		status = 403
		message = "tenant_inactive"
	case errors.Is(err, domain.ErrQuota):
		status = 429
		message = "quota_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		status = 504
		message = "deadline_exceeded"
	}
	if status == 500 {
		s.Logger.Error("request failed", "correlation_id", w.Header().Get("X-Correlation-ID"))
	}
	write(w, status, map[string]string{"error": message, "correlation_id": w.Header().Get("X-Correlation-ID")})
}
func pageLimit(r *http.Request) (int, error) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 50, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 100 {
		return 0, domain.ErrInvalid
	}
	return n, nil
}

type tenantInput struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	AdminUserID string         `json:"admin_user_id"`
	Timezone    string         `json:"timezone"`
	Region      string         `json:"region"`
	Quotas      *domain.Quotas `json:"quotas,omitempty"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	var in tenantInput
	if !s.decode(w, r, &in) {
		return
	}
	if !identifier.MatchString(in.ID) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || strings.TrimSpace(in.AdminUserID) == "" || len(in.AdminUserID) > 200 || strings.TrimSpace(in.Region) == "" || len(in.Region) > 80 {
		s.fail(w, r, domain.ErrInvalid)
		return
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		s.fail(w, r, domain.ErrInvalid)
		return
	}
	q := domain.DefaultQuotas()
	if in.Quotas != nil {
		q = *in.Quotas
	}
	if !q.Valid() {
		s.fail(w, r, domain.ErrInvalid)
		return
	}
	t := domain.Tenant{ID: in.ID, Name: in.Name, Status: "active", Timezone: in.Timezone, Region: in.Region, IsolationMode: "shared_schema", Version: 1, Quotas: q, CreatedAt: s.Service.Now()}
	m := domain.Membership{TenantID: t.ID, UserID: in.AdminUserID, Roles: []string{"tenant_admin"}, Facilities: []string{"*"}, Active: true}
	err := s.Service.Store.CreateTenant(r.Context(), t, m)
	if errors.Is(err, domain.ErrConflict) {
		existing, e := s.Service.Store.Tenant(r.Context(), t.ID)
		admin, me := s.Service.Store.Membership(r.Context(), t.ID, in.AdminUserID)
		if e == nil && me == nil && admin.Active && existing.Name == t.Name && existing.Timezone == t.Timezone && existing.Region == t.Region && existing.Quotas == q {
			write(w, 200, existing)
			return
		}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 201, t)
}
func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	limit, err := pageLimit(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	list, err := s.Service.Store.Tenants(r.Context(), r.URL.Query().Get("after"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	cursor := ""
	if len(list) == limit {
		cursor = list[len(list)-1].ID
	}
	write(w, 200, map[string]any{"items": list, "next_cursor": cursor})
}
func (s *Server) getTenant(w http.ResponseWriter, r *http.Request) {
	t, err := s.Service.Store.Tenant(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, t)
}

type tenantUpdate struct {
	Status  string         `json:"status,omitempty"`
	Quotas  *domain.Quotas `json:"quotas,omitempty"`
	Version int            `json:"version"`
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request) {
	var in tenantUpdate
	if !s.decode(w, r, &in) {
		return
	}
	t, err := s.Service.Store.Tenant(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if in.Version != t.Version {
		s.fail(w, r, domain.ErrConflict)
		return
	}
	if in.Status != "" && in.Status != t.Status {
		allowed := map[string][]string{"active": {"suspended", "offboarding"}, "suspended": {"active", "offboarding"}, "offboarding": {"archived"}}
		ok := false
		for _, v := range allowed[t.Status] {
			if v == in.Status {
				ok = true
			}
		}
		if !ok {
			s.fail(w, r, domain.ErrInvalid)
			return
		}
		t.Status = in.Status
	}
	if in.Quotas != nil {
		if !in.Quotas.Valid() {
			s.fail(w, r, domain.ErrInvalid)
			return
		}
		t.Quotas = *in.Quotas
	}
	t.Version++
	if err = s.Service.Store.UpdateTenant(r.Context(), t); err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, t)
}

type membershipInput struct {
	Roles      []string `json:"roles"`
	Facilities []string `json:"facilities"`
	Active     bool     `json:"active"`
}

func (s *Server) putMembership(w http.ResponseWriter, r *http.Request) {
	scope, _ := tenancy.From(r.Context())
	if !scope.Has("tenant_admin") {
		s.fail(w, r, domain.ErrForbidden)
		return
	}
	var in membershipInput
	if !s.decode(w, r, &in) {
		return
	}
	if len(in.Roles) < 1 || len(in.Roles) > 6 || len(in.Facilities) > 100 || len(r.PathValue("user")) > 200 {
		s.fail(w, r, domain.ErrInvalid)
		return
	}
	for _, role := range in.Roles {
		switch role {
		case "tenant_admin", "clinical", "billing", "auditor", "integration":
		default:
			s.fail(w, r, domain.ErrInvalid)
			return
		}
	}
	err := s.Service.Store.View(r.Context(), func(tx database.Tx) error {
		for _, id := range in.Facilities {
			if id == "*" {
				continue
			}
			if _, err := database.Get[domain.Facility](tx, "facilities", id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	m := domain.Membership{TenantID: scope.TenantID, UserID: r.PathValue("user"), Roles: in.Roles, Facilities: in.Facilities, Active: in.Active}
	if err = s.Service.Store.PutMembership(r.Context(), m); err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, m)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(w, "# TYPE healthcare_http_requests_total counter\nhealthcare_http_requests_total %d\n# TYPE healthcare_goroutines gauge\nhealthcare_goroutines %d\n# TYPE healthcare_heap_bytes gauge\nhealthcare_heap_bytes %d\n", s.Requests.Load(), runtime.NumGoroutine(), m.HeapAlloc)
	if s.Runner != nil {
		fmt.Fprintf(w, "# TYPE healthcare_jobs_completed_total counter\nhealthcare_jobs_completed_total %d\n# TYPE healthcare_jobs_failed_total counter\nhealthcare_jobs_failed_total %d\n# TYPE healthcare_memory_reserved_mb gauge\nhealthcare_memory_reserved_mb %d\n", s.Runner.Completed.Load(), s.Runner.Failed.Load(), s.Runner.Memory.Used())
	}
}

func fhir(p domain.Patient) map[string]any {
	return map[string]any{"resourceType": "Patient", "id": p.ID, "identifier": []any{map[string]string{"system": "urn:healthcare:tenant:" + p.TenantID, "value": p.ExternalID}}, "name": []any{map[string]string{"text": p.Name}}, "birthDate": p.BirthDate}
}
func (s *Server) fhirPatient(w http.ResponseWriter, r *http.Request) {
	b, err := s.Service.Get(r.Context(), "patients", r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var p domain.Patient
	if err = json.Unmarshal(b, &p); err != nil {
		s.fail(w, r, err)
		return
	}
	write(w, 200, fhir(p))
}
func (s *Server) fhirPatients(w http.ResponseWriter, r *http.Request) {
	limit, err := pageLimit(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page, err := s.Service.List(r.Context(), "patients", r.URL.Query().Get("after"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entries := []any{}
	for _, b := range page.Items {
		var p domain.Patient
		if err = json.Unmarshal(b, &p); err != nil {
			s.fail(w, r, err)
			return
		}
		entries = append(entries, map[string]any{"resource": fhir(p)})
	}
	bundle := map[string]any{"resourceType": "Bundle", "type": "searchset", "entry": entries}
	if page.NextCursor != "" {
		bundle["link"] = []any{map[string]string{"relation": "next", "url": "/fhir/Patient?after=" + page.NextCursor + "&limit=" + strconv.Itoa(limit)}}
	}
	write(w, 200, bundle)
}
