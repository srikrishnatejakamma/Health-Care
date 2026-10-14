package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"healthcare/internal/adapters/rest"
	"healthcare/internal/application"
	"healthcare/internal/bootstrap"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/security"
	"healthcare/internal/framework/tenancy"
	"healthcare/internal/framework/workflow"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	store                  database.Store
	svc                    *application.Service
	runner                 *workflow.Runner
	auth                   *security.Auth
	handler                http.Handler
	north, south, platform string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	store := database.NewMemory()
	if err := bootstrap.Seed(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	return setupWith(t, store)
}
func setupWith(t *testing.T, store database.Store) *fixture {
	t.Helper()
	svc := application.New(store)
	auth := &security.Auth{Key: bytes.Repeat([]byte("x"), 32), Issuer: "test", Audience: "api", Store: store}
	runner := workflow.New(svc, 4, 128)
	f := &fixture{store: store, svc: svc, auth: auth, runner: runner}
	f.handler = rest.New(svc, auth, runner).Handler()
	var err error
	f.north, err = auth.Sign("north-admin", "north-health", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.south, err = auth.Sign("south-admin", "south-health", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.platform, err = auth.Sign("operator", "", true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) call(method, path, token, key string, body any, headers ...string) (int, []byte) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}
func call[T any](t *testing.T, f *fixture, method, path, token, key string, in any, status int) T {
	t.Helper()
	actual, b := f.call(method, path, token, key, in)
	if actual != status {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, status, actual, b)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func (f *fixture) drain(t *testing.T) {
	t.Helper()
	ctx := tenancy.Worker(context.Background(), "north-health")
	for i := 0; i < 100; i++ {
		j, err := f.runner.Claim(ctx)
		if errors.Is(err, domain.ErrNotFound) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = f.runner.Execute(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("worker did not drain")
}

type clinical struct {
	facility domain.Facility
	staff    domain.Staff
	resource domain.CareResource
	patient  domain.Patient
	order    domain.Order
}

func prepare(t *testing.T, f *fixture) clinical {
	t.Helper()
	c := clinical{}
	c.facility = call[domain.Facility](t, f, "POST", "/api/v1/facilities", f.north, "facility", application.FacilityInput{Name: "Test facility", Capacity: 2}, 201)
	c.staff = call[domain.Staff](t, f, "POST", "/api/v1/staff", f.north, "staff", application.StaffInput{FacilityID: c.facility.ID, Name: "Test nurse", Qualifications: []string{"infusion"}}, 201)
	c.resource = call[domain.CareResource](t, f, "POST", "/api/v1/care-resources", f.north, "resource", application.ResourceInput{FacilityID: c.facility.ID, Name: "Chair 1", Kind: "chair"}, 201)
	c.patient = call[domain.Patient](t, f, "POST", "/api/v1/patients", f.north, "patient", application.PatientInput{FacilityID: c.facility.ID, Name: "Synthetic Patient", ExternalID: "synthetic-1", BirthDate: "1990-01-01"}, 201)
	c.order = call[domain.Order](t, f, "POST", "/api/v1/orders", f.north, "order", application.OrderInput{PatientID: c.patient.ID, Medication: "Test medication", Prescription: "synthetic-prescription"}, 201)
	return c
}
func approve(t *testing.T, f *fixture, id string) {
	t.Helper()
	for step, outcome := range map[string]string{"insurance": "approved", "inventory": "available", "clearance": "approved"} {
		call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+id+"/outcomes", f.north, "approve-"+step+id, application.OutcomeInput{Step: step, Outcome: outcome}, 200)
	}
	f.drain(t)
}

func TestTherapyWorkflowEndToEnd(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	f.drain(t)
	w := call[domain.Workflow](t, f, "GET", "/api/v1/workflows/"+c.order.WorkflowID, f.north, "", nil, 200)
	if w.Status != "awaiting_approvals" {
		t.Fatal("API success must not imply insurance or clinical approval")
	}
	approve(t, f, w.ID)
	start := time.Now().Add(time.Hour).UTC()
	a := call[domain.Appointment](t, f, "POST", "/api/v1/appointments", f.north, "appointment", application.AppointmentInput{OrderID: c.order.ID, NurseID: c.staff.ID, ResourceID: c.resource.ID, StartsAt: start, EndsAt: start.Add(time.Hour)}, 201)
	if a.Status != "confirmed" {
		t.Fatal(a)
	}
	call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+w.ID+"/outcomes", f.north, "administer", application.OutcomeInput{Step: "administration", Outcome: "completed"}, 200)
	f.drain(t)
	w = call[domain.Workflow](t, f, "GET", "/api/v1/workflows/"+w.ID, f.north, "", nil, 200)
	if w.Status != "awaiting_billing" {
		t.Fatal(w.Status)
	}
	call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+w.ID+"/outcomes", f.north, "billing", application.OutcomeInput{Step: "billing", Outcome: "submitted"}, 200)
	f.drain(t)
	w = call[domain.Workflow](t, f, "GET", "/api/v1/workflows/"+w.ID, f.north, "", nil, 200)
	if w.Status != "completed" || w.DefinitionVersion != 1 || len(w.Checkpoints) < 5 {
		t.Fatalf("unexpected workflow: %+v", w)
	}
	events := call[application.Page](t, f, "GET", "/api/v1/events", f.north, "", nil, 200)
	if len(events.Items) < 10 {
		t.Fatal("missing transactional events")
	}
}

func TestTenantIsolationAndSelectors(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	for _, path := range []string{"/api/v1/patients/" + c.patient.ID, "/api/v1/orders/" + c.order.ID, "/api/v1/workflows/" + c.order.WorkflowID, "/fhir/Patient/" + c.patient.ID} {
		status, b := f.call("GET", path, f.south, "", nil)
		if status != 404 {
			t.Fatalf("cross-tenant access: %d %s", status, b)
		}
	}
	p := call[application.Page](t, f, "GET", "/api/v1/patients", f.south, "", nil, 200)
	if len(p.Items) != 0 {
		t.Fatal("cross-tenant list leak")
	}
	status, _ := f.call("GET", "/api/v1/patients", f.north, "", nil, "X-Tenant-ID", "south-health")
	if status != 403 {
		t.Fatal("spoofed tenant header accepted")
	}
	status, _ = f.call("GET", "/api/v1/patients?tenant_id=south-health", f.north, "", nil, "X-Tenant-ID", "north-health")
	if status != 403 {
		t.Fatal("conflicting selectors accepted")
	}
	status, _ = f.call("GET", "/api/v1/patients", f.platform, "", nil, "X-Tenant-ID", "north-health")
	if status != 403 {
		t.Fatal("platform operator received routine clinical access")
	}
	status, _ = f.call("POST", "/api/v1/patients", f.north, "spoof", map[string]any{"tenant_id": "south-health", "name": "Synthetic"})
	if status != 400 {
		t.Fatal("tenant ownership in caller payload accepted")
	}
	token, err := f.auth.Sign("north-admin", "", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	status, _ = f.call("GET", "/api/v1/me", token, "", nil)
	if status != 403 {
		t.Fatal("missing tenant selection accepted")
	}
	status, _ = f.call("GET", "/api/v1/me", token, "", nil, "X-Tenant-ID", "north-health")
	if status != 200 {
		t.Fatal("authorized tenant selection rejected")
	}
}

func TestTenantScopedIdempotencyAndRollback(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	in := application.OrderInput{PatientID: c.patient.ID, Medication: "Test medication", Prescription: "synthetic-prescription"}
	again := call[domain.Order](t, f, "POST", "/api/v1/orders", f.north, "order", in, 201)
	if again.ID != c.order.ID {
		t.Fatal("idempotency did not preserve result")
	}
	in.Medication = "different"
	status, _ := f.call("POST", "/api/v1/orders", f.north, "order", in)
	if status != 409 {
		t.Fatal("idempotency key reuse with different payload accepted")
	}
	tnt, err := f.store.Tenant(context.Background(), "north-health")
	if err != nil {
		t.Fatal(err)
	}
	tnt.Version++
	tnt.Quotas.QueuedJobs = 1
	if err = f.store.UpdateTenant(context.Background(), tnt); err != nil {
		t.Fatal(err)
	}
	status, _ = f.call("POST", "/api/v1/orders", f.north, "order-2", in)
	if status != 429 {
		t.Fatal("expected queue backpressure")
	}
	p := call[application.Page](t, f, "GET", "/api/v1/orders", f.north, "", nil, 200)
	if len(p.Items) != 1 {
		t.Fatal("partial order persisted after queue admission failed")
	}
	southFacility := call[domain.Facility](t, f, "POST", "/api/v1/facilities", f.south, "facility", application.FacilityInput{Name: "South", Capacity: 1}, 201)
	southPatient := call[domain.Patient](t, f, "POST", "/api/v1/patients", f.south, "patient", application.PatientInput{FacilityID: southFacility.ID, Name: "Synthetic South", ExternalID: "synthetic-1", BirthDate: "1990-01-01"}, 201)
	if southPatient.TenantID != "south-health" {
		t.Fatal("tenant identifiers collided")
	}
}

func TestAppointmentConflictIsAtomic(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	o2 := call[domain.Order](t, f, "POST", "/api/v1/orders", f.north, "order2", application.OrderInput{PatientID: c.patient.ID, Medication: "Test medication", Prescription: "ref-2"}, 201)
	approve(t, f, c.order.WorkflowID)
	approve(t, f, o2.WorkflowID)
	start := time.Now().Add(time.Hour).UTC()
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, id := range []string{c.order.ID, o2.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := f.call("POST", "/api/v1/appointments", f.north, "booking-"+id, application.AppointmentInput{OrderID: id, NurseID: c.staff.ID, ResourceID: c.resource.ID, StartsAt: start, EndsAt: start.Add(time.Hour)})
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("conflicting reservations: %v", counts)
	}
}

func TestCancellationCompensatesReservation(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	approve(t, f, c.order.WorkflowID)
	start := time.Now().Add(time.Hour).UTC()
	a := call[domain.Appointment](t, f, "POST", "/api/v1/appointments", f.north, "appointment", application.AppointmentInput{OrderID: c.order.ID, NurseID: c.staff.ID, ResourceID: c.resource.ID, StartsAt: start, EndsAt: start.Add(time.Hour)}, 201)
	w := call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+c.order.WorkflowID+"/cancel", f.north, "cancel", nil, 200)
	if w.Status != "cancelled" {
		t.Fatal(w.Status)
	}
	a = call[domain.Appointment](t, f, "GET", "/api/v1/appointments/"+a.ID, f.north, "", nil, 200)
	if a.Status != "cancelled" {
		t.Fatal("reservation was not compensated")
	}
}

func TestWorkerLeaseFencingAndTenantSuspension(t *testing.T) {
	f := setup(t)
	now := time.Now().UTC()
	f.svc.Now = func() time.Time { return now }
	prepare(t, f)
	ctx := tenancy.Worker(context.Background(), "north-health")
	first, err := f.runner.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(f.runner.Lease + time.Second)
	second, err := f.runner.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.LeaseID == second.LeaseID {
		t.Fatal("lease was not recovered with a new fencing token")
	}
	if err = f.runner.Execute(ctx, first); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale worker was not fenced", err)
	}
	tenant, err := f.store.Tenant(ctx, "north-health")
	if err != nil {
		t.Fatal(err)
	}
	tenant.Status = "suspended"
	tenant.Version++
	if err = f.store.UpdateTenant(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	if err = f.runner.Execute(ctx, second); !errors.Is(err, domain.ErrInactive) {
		t.Fatal("suspended tenant executed work", err)
	}
	status, _ := f.call("GET", "/api/v1/patients", f.north, "", nil)
	if status != 403 {
		t.Fatal("suspension did not block API")
	}
	tenant.Status = "active"
	tenant.Version++
	if err = f.store.UpdateTenant(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	if err = f.runner.Execute(ctx, second); err != nil {
		t.Fatal(err)
	}
}

func TestFacilityAndRoleScopeAndRevocation(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	f2 := call[domain.Facility](t, f, "POST", "/api/v1/facilities", f.north, "other", application.FacilityInput{Name: "Other", Capacity: 1}, 201)
	call[domain.Membership](t, f, "PUT", "/api/v1/memberships/clinician", f.north, "", map[string]any{"roles": []string{"clinical"}, "facilities": []string{f2.ID}, "active": true}, 200)
	token, err := f.auth.Sign("clinician", "north-health", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	status, _ := f.call("GET", "/api/v1/patients/"+c.patient.ID, token, "", nil)
	if status != 404 {
		t.Fatal("facility scope was ignored")
	}
	p := call[application.Page](t, f, "GET", "/api/v1/patients", token, "", nil, 200)
	if len(p.Items) != 0 {
		t.Fatal("facility list leak")
	}
	status, _ = f.call("POST", "/api/v1/facilities", token, "denied", application.FacilityInput{Name: "Denied", Capacity: 1})
	if status != 403 {
		t.Fatal("role scope ignored")
	}
	call[domain.Membership](t, f, "PUT", "/api/v1/memberships/clinician", f.north, "", map[string]any{"roles": []string{"clinical"}, "facilities": []string{f2.ID}, "active": false}, 200)
	status, _ = f.call("GET", "/api/v1/me", token, "", nil)
	if status != 403 {
		t.Fatal("membership revocation ignored")
	}
}

func TestUnauthenticatedAndMalformedRequests(t *testing.T) {
	f := setup(t)
	for _, token := range []string{"", "garbage"} {
		status, _ := f.call("GET", "/api/v1/patients", token, "", nil)
		if status != 401 {
			t.Fatal("unauthenticated request accepted")
		}
	}
	r := httptest.NewRequest("POST", "/api/v1/facilities", io.NopCloser(bytes.NewBufferString(`{"name":"A","capacity":1} {"name":"B"}`)))
	r.Header.Set("Authorization", "Bearer "+f.north)
	r.Header.Set("Idempotency-Key", "bad")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("multiple JSON bodies accepted")
	}
}

func TestScheduledExecutionAndPause(t *testing.T) {
	f := setup(t)
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	f.svc.Now = func() time.Time { return now }
	v := call[domain.Schedule](t, f, "POST", "/api/v1/schedules", f.north, "schedule", application.ScheduleInput{Expression: "@every 1m", Kind: "reconciliation"}, 201)
	ctx := tenancy.Worker(context.Background(), "north-health")
	if _, err := f.runner.Claim(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("schedule executed too early", err)
	}
	now = v.NextRun
	job, err := f.runner.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.Kind != "reconciliation" {
		t.Fatal(job.Kind)
	}
	if err = f.runner.Execute(ctx, job); err != nil {
		t.Fatal(err)
	}
	call[domain.Schedule](t, f, "POST", "/api/v1/schedules/"+v.ID+"/actions", f.north, "pause", application.ScheduleAction{Action: "pause"}, 200)
	now = now.Add(time.Hour)
	if _, err = f.runner.Claim(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("paused schedule dispatched", err)
	}
}

func TestTenantRateLimit(t *testing.T) {
	f := setup(t)
	tenant, err := f.store.Tenant(context.Background(), "north-health")
	if err != nil {
		t.Fatal(err)
	}
	tenant.Quotas.RequestsPerMinute = 1
	tenant.Version++
	if err = f.store.UpdateTenant(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	status, _ := f.call("GET", "/api/v1/me", f.north, "", nil)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = f.call("GET", "/api/v1/me", f.north, "", nil)
	if status != 429 {
		t.Fatal("tenant rate limit ignored")
	}
	status, _ = f.call("GET", "/api/v1/me", f.south, "", nil)
	if status != 200 {
		t.Fatal("one tenant exhausted another tenant's request budget")
	}
}

func TestWorkflowConfigurationRemainsPinned(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	f.drain(t)
	tenant, err := f.store.Tenant(context.Background(), "north-health")
	if err != nil {
		t.Fatal(err)
	}
	tenant.Version++
	if err = f.store.UpdateTenant(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+c.order.WorkflowID+"/outcomes", f.north, "pinned", application.OutcomeInput{Step: "insurance", Outcome: "approved"}, 200)
	ctx := tenancy.Worker(context.Background(), "north-health")
	job, err := f.runner.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.Version != 1 {
		t.Fatal("active workflow silently changed configuration version", job.Version)
	}
	if err = f.runner.Execute(ctx, job); err != nil {
		t.Fatal(err)
	}
	page := call[application.Page](t, f, "GET", "/api/v1/events", f.north, "", nil, 200)
	found := false
	for _, b := range page.Items {
		var e domain.Event
		if err = json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Type == "workflow.outcome_recorded" {
			found = true
			if e.ConfigurationVersion != 1 {
				t.Fatal("workflow event lost original configuration version")
			}
		}
	}
	if !found {
		t.Fatal("missing outcome event")
	}
}
func TestAdministrationCannotBeCancelledBeforeWorkerAdvances(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	approve(t, f, c.order.WorkflowID)
	start := time.Now().Add(time.Hour).UTC()
	call[domain.Appointment](t, f, "POST", "/api/v1/appointments", f.north, "appointment", application.AppointmentInput{OrderID: c.order.ID, NurseID: c.staff.ID, ResourceID: c.resource.ID, StartsAt: start, EndsAt: start.Add(time.Hour)}, 201)
	call[domain.Workflow](t, f, "POST", "/api/v1/workflows/"+c.order.WorkflowID+"/outcomes", f.north, "administer", application.OutcomeInput{Step: "administration", Outcome: "completed"}, 200)
	status, _ := f.call("POST", "/api/v1/workflows/"+c.order.WorkflowID+"/cancel", f.north, "cancel", nil)
	if status != 409 {
		t.Fatal("recorded administration was cancelled before billing")
	}
	f.drain(t)
	w := call[domain.Workflow](t, f, "GET", "/api/v1/workflows/"+c.order.WorkflowID, f.north, "", nil, 200)
	if w.Status != "awaiting_billing" {
		t.Fatal(w.Status)
	}
}

func TestClinicalReadsAreAudited(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	call[domain.Patient](t, f, "GET", "/api/v1/patients/"+c.patient.ID, f.north, "", nil, 200)
	page := call[application.Page](t, f, "GET", "/api/v1/audit", f.north, "", nil, 200)
	for _, b := range page.Items {
		var a domain.Audit
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}
		if a.Action == "patients.read" && a.ResourceID == c.patient.ID {
			if a.Actor != "north-admin" || a.CorrelationID == "" {
				t.Fatal("incomplete access audit")
			}
			return
		}
	}
	t.Fatal("clinical read returned without an access audit")
}
