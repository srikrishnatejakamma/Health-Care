package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func OpenPostgres(ctx context.Context, dsn string, maxConnections int32) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConnections
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 5 * time.Minute
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Postgres, error) { p.Close(); return nil, err }
	if err = p.Ping(ctx); err != nil {
		return fail(err)
	}
	var unsafe bool
	err = p.QueryRow(ctx, `SELECT r.rolsuper OR r.rolbypassrls OR c.relowner=r.oid OR NOT c.relrowsecurity OR NOT c.relforcerowsecurity FROM pg_roles r CROSS JOIN pg_class c WHERE r.rolname=current_user AND c.oid='resources'::regclass`).Scan(&unsafe)
	if err != nil {
		return fail(err)
	}
	if unsafe {
		return fail(fmt.Errorf("runtime database role must be a non-owner without superuser/BYPASSRLS privileges; FORCE RLS must be enabled"))
	}
	return &Postgres{pool: p}, nil
}
func pgError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return domain.ErrConflict
		case "23503", "23514":
			return domain.ErrInvalid
		case "42501":
			return domain.ErrForbidden
		}
	}
	return err
}

type controlActorKey struct{}

func ControlActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, controlActorKey{}, actor)
}
func actor(ctx context.Context) string {
	a, _ := ctx.Value(controlActorKey{}).(string)
	if a == "" {
		a = "system"
	}
	return a
}
func controlAudit(ctx context.Context, tx pgx.Tx, t, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO control_audit(id,tenant_id,action,actor) VALUES($1,$2,$3,$4)`, domain.ID(), t, action, actor(ctx))
	return err
}
func (p *Postgres) CreateTenant(ctx context.Context, t domain.Tenant, m domain.Membership) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(t)
	u, _ := json.Marshal(m)
	if _, err = tx.Exec(ctx, `INSERT INTO tenants(id,version,data) VALUES($1,$2,$3)`, t.ID, t.Version, b); err != nil {
		return pgError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(tenant_id,user_id,data) VALUES($1,$2,$3)`, t.ID, m.UserID, u); err != nil {
		return pgError(err)
	}
	if err = controlAudit(ctx, tx, t.ID, "tenant.created"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) Tenant(ctx context.Context, id string) (domain.Tenant, error) {
	var b []byte
	var t domain.Tenant
	err := p.pool.QueryRow(ctx, `SELECT data FROM tenants WHERE id=$1`, id).Scan(&b)
	if err != nil {
		return t, pgError(err)
	}
	err = json.Unmarshal(b, &t)
	return t, err
}
func (p *Postgres) Tenants(ctx context.Context, after string, limit int) ([]domain.Tenant, error) {
	rows, err := p.pool.Query(ctx, `SELECT data FROM tenants WHERE id>$1 ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Tenant{}
	for rows.Next() {
		var b []byte
		var t domain.Tenant
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (p *Postgres) UpdateTenant(ctx context.Context, t domain.Tenant) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(t)
	r, err := tx.Exec(ctx, `UPDATE tenants SET data=$2,version=$3 WHERE id=$1 AND version=$3-1`, t.ID, b, t.Version)
	if err != nil {
		return pgError(err)
	}
	if r.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	if err = controlAudit(ctx, tx, t.ID, "tenant.updated"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) Membership(ctx context.Context, t, u string) (domain.Membership, error) {
	var b []byte
	var m domain.Membership
	err := p.pool.QueryRow(ctx, `SELECT data FROM memberships WHERE tenant_id=$1 AND user_id=$2`, t, u).Scan(&b)
	if err != nil {
		return m, pgError(err)
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
func (p *Postgres) PutMembership(ctx context.Context, m domain.Membership) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(m)
	_, err = tx.Exec(ctx, `INSERT INTO memberships(tenant_id,user_id,data) VALUES($1,$2,$3) ON CONFLICT(tenant_id,user_id) DO UPDATE SET data=EXCLUDED.data`, m.TenantID, m.UserID, b)
	if err != nil {
		return pgError(err)
	}
	if err = controlAudit(ctx, tx, m.TenantID, "membership.updated"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) Atomic(ctx context.Context, fn func(Tx) error) error { return p.run(ctx, fn, false) }
func (p *Postgres) View(ctx context.Context, fn func(Tx) error) error   { return p.run(ctx, fn, true) }
func (p *Postgres) run(ctx context.Context, fn func(Tx) error, readOnly bool) error {
	s, err := tenancy.From(ctx)
	if err != nil {
		return err
	}
	options := pgx.TxOptions{}
	if readOnly {
		options.AccessMode = pgx.ReadOnly
	}
	tx, err := p.pool.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	// Local setting expires at COMMIT/ROLLBACK, so a reused connection cannot leak scope.
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true)`, s.TenantID); err != nil {
		return err
	}
	query := `SELECT data FROM tenants WHERE id=$1`
	if !readOnly {
		query += ` FOR UPDATE`
	}
	var b []byte
	var t domain.Tenant
	if err = tx.QueryRow(ctx, query, s.TenantID).Scan(&b); err != nil {
		return pgError(err)
	}
	if err = json.Unmarshal(b, &t); err != nil {
		return err
	}
	if t.Status != "active" {
		return domain.ErrInactive
	}
	if err = fn(&postgresTx{ctx: ctx, tx: tx, tenant: t, readOnly: readOnly}); err != nil {
		return err
	}
	return pgError(tx.Commit(ctx))
}
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Postgres) Close()                         { p.pool.Close() }

type postgresTx struct {
	ctx      context.Context
	tx       pgx.Tx
	tenant   domain.Tenant
	readOnly bool
}

func (t *postgresTx) Tenant() domain.Tenant { return t.tenant }
func (t *postgresTx) Get(kind, id string) (json.RawMessage, error) {
	var b []byte
	err := t.tx.QueryRow(t.ctx, `SELECT data FROM resources WHERE tenant_id=$1 AND kind=$2 AND id=$3`, t.tenant.ID, kind, id).Scan(&b)
	return b, pgError(err)
}
func (t *postgresTx) List(kind, after string, limit int) ([]json.RawMessage, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT data FROM resources WHERE tenant_id=$1 AND kind=$2 AND id>$3 ORDER BY id LIMIT $4`, t.tenant.ID, kind, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(b))
	}
	return out, rows.Err()
}
func (t *postgresTx) Put(kind, id string, b json.RawMessage) error {
	if t.readOnly {
		return domain.ErrForbidden
	}
	if err := validateRecord(t.tenant.ID, id, b); err != nil {
		return err
	}
	_, err := t.tx.Exec(t.ctx, `INSERT INTO resources(tenant_id,kind,id,data) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,kind,id) DO UPDATE SET data=EXCLUDED.data`, t.tenant.ID, kind, id, []byte(b))
	return pgError(err)
}
func (t *postgresTx) Delete(kind, id string) error {
	if t.readOnly {
		return domain.ErrForbidden
	}
	_, err := t.tx.Exec(t.ctx, `DELETE FROM resources WHERE tenant_id=$1 AND kind=$2 AND id=$3`, t.tenant.ID, kind, id)
	return pgError(err)
}
