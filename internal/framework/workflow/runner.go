package workflow

import (
	"context"
	"errors"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/memory"
	"healthcare/internal/framework/tenancy"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

type Runner struct {
	Service   *application.Service
	Workers   int
	Poll      time.Duration
	Lease     time.Duration
	Memory    *memory.Governor
	Completed atomic.Uint64
	Failed    atomic.Uint64
	Logger    *slog.Logger
}

func New(s *application.Service, workers int, memoryMB int64) *Runner {
	return &Runner{Service: s, Workers: workers, Poll: 100 * time.Millisecond, Lease: 30 * time.Second, Memory: memory.New(memoryMB), Logger: slog.Default()}
}

// Round-robin tenant polling and one dispatch per visit prevent queue monopolization.
// Database claims and completion are fenced by a unique lease ID across replicas.
func (r *Runner) Run(ctx context.Context) {
	sem := make(chan struct{}, r.Workers)
	var wg sync.WaitGroup
	defer wg.Wait()
	ticker := time.NewTicker(r.Poll)
	defer ticker.Stop()
	cursor := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		tenants, err := r.Service.Store.Tenants(ctx, cursor, 1)
		if err != nil {
			r.Logger.Error("worker tenant lookup failed")
			continue
		}
		if len(tenants) == 0 {
			cursor = ""
			continue
		}
		t := tenants[0]
		cursor = t.ID
		if t.Status != "active" {
			continue
		}
		select {
		case sem <- struct{}{}:
		default:
			continue
		}
		release, ok := r.Memory.Acquire(t.ID, 8, t.Quotas.MemoryMB)
		if !ok {
			<-sem
			continue
		}
		tenantCtx := tenancy.Worker(ctx, t.ID)
		j, err := r.Claim(tenantCtx)
		if err != nil {
			release()
			<-sem
			if !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrQuota) && !errors.Is(err, domain.ErrInactive) {
				r.Logger.Error("worker claim failed", "tenant_id", t.ID)
			}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem; release() }()
			runCtx, cancel := context.WithTimeout(tenantCtx, r.Lease/2)
			defer cancel()
			if err := r.Execute(runCtx, j); err != nil {
				r.Failed.Add(1)
				retryCtx, retryCancel := context.WithTimeout(context.WithoutCancel(tenantCtx), 5*time.Second)
				defer retryCancel()
				if e := r.fail(retryCtx, j); e != nil && !errors.Is(e, domain.ErrInactive) {
					r.Logger.Error("worker retry checkpoint failed", "tenant_id", j.TenantID)
				}
				return
			}
			r.Completed.Add(1)
		}()
	}
}
func (r *Runner) Claim(ctx context.Context) (domain.Job, error) {
	var picked domain.Job
	now := r.Service.Now()
	err := r.Service.Store.Atomic(ctx, func(tx database.Tx) error {
		// Scheduling and job creation commit together. Missed intervals coalesce to one execution.
		created := 0
		err := database.Each[domain.Schedule](tx, "schedules", func(v domain.Schedule) error {
			if v.Paused || v.NextRun.After(now) || created >= 10 {
				return nil
			}
			if err := r.Service.Enqueue(tx, v.Kind, v.ID); err != nil {
				if errors.Is(err, domain.ErrQuota) {
					return nil
				}
				return err
			}
			next, err := application.NextRun(v.Expression, v.Timezone, now)
			if err != nil {
				return err
			}
			v.NextRun = next
			created++
			return database.Put(tx, "schedules", v.ID, v)
		})
		if err != nil {
			return err
		}
		running := 0
		err = database.Each[domain.Job](tx, "jobs", func(j domain.Job) error {
			if j.Status == "running" && j.LeaseUntil.After(now) {
				running++
				return nil
			}
			eligible := (j.Status == "queued" && !j.DueAt.After(now)) || (j.Status == "running" && !j.LeaseUntil.After(now))
			if !eligible {
				return nil
			}
			if j.Attempt >= j.MaxAttempts {
				j.Status = "dead"
				j.LeaseID = ""
				return database.Put(tx, "jobs", j.ID, j)
			}
			if picked.ID == "" || j.DueAt.Before(picked.DueAt) {
				picked = j
			}
			return nil
		})
		if err != nil {
			return err
		}
		if running >= tx.Tenant().Quotas.ConcurrentJobs {
			return domain.ErrQuota
		}
		if picked.ID == "" {
			return nil
		}
		picked.Status = "running"
		picked.Attempt++
		picked.LeaseID = domain.ID()
		picked.LeaseUntil = now.Add(r.Lease)
		return database.Put(tx, "jobs", picked.ID, picked)
	})
	if err == nil && picked.ID == "" {
		err = domain.ErrNotFound
	}
	return picked, err
}
func (r *Runner) Execute(ctx context.Context, j domain.Job) error {
	return r.Service.Store.Atomic(ctx, func(tx database.Tx) error {
		current, err := database.Get[domain.Job](tx, "jobs", j.ID)
		if err != nil {
			return err
		}
		if current.Status != "running" || current.LeaseID != j.LeaseID || !current.LeaseUntil.After(r.Service.Now()) {
			return domain.ErrConflict
		}
		switch current.Kind {
		case "workflow.advance":
			err = r.Service.Advance(ctx, tx, current.TargetID)
		case "reconciliation":
			err = database.Each[domain.Workflow](tx, "workflows", func(w domain.Workflow) error { return r.Service.Advance(ctx, tx, w.ID) })
		default:
			err = domain.ErrInvalid
		}
		if err != nil {
			return err
		}
		if !current.LeaseUntil.After(r.Service.Now()) {
			return domain.ErrConflict
		}
		current.Status = "completed"
		current.LeaseID = ""
		current.LastError = ""
		if err = database.Put(tx, "jobs", current.ID, current); err != nil {
			return err
		}
		return r.Service.Emit(ctx, tx, "job.completed", current.ID, current.Version)
	})
}
func (r *Runner) fail(ctx context.Context, j domain.Job) error {
	return r.Service.Store.Atomic(ctx, func(tx database.Tx) error {
		current, err := database.Get[domain.Job](tx, "jobs", j.ID)
		if err != nil {
			return err
		}
		if current.Status != "running" || current.LeaseID != j.LeaseID {
			return domain.ErrConflict
		}
		current.LeaseID = ""
		current.LastError = "handler_failed"
		if current.Attempt >= current.MaxAttempts {
			current.Status = "dead"
		} else {
			current.Status = "queued"
			delay := time.Second * time.Duration(1<<min(current.Attempt, 8))
			current.DueAt = r.Service.Now().Add(delay + time.Duration(rand.IntN(500))*time.Millisecond)
		}
		if err = database.Put(tx, "jobs", current.ID, current); err != nil {
			return err
		}
		return r.Service.Emit(ctx, tx, "job."+current.Status, current.ID, current.Version)
	})
}
