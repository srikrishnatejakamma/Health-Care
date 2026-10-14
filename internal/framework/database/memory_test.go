package database

import (
	"context"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"testing"
)

func TestRepositoryFailsClosedAndRollsBack(t *testing.T) {
	m := NewMemory()
	ctx := context.Background()
	tenant := domain.Tenant{ID: "a", Status: "active", Version: 1, Quotas: domain.DefaultQuotas()}
	if err := m.CreateTenant(ctx, tenant, domain.Membership{TenantID: "a", UserID: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := m.View(ctx, func(Tx) error { return nil }); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("missing scope accepted")
	}
	ctx = tenancy.With(ctx, tenancy.Scope{TenantID: "a", Actor: "user"})
	marker := errors.New("rollback")
	err := m.Atomic(ctx, func(tx Tx) error {
		if err := Put(tx, "facilities", "f", domain.Facility{ID: "f", TenantID: "a", Capacity: 1}); err != nil {
			return err
		}
		return marker
	})
	if !errors.Is(err, marker) {
		t.Fatal(err)
	}
	err = m.View(ctx, func(tx Tx) error { _, err := tx.Get("facilities", "f"); return err })
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("rollback failed")
	}
	err = m.Atomic(ctx, func(tx Tx) error {
		return Put(tx, "facilities", "f", domain.Facility{ID: "f", TenantID: "b", Capacity: 1})
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("cross-tenant write accepted")
	}
}
