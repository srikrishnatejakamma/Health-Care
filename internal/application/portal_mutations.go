package application

import (
	"context"
	"encoding/json"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"strings"
	"time"
)

func (s *Service) UpdatePatient(ctx context.Context, key, id string, in PatientInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	birth, err := time.Parse("2006-01-02", in.BirthDate)
	if err != nil || birth.After(s.Now()) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || strings.TrimSpace(in.ExternalID) == "" || len(in.ExternalID) > 128 {
		return nil, invalid("name, external_id and valid birth_date required")
	}
	return s.mutate(ctx, "patient.update/"+id, key, in, func(tx database.Tx) (any, error) {
		p, err := database.Get[domain.Patient](tx, "patients", id)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(p.FacilityID) || !scope.Facility(in.FacilityID) {
			return nil, domain.ErrForbidden
		}
		if p.FacilityID != in.FacilityID {
			return nil, invalid("patient facility transfers require a separate care transfer workflow")
		}
		if err = database.Each[domain.Patient](tx, "patients", func(other domain.Patient) error {
			if other.ID != p.ID && other.ExternalID == in.ExternalID {
				return domain.ErrConflict
			}
			return nil
		}); err != nil {
			return nil, err
		}
		p.Name = in.Name
		p.ExternalID = in.ExternalID
		p.BirthDate = in.BirthDate
		if err = database.Put(tx, "patients", p.ID, p); err != nil {
			return nil, err
		}
		return p, s.Emit(ctx, tx, "patient.updated", p.ID)
	})
}
func (s *Service) CancelAppointment(ctx context.Context, key, id string) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, "appointment.cancel/"+id, key, struct{}{}, func(tx database.Tx) (any, error) {
		a, err := database.Get[domain.Appointment](tx, "appointments", id)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(a.FacilityID) {
			return nil, domain.ErrForbidden
		}
		o, err := database.Get[domain.Order](tx, "orders", a.OrderID)
		if err != nil {
			return nil, err
		}
		w, err := database.Get[domain.Workflow](tx, "workflows", o.WorkflowID)
		if err != nil {
			return nil, err
		}
		if a.Status != "confirmed" || w.Status != "scheduled" || w.Outcomes["administration"] == "completed" {
			return nil, domain.ErrConflict
		}
		a.Status = "cancelled"
		w.Status = "ready_to_schedule"
		w.UpdatedAt = s.Now()
		w.Checkpoints = append(w.Checkpoints, "appointment_cancelled")
		o.Status = w.Status
		if err = database.Put(tx, "appointments", a.ID, a); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "orders", o.ID, o); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "workflows", w.ID, w); err != nil {
			return nil, err
		}
		return a, s.Emit(ctx, tx, "appointment.cancelled", a.ID, w.Version)
	})
}
