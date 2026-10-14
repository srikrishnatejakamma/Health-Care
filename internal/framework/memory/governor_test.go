package memory

import (
	"sync"
	"testing"
)

func TestReservationsAndIdempotentRelease(t *testing.T) {
	g := New(16)
	release, ok := g.Acquire("a", 8, 8)
	if !ok {
		t.Fatal("valid reservation rejected")
	}
	if _, ok = g.Acquire("a", 8, 8); ok {
		t.Fatal("tenant budget exceeded")
	}
	other, ok := g.Acquire("b", 8, 8)
	if !ok {
		t.Fatal("one tenant consumed another's budget")
	}
	if _, ok = g.Acquire("c", 8, 8); ok {
		t.Fatal("global budget exceeded")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); release() }()
	}
	wg.Wait()
	other()
	if g.Used() != 0 {
		t.Fatal("reservation was released more than once")
	}
}
