package resilience

import (
	"context"
	"errors"
	"healthcare/internal/framework/tenancy"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSafeRetriesAndTenantCircuitIsolation(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer srv.Close()
	c, err := New(srv.URL, 2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := func(id string) context.Context {
		return tenancy.With(context.Background(), tenancy.Scope{TenantID: id, Actor: "test"})
	}
	for i := 0; i < 3; i++ {
		if _, _, err = c.Do(ctx("a"), "POST", "/payer", "", "", nil); err == nil {
			t.Fatal("failure ignored")
		}
	}
	if calls.Load() != 3 {
		t.Fatal("unsafe POST was retried")
	}
	if _, _, err = c.Do(ctx("a"), "GET", "/payer", "", "", nil); !errors.Is(err, ErrCircuitOpen) {
		t.Fatal("failing tenant's circuit remained open to requests")
	}
	if _, _, err = c.Do(ctx("b"), "GET", "/payer", "", "", nil); err == nil || errors.Is(err, ErrCircuitOpen) {
		t.Fatal("unrelated tenant inherited circuit state")
	}
	if calls.Load() != 6 {
		t.Fatal("safe read was not retried")
	}
	if _, _, err = c.Do(ctx("b"), "GET", "//other-host", "", "", nil); err == nil {
		t.Fatal("untrusted absolute endpoint accepted")
	}
}
