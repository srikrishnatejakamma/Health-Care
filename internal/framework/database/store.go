package database

import (
	"context"
	"encoding/json"
	"healthcare/internal/domain"
)

// All data-plane access requires a validated tenancy.Scope in context.
// Atomic serializes writes for one tenant and commits data, jobs, outbox and audit together.
type Store interface {
	CreateTenant(context.Context, domain.Tenant, domain.Membership) error
	Tenant(context.Context, string) (domain.Tenant, error)
	Tenants(context.Context, string, int) ([]domain.Tenant, error)
	UpdateTenant(context.Context, domain.Tenant) error
	Membership(context.Context, string, string) (domain.Membership, error)
	PutMembership(context.Context, domain.Membership) error
	Atomic(context.Context, func(Tx) error) error
	View(context.Context, func(Tx) error) error
	Ping(context.Context) error
	Close()
}
type Tx interface {
	Tenant() domain.Tenant
	Get(kind, id string) (json.RawMessage, error)
	List(kind, after string, limit int) ([]json.RawMessage, error)
	Put(kind, id string, data json.RawMessage) error
	Delete(kind, id string) error
}

func Get[T any](tx Tx, kind, id string) (T, error) {
	var v T
	b, err := tx.Get(kind, id)
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(b, &v)
	return v, err
}
func Put[T any](tx Tx, kind, id string, v T) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return tx.Put(kind, id, b)
}
func List[T any](tx Tx, kind, after string, limit int) ([]T, error) {
	rows, err := tx.List(kind, after, limit)
	if err != nil {
		return nil, err
	}
	result := make([]T, 0, len(rows))
	for _, b := range rows {
		var v T
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

// Each bounds memory even when a tenant owns a large number of records.
func Each[T any](tx Tx, kind string, fn func(T) error) error {
	after := ""
	for {
		rows, err := tx.List(kind, after, 100)
		if err != nil {
			return err
		}
		for _, b := range rows {
			var v T
			if err = json.Unmarshal(b, &v); err != nil {
				return err
			}
			if err = fn(v); err != nil {
				return err
			}
			var id struct {
				ID      string `json:"id"`
				EventID string `json:"event_id"`
			}
			if err = json.Unmarshal(b, &id); err != nil {
				return err
			}
			after = id.ID
			if after == "" {
				after = id.EventID
			}
		}
		if len(rows) < 100 {
			return nil
		}
	}
}
