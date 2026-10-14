package rediscache

import (
	"context"
	"healthcare/internal/framework/tenancy"
	"testing"
)

func TestKeysDoNotCollideAcrossTenantsOrSegments(t *testing.T) {
	ctx := func(id string) context.Context {
		return tenancy.With(context.Background(), tenancy.Scope{TenantID: id, Actor: "test"})
	}
	a, err := Key(ctx("a:b"), "dev", "patients", "id")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Key(ctx("a"), "dev", "b:patients", "id")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("cache namespace collision")
	}
	if _, err = Key(context.Background(), "dev", "patients", "id"); err == nil {
		t.Fatal("missing tenant context accepted")
	}
}
