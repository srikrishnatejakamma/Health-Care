package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/tenancy"
	"slices"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type Service struct {
	Store database.Store
	Now   func() time.Time
}

func New(store database.Store) *Service {
	return &Service{Store: store, Now: func() time.Time { return time.Now().UTC() }}
}
func invalid(message string) error { return fmt.Errorf("%w: %s", domain.ErrInvalid, message) }
func authorize(ctx context.Context, roles ...string) (tenancy.Scope, error) {
	s, err := tenancy.From(ctx)
	if err != nil {
		return s, err
	}
	if !s.Has(roles...) {
		return s, domain.ErrForbidden
	}
	return s, nil
}
func (s *Service) Emit(ctx context.Context, tx database.Tx, action, id string) error {
	scope, err := tenancy.From(ctx)
	if err != nil {
		return err
	}
	now := s.Now()
	a := domain.Audit{ID: domain.ID(), TenantID: scope.TenantID, Actor: scope.Actor, Action: action, ResourceID: id, CorrelationID: scope.CorrelationID, At: now}
	if err = database.Put(tx, "audit", a.ID, a); err != nil {
		return err
	}
	e := domain.Event{ID: domain.ID(), TenantID: scope.TenantID, Type: action, AggregateID: id, SchemaVersion: 1, ConfigurationVersion: tx.Tenant().Version, CorrelationID: scope.CorrelationID, OccurredAt: now}
	return database.Put(tx, "outbox", e.ID, e)
}

type idempotency struct {
	ID       string          `json:"id"`
	TenantID string          `json:"tenant_id"`
	Hash     string          `json:"hash"`
	Response json.RawMessage `json:"response"`
}

// Mutations, idempotency responses, audit records and events share one transaction.
func (s *Service) mutate(ctx context.Context, op, key string, input any, fn func(database.Tx) (any, error)) (json.RawMessage, error) {
	scope, err := tenancy.From(ctx)
	if err != nil {
		return nil, err
	}
	if len(key) < 1 || len(key) > 128 {
		return nil, invalid("Idempotency-Key must contain 1 to 128 characters")
	}
	b, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	requestHash := sha256.Sum256(b)
	keyHash := sha256.Sum256([]byte(op + "/" + key))
	id := hex.EncodeToString(keyHash[:])
	hash := hex.EncodeToString(requestHash[:])
	var response json.RawMessage
	kind := "idempotency"
	if strings.HasPrefix(op, "integration.") { kind = "inbox" }
	err = s.Store.Atomic(ctx, func(tx database.Tx) error {
		prior, err := database.Get[idempotency](tx, kind, id)
		if err == nil {
			if prior.Hash != hash {
				return domain.ErrConflict
			}
			var resource struct {
				FacilityID string `json:"facility_id"`
			}
			if err = json.Unmarshal(prior.Response, &resource); err != nil {
				return err
			}
			if resource.FacilityID != "" && !scope.Facility(resource.FacilityID) {
				return domain.ErrForbidden
			}
			response = prior.Response
			return nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		result, err := fn(tx)
		if err != nil {
			return err
		}
		response, err = json.Marshal(result)
		if err != nil {
			return err
		}
		return database.Put(tx, kind, id, idempotency{ID: id, TenantID: scope.TenantID, Hash: hash, Response: response})
	})
	return response, err
}

type FacilityInput struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}

func (s *Service) CreateFacility(ctx context.Context, key string, in FacilityInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || in.Capacity < 1 || in.Capacity > 1000 {
		return nil, invalid("name and capacity between 1 and 1000 required")
	}
	return s.mutate(ctx, "facility.create", key, in, func(tx database.Tx) (any, error) {
		v := domain.Facility{ID: domain.ID(), TenantID: scope.TenantID, Name: in.Name, Capacity: in.Capacity}
		if err := database.Put(tx, "facilities", v.ID, v); err != nil {
			return nil, err
		}
		return v, s.Emit(ctx, tx, "facility.created", v.ID)
	})
}

type StaffInput struct {
	FacilityID     string   `json:"facility_id"`
	Name           string   `json:"name"`
	Qualifications []string `json:"qualifications"`
}

