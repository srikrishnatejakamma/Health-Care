package integration

import (
	"bufio"
	"context"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPortalOverviewRespectsTenantAndRoles(t *testing.T) {
	f := setup(t)
	prepare(t, f)
	north := call[application.Overview](t, f, "GET", "/api/v1/portal/overview?days=7", f.north, "", nil, 200)
	south := call[application.Overview](t, f, "GET", "/api/v1/portal/overview?days=7", f.south, "", nil, 200)
	if north.Counts["patients"] != 1 || north.Counts["active_orders"] != 1 || south.Counts["patients"] != 0 {
		t.Fatal("overview leaked or lost tenant counts", north, south)
	}
	if len(north.Series) != 7 {
		t.Fatal("overview date window invalid")
	}
	status, _ := f.call("GET", "/api/v1/portal/overview?days=999", f.north, "", nil)
	if status != 400 {
		t.Fatal("unbounded overview window accepted")
	}
	call[domain.Membership](t, f, "PUT", "/api/v1/memberships/portal-auditor", f.north, "", map[string]any{"roles": []string{"auditor"}, "facilities": []string{"*"}, "active": true}, 200)
	token, err := f.auth.Sign("portal-auditor", "north-health", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	limited := call[application.Overview](t, f, "GET", "/api/v1/portal/overview", token, "", nil, 200)
	if _, ok := limited.Counts["patients"]; ok {
		t.Fatal("auditor received clinical counts")
	}
	if _, ok := limited.Counts["queued_jobs"]; !ok {
		t.Fatal("auditor operational counts missing")
	}
}

func TestPatientUpdateAndAppointmentRelease(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	updated := call[domain.Patient](t, f, "PATCH", "/api/v1/patients/"+c.patient.ID, f.north, "edit-patient", application.PatientInput{FacilityID: c.facility.ID, ExternalID: c.patient.ExternalID, Name: "Updated Synthetic Patient", BirthDate: c.patient.BirthDate}, 200)
	if updated.Name != "Updated Synthetic Patient" || updated.TenantID != "north-health" {
		t.Fatal("patient update failed")
	}
	status, _ := f.call("PATCH", "/api/v1/patients/"+c.patient.ID, f.south, "cross", application.PatientInput{FacilityID: c.facility.ID, ExternalID: c.patient.ExternalID, Name: "Other", BirthDate: c.patient.BirthDate})
	if status != 404 {
		t.Fatal("cross-tenant update accepted")
	}
	approve(t, f, c.order.WorkflowID)
	start := time.Now().Add(time.Hour).UTC()
	a := call[domain.Appointment](t, f, "POST", "/api/v1/appointments", f.north, "portal-book", application.AppointmentInput{OrderID: c.order.ID, NurseID: c.staff.ID, ResourceID: c.resource.ID, StartsAt: start, EndsAt: start.Add(time.Hour)}, 201)
	cancelled := call[domain.Appointment](t, f, "POST", "/api/v1/appointments/"+a.ID+"/cancel", f.north, "portal-release", nil, 200)
	if cancelled.Status != "cancelled" {
		t.Fatal("reservation not released")
	}
	w := call[domain.Workflow](t, f, "GET", "/api/v1/workflows/"+c.order.WorkflowID, f.north, "", nil, 200)
	if w.Status != "ready_to_schedule" {
		t.Fatal("released therapy not eligible for rebooking")
	}
}

func TestTenantConfigurationCannotRaisePlatformCeilings(t *testing.T) {
	f := setup(t)
	tenant, err := f.store.Tenant(context.Background(), "north-health")
	if err != nil {
		t.Fatal(err)
	}
	higher := tenant.Quotas
	higher.ConcurrentJobs++
	status, _ := f.call("PATCH", "/api/v1/configuration", f.north, "", map[string]any{"version": tenant.Version, "quotas": higher})
	if status != 400 {
		t.Fatal("tenant raised assigned platform ceiling")
	}
	lower := tenant.Quotas
	lower.ConcurrentJobs = 1
	updated := call[domain.Tenant](t, f, "PATCH", "/api/v1/configuration", f.north, "", map[string]any{"version": tenant.Version, "quotas": lower}, 200)
	if updated.Version != tenant.Version+1 || updated.Quotas.ConcurrentJobs != 1 {
		t.Fatal("versioned budget update failed")
	}
	status, _ = f.call("PATCH", "/api/v1/configuration", f.north, "", map[string]any{"version": tenant.Version, "quotas": lower})
	if status != 409 {
		t.Fatal("stale budget write accepted")
	}
}

func TestAuthenticatedStreamContainsOnlyTenantRefreshHints(t *testing.T) {
	f := setup(t)
	server := httptest.NewServer(f.handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.north)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("stream authentication or content type invalid")
	}
	reader := bufio.NewReader(response.Body)
	event, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	data, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if event != "event: refresh\n" || !strings.Contains(data, `"tenant_id":"north-health"`) || strings.Contains(data, "patient") || strings.Contains(data, f.north) {
		t.Fatal("stream exposed unexpected data", event, data)
	}
	cancel()
}
