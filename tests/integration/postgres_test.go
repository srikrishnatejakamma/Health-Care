package integration

import (
	"context"
	"errors"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/tenancy"
	"healthcare/internal/framework/workflow"
	"healthcare/migrations"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgresRLSConstraintsAndDurability(t *testing.T) {
	adminURL, runtimeURL := os.Getenv("PG_TEST_ADMIN_URL"), os.Getenv("PG_TEST_RUNTIME_URL")
	if adminURL == "" || runtimeURL == "" {
		t.Skip("set PG_TEST_ADMIN_URL and PG_TEST_RUNTIME_URL to enable real PostgreSQL isolation tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, migrations.Initial); err != nil {
		t.Fatal(err)
	}
	if unsafe, err := database.OpenPostgres(ctx, adminURL, 2); err == nil {
		unsafe.Close()
		t.Fatal("superuser runtime was accepted")
	}
	store, err := database.OpenPostgres(ctx, runtimeURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	prefix := "test-" + domain.ID()
	a, b := prefix+"-a", prefix+"-b"
	for _, id := range []string{a, b} {
		tenant := domain.Tenant{ID: id, Name: "Synthetic integration test", Status: "active", Timezone: "UTC", Region: "test", IsolationMode: "shared_schema", Version: 1, Quotas: domain.DefaultQuotas(), CreatedAt: time.Now().UTC()}
		if err = store.CreateTenant(ctx, tenant, domain.Membership{TenantID: id, UserID: "test-user", Roles: []string{"tenant_admin"}, Facilities: []string{"*"}, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	scope := func(id string) context.Context {
		return tenancy.With(ctx, tenancy.Scope{TenantID: id, Actor: "test-user", Roles: []string{"tenant_admin"}, Facilities: []string{"*"}, CorrelationID: domain.ID()})
	}
	facility := domain.Facility{ID: domain.ID(), TenantID: a, Name: "Synthetic", Capacity: 1}
	patient := domain.Patient{ID: domain.ID(), TenantID: a, FacilityID: facility.ID, ExternalID: "same-external", Name: "Synthetic", BirthDate: "1990-01-01"}
	if err = store.Atomic(scope(a), func(tx database.Tx) error {
		if err := database.Put(tx, "facilities", facility.ID, facility); err != nil {
			return err
		}
		return database.Put(tx, "patients", patient.ID, patient)
	}); err != nil {
		t.Fatal(err)
	}
	if err = store.View(scope(b), func(tx database.Tx) error {
		_, err := database.Get[domain.Patient](tx, "patients", patient.ID)
		return err
	}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("repository leaked cross-tenant record", err)
	}
	bad := patient
	bad.TenantID = b
	bad.ID = domain.ID()
	if err = store.Atomic(scope(b), func(tx database.Tx) error { return database.Put(tx, "patients", bad.ID, bad) }); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("cross-tenant facility relationship was not rejected", err)
	}
	copyPatient := patient
	copyPatient.ID = domain.ID()
	if err = store.Atomic(scope(a), func(tx database.Tx) error { return database.Put(tx, "patients", copyPatient.ID, copyPatient) }); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("tenant external identifier uniqueness was not enforced", err)
	}
	rollback := errors.New("rollback marker")
	copyPatient.ExternalID = "rollback"
	if err = store.Atomic(scope(a), func(tx database.Tx) error {
		if err := database.Put(tx, "patients", copyPatient.ID, copyPatient); err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if err = store.View(scope(a), func(tx database.Tx) error { _, err := tx.Get("patients", copyPatient.ID); return err }); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("rolled-back patient persisted")
	}
	// Deliberately omit repository predicates and prove database-level isolation.
	raw, err := pgx.Connect(ctx, runtimeURL)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close(ctx)
	var count int
	if err = raw.QueryRow(ctx, `SELECT count(*) FROM resources WHERE tenant_id=ANY($1)`, []string{a, b}).Scan(&count); err != nil || count != 0 {
		t.Fatal("missing-context RLS did not fail closed", count, err)
	}
	tx, err := raw.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, b); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM resources WHERE tenant_id=$1`, a).Scan(&count); err != nil || count != 0 {
		t.Fatal("RLS exposed another tenant", count, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = raw.QueryRow(ctx, `SELECT count(*) FROM resources WHERE tenant_id=$1`, a).Scan(&count); err != nil || count != 0 {
		t.Fatal("transaction-local tenant context leaked after commit", count, err)
	}
	// Two independent connection pools simulate two worker processes.
	other, err := database.OpenPostgres(ctx, runtimeURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	svc := application.New(store)
	payload, err := svc.CreateOrder(scope(a), "pg-order", application.OrderInput{PatientID: patient.ID, Medication: "Synthetic", Prescription: "synthetic-reference"})
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("missing order")
	}
	runners := []*workflow.Runner{workflow.New(svc, 2, 64), workflow.New(application.New(other), 2, 64)}
	jobs := make(chan domain.Job, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for _, runner := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := runner.Claim(tenancy.Worker(ctx, a))
			if err == nil {
				jobs <- j
			} else {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(jobs)
	close(failures)
	if len(jobs) != 1 {
		t.Fatal("distributed claim dispatched duplicate executions", len(jobs))
	}
	for err := range failures {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatal(err)
		}
	}
	j := <-jobs
	if err = runners[0].Execute(tenancy.Worker(ctx, a), j); err != nil {
		t.Fatal(err)
	}
	if err = other.View(scope(a), func(tx database.Tx) error {
		v, err := database.Get[domain.Patient](tx, "patients", patient.ID)
		if err == nil && v.ID != patient.ID {
			t.Fatal("durability mismatch")
		}
		return err
	}); err != nil {
		t.Fatal("second process could not read persisted data", err)
	}
}