func (s *Service) CreateStaff(ctx context.Context, key string, in StaffInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || len(in.Qualifications) > 20 {
		return nil, invalid("valid staff name and qualifications required")
	}
	return s.mutate(ctx, "staff.create", key, in, func(tx database.Tx) (any, error) {
		if _, err := database.Get[domain.Facility](tx, "facilities", in.FacilityID); err != nil {
			return nil, err
		}
		v := domain.Staff{ID: domain.ID(), TenantID: scope.TenantID, FacilityID: in.FacilityID, Name: in.Name, Qualifications: in.Qualifications, Active: true}
		if err := database.Put(tx, "staff", v.ID, v); err != nil {
			return nil, err
		}
		return v, s.Emit(ctx, tx, "staff.created", v.ID)
	})
}

type ResourceInput struct {
	FacilityID string `json:"facility_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
}

func (s *Service) CreateResource(ctx context.Context, key string, in ResourceInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || (in.Kind != "chair" && in.Kind != "room") {
		return nil, invalid("name and kind chair or room required")
	}
	return s.mutate(ctx, "resource.create", key, in, func(tx database.Tx) (any, error) {
		if _, err := database.Get[domain.Facility](tx, "facilities", in.FacilityID); err != nil {
			return nil, err
		}
		v := domain.CareResource{ID: domain.ID(), TenantID: scope.TenantID, FacilityID: in.FacilityID, Name: in.Name, Kind: in.Kind}
		if err := database.Put(tx, "care_resources", v.ID, v); err != nil {
			return nil, err
		}
		return v, s.Emit(ctx, tx, "resource.created", v.ID)
	})
}

type PatientInput struct {
	FacilityID string `json:"facility_id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	BirthDate  string `json:"birth_date"`
}

func (s *Service) CreatePatient(ctx context.Context, key string, in PatientInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	birth, err := time.Parse("2006-01-02", in.BirthDate)
	if err != nil || birth.After(s.Now()) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 || strings.TrimSpace(in.ExternalID) == "" || len(in.ExternalID) > 128 {
		return nil, invalid("name, external_id and valid birth_date required")
	}
	if !scope.Facility(in.FacilityID) {
		return nil, domain.ErrForbidden
	}
	return s.mutate(ctx, "patient.create", key, in, func(tx database.Tx) (any, error) {
		if _, err := database.Get[domain.Facility](tx, "facilities", in.FacilityID); err != nil {
			return nil, err
		}
		if err := database.Each[domain.Patient](tx, "patients", func(p domain.Patient) error {
			if p.ExternalID == in.ExternalID {
				return domain.ErrConflict
			}
			return nil
		}); err != nil {
			return nil, err
		}
		p := domain.Patient{ID: domain.ID(), TenantID: scope.TenantID, FacilityID: in.FacilityID, ExternalID: in.ExternalID, Name: in.Name, BirthDate: in.BirthDate, CreatedAt: s.Now()}
		if err := database.Put(tx, "patients", p.ID, p); err != nil {
			return nil, err
		}
		return p, s.Emit(ctx, tx, "patient.created", p.ID)
	})
}

type OrderInput struct {
	PatientID    string `json:"patient_id"`
	Medication   string `json:"medication"`
	Prescription string `json:"prescription"`
}

