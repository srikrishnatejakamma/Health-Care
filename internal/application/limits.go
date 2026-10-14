package application

import (
	"context"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
)

// One shared fixed-window counter per tenant is consistent across API replicas and protocols.
func (s *Service) AdmitRequest(ctx context.Context) error {
	return s.Store.Atomic(ctx, func(tx database.Tx) error {
		type window struct {
			ID       string `json:"id"`
			TenantID string `json:"tenant_id"`
			Minute   int64  `json:"minute"`
			Count    int    `json:"count"`
		}
		v, err := database.Get[window](tx, "rate_limits", "api-window")
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		minute := s.Now().Unix() / 60
		if v.Minute != minute || v.ID == "" {
			v = window{ID: "api-window", TenantID: tx.Tenant().ID, Minute: minute}
		}
		if v.Count >= tx.Tenant().Quotas.RequestsPerMinute {
			return domain.ErrQuota
		}
		v.Count++
		return database.Put(tx, "rate_limits", v.ID, v)
	})
}
