package memory

import "sync"

// Reservations are admission estimates, not hard Go heap limits.
type Governor struct {
	mu          sync.Mutex
	limit, used int64
	tenants     map[string]int64
}

func New(limitMB int64) *Governor { return &Governor{limit: limitMB, tenants: map[string]int64{}} }
func (g *Governor) Acquire(tenant string, mb, tenantLimit int64) (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if mb <= 0 || g.used+mb > g.limit || g.tenants[tenant]+mb > tenantLimit {
		return nil, false
	}
	g.used += mb
	g.tenants[tenant] += mb
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.used -= mb
			g.tenants[tenant] -= mb
			if g.tenants[tenant] == 0 {
				delete(g.tenants, tenant)
			}
		})
	}, true
}
func (g *Governor) Used() int64 { g.mu.Lock(); defer g.mu.Unlock(); return g.used }