func (s *Service) CreateOrder(ctx context.Context, key string, in OrderInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Medication) == "" || len(in.Medication) > 200 || strings.TrimSpace(in.Prescription) == "" || len(in.Prescription) > 2000 {
		return nil, invalid("medication and prescription reference required")
	}
	return s.mutate(ctx, "order.create", key, in, func(tx database.Tx) (any, error) {
		p, err := database.Get[domain.Patient](tx, "patients", in.PatientID)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(p.FacilityID) {
			return nil, domain.ErrForbidden
		}
		o := domain.Order{ID: domain.ID(), TenantID: scope.TenantID, PatientID: p.ID, FacilityID: p.FacilityID, Medication: in.Medication, Prescription: in.Prescription, Status: "awaiting_approvals", WorkflowID: domain.ID(), CreatedAt: s.Now()}
		w := domain.Workflow{ID: o.WorkflowID, TenantID: scope.TenantID, FacilityID: o.FacilityID, OrderID: o.ID, Status: o.Status, Version: tx.Tenant().Version, DefinitionVersion: 1, Outcomes: map[string]string{}, Checkpoints: []string{"patient_and_prescription_reference_validated"}, UpdatedAt: s.Now()}
		if err = database.Put(tx, "orders", o.ID, o); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "workflows", w.ID, w); err != nil {
			return nil, err
		}
		if err = s.Enqueue(tx, "workflow.advance", w.ID); err != nil {
			return nil, err
		}
		return o, s.Emit(ctx, tx, "therapy.order_created", o.ID)
	})
}
func (s *Service) Enqueue(tx database.Tx, kind, target string) error {
	count := 0
	duplicate := false
	err := database.Each[domain.Job](tx, "jobs", func(j domain.Job) error {
		if j.Status == "queued" || j.Status == "running" {
			count++
			if j.Kind == kind && j.TargetID == target {
				duplicate = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if duplicate && kind == "workflow.advance" {
		return nil
	}
	if count >= tx.Tenant().Quotas.QueuedJobs {
		return domain.ErrQuota
	}
	j := domain.Job{ID: domain.ID(), TenantID: tx.Tenant().ID, Kind: kind, TargetID: target, Status: "queued", DueAt: s.Now(), MaxAttempts: 5, MemoryMB: 8, Version: tx.Tenant().Version}
	return database.Put(tx, "jobs", j.ID, j)
}

type OutcomeInput struct {
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
}

func (s *Service) Outcome(ctx context.Context, key, id string, in OutcomeInput) (json.RawMessage, error) {
	return s.recordOutcome(ctx, key, id, in, false)
}
func (s *Service) IntegrationOutcome(ctx context.Context, key, id string, in OutcomeInput) (json.RawMessage, error) {
	return s.recordOutcome(ctx, key, id, in, true)
}
func (s *Service) recordOutcome(ctx context.Context, key, id string, in OutcomeInput, inbox bool) (json.RawMessage, error) {
	scope, err := tenancy.From(ctx)
	if err != nil {
		return nil, err
	}
	var allowed bool
	switch in.Step {
	case "insurance", "clearance":
		allowed = in.Outcome == "approved" || in.Outcome == "rejected"
	case "inventory":
		allowed = in.Outcome == "available" || in.Outcome == "unavailable"
	case "administration":
		allowed = in.Outcome == "completed"
	case "billing":
		allowed = in.Outcome == "submitted"
	default:
		return nil, invalid("unsupported workflow step")
	}
	if !allowed {
		return nil, invalid("unsupported outcome")
	}
	if in.Step == "billing" {
		if !scope.Has("tenant_admin", "billing", "integration") {
			return nil, domain.ErrForbidden
		}
	} else if in.Step == "insurance" || in.Step == "inventory" {
		if !scope.Has("tenant_admin", "clinical", "integration") {
			return nil, domain.ErrForbidden
		}
	} else if !scope.Has("tenant_admin", "clinical") {
		return nil, domain.ErrForbidden
	}
	op := "workflow.outcome/"+id
	if inbox { op = "integration.outcome" }
	input := struct { WorkflowID string `json:"workflow_id"`; OutcomeInput }{id, in}
	return s.mutate(ctx, op, key, input, func(tx database.Tx) (any, error) {
		w, err := database.Get[domain.Workflow](tx, "workflows", id)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(w.FacilityID) {
			return nil, domain.ErrForbidden
		}
		expected := "awaiting_approvals"
		if in.Step == "administration" {
			expected = "scheduled"
		}
		if in.Step == "billing" {
			expected = "awaiting_billing"
		}
		if w.Status != expected {
			return nil, domain.ErrConflict
		}
		if prior, ok := w.Outcomes[in.Step]; ok && prior != in.Outcome {
			return nil, domain.ErrConflict
		}
		w.Outcomes[in.Step] = in.Outcome
		w.UpdatedAt = s.Now()
		if err = database.Put(tx, "workflows", w.ID, w); err != nil {
			return nil, err
		}
		if err = s.Enqueue(tx, "workflow.advance", w.ID); err != nil {
			return nil, err
		}
		return w, s.Emit(ctx, tx, "workflow.outcome_recorded", w.ID)
	})
}

// Advance is deterministic and runs in the same transaction as the worker's fenced completion.
// External clinical outcomes must be explicitly recorded; successful HTTP calls never imply approval.
func (s *Service) Advance(ctx context.Context, tx database.Tx, id string) error {
	w, err := database.Get[domain.Workflow](tx, "workflows", id)
	if err != nil {
		return err
	}
	old := w.Status
	switch w.Status {
	case "awaiting_approvals":
		if w.Outcomes["insurance"] == "rejected" || w.Outcomes["inventory"] == "unavailable" || w.Outcomes["clearance"] == "rejected" {
			w.Status = "rejected"
		} else if w.Outcomes["insurance"] == "approved" && w.Outcomes["inventory"] == "available" && w.Outcomes["clearance"] == "approved" {
			w.Status = "ready_to_schedule"
		}
	case "scheduled":
		if w.Outcomes["administration"] == "completed" {
			w.Status = "awaiting_billing"
			if err = database.Each[domain.Appointment](tx, "appointments", func(a domain.Appointment) error {
				if a.OrderID == w.OrderID && a.Status == "confirmed" {
					a.Status = "completed"
					return database.Put(tx, "appointments", a.ID, a)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	case "awaiting_billing":
		if w.Outcomes["billing"] == "submitted" {
			w.Status = "completed"
		}
	}
	if old == w.Status {
		return nil
	}
	w.UpdatedAt = s.Now()
	w.Checkpoints = append(w.Checkpoints, w.Status)
	o, err := database.Get[domain.Order](tx, "orders", w.OrderID)
	if err != nil {
		return err
	}
	o.Status = w.Status
	if err = database.Put(tx, "orders", o.ID, o); err != nil {
		return err
	}
	if err = database.Put(tx, "workflows", w.ID, w); err != nil {
		return err
	}
	return s.Emit(ctx, tx, "workflow."+w.Status, w.ID)
}

type AppointmentInput struct {
	OrderID    string    `json:"order_id"`
	NurseID    string    `json:"nurse_id"`
	ResourceID string    `json:"resource_id"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
}

func (s *Service) CreateAppointment(ctx context.Context, key string, in AppointmentInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	if !in.EndsAt.After(in.StartsAt) || in.EndsAt.Sub(in.StartsAt) > 24*time.Hour || in.StartsAt.Before(s.Now()) {
		return nil, invalid("future appointment between 0 and 24 hours required")
	}
	return s.mutate(ctx, "appointment.create", key, in, func(tx database.Tx) (any, error) {
		o, err := database.Get[domain.Order](tx, "orders", in.OrderID)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(o.FacilityID) {
			return nil, domain.ErrForbidden
		}
		w, err := database.Get[domain.Workflow](tx, "workflows", o.WorkflowID)
		if err != nil {
			return nil, err
		}
		if w.Status != "ready_to_schedule" {
			return nil, domain.ErrConflict
		}
		f, err := database.Get[domain.Facility](tx, "facilities", o.FacilityID)
		if err != nil {
			return nil, err
		}
		n, err := database.Get[domain.Staff](tx, "staff", in.NurseID)
		if err != nil {
			return nil, err
		}
		r, err := database.Get[domain.CareResource](tx, "care_resources", in.ResourceID)
		if err != nil {
			return nil, err
		}
		if n.FacilityID != f.ID || r.FacilityID != f.ID || !n.Active || !slices.Contains(n.Qualifications, "infusion") {
			return nil, invalid("active infusion-qualified nurse and resource must belong to the order facility")
		}
		// Tenant writes serialize this check with reservations, including across API replicas.
		overlaps := 0
		err = database.Each[domain.Appointment](tx, "appointments", func(a domain.Appointment) error {
			if a.Status != "confirmed" {
				return nil
			}
			if a.OrderID == o.ID {
				return domain.ErrConflict
			}
			if a.StartsAt.Before(in.EndsAt) && in.StartsAt.Before(a.EndsAt) {
				if a.NurseID == n.ID || a.ResourceID == r.ID || a.PatientID == o.PatientID {
					return domain.ErrConflict
				}
				if a.FacilityID == f.ID {
					overlaps++
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if overlaps >= f.Capacity {
			return nil, domain.ErrConflict
		}
		a := domain.Appointment{ID: domain.ID(), TenantID: scope.TenantID, PatientID: o.PatientID, FacilityID: f.ID, OrderID: o.ID, NurseID: n.ID, ResourceID: r.ID, StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(), Status: "confirmed"}
		w.Status = "scheduled"
		w.UpdatedAt = s.Now()
		w.Checkpoints = append(w.Checkpoints, "scheduled")
		o.Status = w.Status
		if err = database.Put(tx, "appointments", a.ID, a); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "workflows", w.ID, w); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "orders", o.ID, o); err != nil {
			return nil, err
		}
		return a, s.Emit(ctx, tx, "appointment.confirmed", a.ID)
	})
}
func (s *Service) CancelWorkflow(ctx context.Context, key, id string) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin", "clinical")
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, "workflow.cancel/"+id, key, struct{}{}, func(tx database.Tx) (any, error) {
		w, err := database.Get[domain.Workflow](tx, "workflows", id)
		if err != nil {
			return nil, err
		}
		if !scope.Facility(w.FacilityID) {
			return nil, domain.ErrForbidden
		}
		if w.Status == "completed" || w.Status == "awaiting_billing" {
			return nil, domain.ErrConflict
		}
		if err = database.Each[domain.Appointment](tx, "appointments", func(a domain.Appointment) error {
			if a.OrderID == w.OrderID && a.Status == "confirmed" {
				a.Status = "cancelled"
				return database.Put(tx, "appointments", a.ID, a)
			}
			return nil
		}); err != nil {
			return nil, err
		}
		w.Status = "cancelled"
		w.UpdatedAt = s.Now()
		w.Checkpoints = append(w.Checkpoints, "reservations_compensated")
		o, err := database.Get[domain.Order](tx, "orders", w.OrderID)
		if err != nil {
			return nil, err
		}
		o.Status = w.Status
		if err = database.Put(tx, "orders", o.ID, o); err != nil {
			return nil, err
		}
		if err = database.Put(tx, "workflows", w.ID, w); err != nil {
			return nil, err
		}
		return w, s.Emit(ctx, tx, "workflow.cancelled", w.ID)
	})
}

type ScheduleInput struct {
	Expression string `json:"expression"`
	Kind       string `json:"kind"`
}

func NextRun(expression, zone string, now time.Time) (time.Time, error) {
	if strings.Contains(expression, "TZ=") {
		return time.Time{}, invalid("timezone overrides in cron expressions are not permitted")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, invalid("invalid timezone")
	}
	schedule, err := cron.ParseStandard(expression)
	if err != nil {
		return time.Time{}, invalid("invalid cron expression")
	}
	next := schedule.Next(now.In(loc))
	if next.IsZero() || next.Sub(now) < time.Second {
		return time.Time{}, invalid("schedule must have a future execution at least one second away")
	}
	return next.UTC(), nil
}
func (s *Service) CreateSchedule(ctx context.Context, key string, in ScheduleInput) (json.RawMessage, error) {
	scope, err := authorize(ctx, "tenant_admin")
	if err != nil {
		return nil, err
	}
	if in.Kind != "reconciliation" {
		return nil, invalid("only reconciliation schedules are supported")
	}
	return s.mutate(ctx, "schedule.create", key, in, func(tx database.Tx) (any, error) {
		next, err := NextRun(in.Expression, tx.Tenant().Timezone, s.Now())
		if err != nil {
			return nil, err
		}
		v := domain.Schedule{ID: domain.ID(), TenantID: scope.TenantID, Expression: in.Expression, Timezone: tx.Tenant().Timezone, Kind: in.Kind, NextRun: next, Version: 1}
		if err = database.Put(tx, "schedules", v.ID, v); err != nil {
			return nil, err
		}
		return v, s.Emit(ctx, tx, "schedule.created", v.ID)
	})
}

type ScheduleAction struct {
	Action string `json:"action"`
}

func (s *Service) ScheduleAction(ctx context.Context, key, id string, in ScheduleAction) (json.RawMessage, error) {
	if _, err := authorize(ctx, "tenant_admin"); err != nil {
		return nil, err
	}
	return s.mutate(ctx, "schedule.action/"+id, key, in, func(tx database.Tx) (any, error) {
		v, err := database.Get[domain.Schedule](tx, "schedules", id)
		if err != nil {
			return nil, err
		}
		switch in.Action {
		case "pause":
			v.Paused = true
		case "resume":
			v.Paused = false
			v.NextRun, err = NextRun(v.Expression, v.Timezone, s.Now())
			if err != nil {
				return nil, err
			}
		case "trigger":
			if err = s.Enqueue(tx, v.Kind, v.ID); err != nil {
				return nil, err
			}
		default:
			return nil, invalid("action must be pause, resume or trigger")
		}
		v.Version++
		if err = database.Put(tx, "schedules", v.ID, v); err != nil {
			return nil, err
		}
		return v, s.Emit(ctx, tx, "schedule."+in.Action, v.ID)
	})
}
func (s *Service) JobAction(ctx context.Context, key, id string, in ScheduleAction) (json.RawMessage, error) {
	if _, err := authorize(ctx, "tenant_admin"); err != nil {
		return nil, err
	}
	return s.mutate(ctx, "job.action/"+id, key, in, func(tx database.Tx) (any, error) {
		j, err := database.Get[domain.Job](tx, "jobs", id)
		if err != nil {
			return nil, err
		}
		switch in.Action {
		case "cancel":
			if j.Status == "completed" {
				return nil, domain.ErrConflict
			}
			j.Status = "cancelled"
			j.LeaseID = ""
		case "retry":
			if j.Status != "dead" && j.Status != "cancelled" {
				return nil, domain.ErrConflict
			}
			j.Status = "queued"
			j.Attempt = 0
			j.DueAt = s.Now()
			j.LeaseID = ""
			j.LastError = ""
		default:
			return nil, invalid("action must be cancel or retry")
		}
		if err = database.Put(tx, "jobs", j.ID, j); err != nil {
			return nil, err
		}
		return j, s.Emit(ctx, tx, "job."+in.Action, j.ID)
	})
}

var readRoles = map[string][]string{
	"facilities": {"tenant_admin", "clinical", "billing", "auditor", "integration"}, "staff": {"tenant_admin", "clinical"}, "care_resources": {"tenant_admin", "clinical"}, "patients": {"tenant_admin", "clinical"}, "orders": {"tenant_admin", "clinical", "billing", "integration"}, "workflows": {"tenant_admin", "clinical", "billing", "integration"}, "appointments": {"tenant_admin", "clinical"}, "jobs": {"tenant_admin", "auditor"}, "schedules": {"tenant_admin", "auditor"}, "audit": {"tenant_admin", "auditor"}, "outbox": {"tenant_admin", "auditor"},
}

func readable(scope tenancy.Scope, kind string, b json.RawMessage) bool {
	var r struct {
		ID         string `json:"id"`
		FacilityID string `json:"facility_id"`
	}
	if json.Unmarshal(b, &r) != nil {
		return false
	}
	if kind == "facilities" {
		return scope.Facility(r.ID)
	}
	return r.FacilityID == "" || scope.Facility(r.FacilityID)
}

type Page struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

func (s *Service) List(ctx context.Context, kind, after string, limit int) (Page, error) {
	scope, err := authorize(ctx, readRoles[kind]...)
	if err != nil {
		return Page{}, err
	}
	if limit < 1 || limit > 100 {
		return Page{}, invalid("limit must be between 1 and 100")
	}
	p := Page{Items: []json.RawMessage{}}
	err = s.Store.View(ctx, func(tx database.Tx) error {
		rows, err := tx.List(kind, after, limit)
		if err != nil {
			return err
		}
		for _, b := range rows {
			if readable(scope, kind, b) {
				p.Items = append(p.Items, b)
			}
		}
		if len(rows) == limit {
			var last struct {
				ID      string `json:"id"`
				EventID string `json:"event_id"`
			}
			if err = json.Unmarshal(rows[len(rows)-1], &last); err != nil {
				return err
			}
			p.NextCursor = last.ID
			if p.NextCursor == "" {
				p.NextCursor = last.EventID
			}
		}
		return nil
	})
	return p, err
}
func (s *Service) Get(ctx context.Context, kind, id string) (json.RawMessage, error) {
	scope, err := authorize(ctx, readRoles[kind]...)
	if err != nil {
		return nil, err
	}
	var b json.RawMessage
	err = s.Store.View(ctx, func(tx database.Tx) error {
		var err error
		b, err = tx.Get(kind, id)
		if err != nil {
			return err
		}
		if !readable(scope, kind, b) {
			return domain.ErrNotFound
		}
		return nil
	})
	return b, err
}
