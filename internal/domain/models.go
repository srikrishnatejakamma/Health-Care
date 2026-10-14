package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("resource not found")
	ErrConflict  = errors.New("resource conflict")
	ErrForbidden = errors.New("access denied")
	ErrInvalid   = errors.New("invalid request")
	ErrInactive  = errors.New("tenant is not active")
	ErrQuota     = errors.New("resource quota exceeded")
)

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

type Quotas struct {
	ConcurrentJobs    int   `json:"concurrent_jobs"`
	QueuedJobs        int   `json:"queued_jobs"`
	MemoryMB          int64 `json:"memory_mb"`
	RequestsPerMinute int   `json:"requests_per_minute"`
}

func DefaultQuotas() Quotas {
	return Quotas{ConcurrentJobs: 2, QueuedJobs: 100, MemoryMB: 64, RequestsPerMinute: 300}
}
func (q Quotas) Valid() bool {
	return q.ConcurrentJobs >= 1 && q.ConcurrentJobs <= 64 && q.QueuedJobs >= 1 && q.QueuedJobs <= 10000 && q.MemoryMB >= 8 && q.MemoryMB <= 4096 && q.RequestsPerMinute >= 1 && q.RequestsPerMinute <= 100000
}

type Tenant struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	Timezone      string    `json:"timezone"`
	Region        string    `json:"region"`
	IsolationMode string    `json:"isolation_mode"`
	Version       int       `json:"version"`
	Quotas        Quotas    `json:"quotas"`
	CreatedAt     time.Time `json:"created_at"`
}
type Membership struct {
	TenantID   string   `json:"tenant_id"`
	UserID     string   `json:"user_id"`
	Roles      []string `json:"roles"`
	Facilities []string `json:"facilities"`
	Active     bool     `json:"active"`
}
type Facility struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}
type Staff struct {
	ID             string   `json:"id"`
	TenantID       string   `json:"tenant_id"`
	FacilityID     string   `json:"facility_id"`
	Name           string   `json:"name"`
	Qualifications []string `json:"qualifications"`
	Active         bool     `json:"active"`
}
type CareResource struct {
	ID         string `json:"id"`
	TenantID   string `json:"tenant_id"`
	FacilityID string `json:"facility_id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
}
type Patient struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	FacilityID string    `json:"facility_id"`
	ExternalID string    `json:"external_id"`
	Name       string    `json:"name"`
	BirthDate  string    `json:"birth_date"`
	CreatedAt  time.Time `json:"created_at"`
}
type Order struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	PatientID    string    `json:"patient_id"`
	FacilityID   string    `json:"facility_id"`
	Medication   string    `json:"medication"`
	Prescription string    `json:"prescription"`
	Status       string    `json:"status"`
	WorkflowID   string    `json:"workflow_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Workflow struct {
	ID                string            `json:"id"`
	TenantID          string            `json:"tenant_id"`
	FacilityID        string            `json:"facility_id"`
	OrderID           string            `json:"order_id"`
	Status            string            `json:"status"`
	Version           int               `json:"version"`
	DefinitionVersion int               `json:"definition_version"`
	Outcomes          map[string]string `json:"outcomes"`
	Checkpoints       []string          `json:"checkpoints"`
	UpdatedAt         time.Time         `json:"updated_at"`
}
type Appointment struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	PatientID  string    `json:"patient_id"`
	FacilityID string    `json:"facility_id"`
	OrderID    string    `json:"order_id"`
	NurseID    string    `json:"nurse_id"`
	ResourceID string    `json:"resource_id"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Status     string    `json:"status"`
}
type Job struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Kind        string    `json:"kind"`
	TargetID    string    `json:"target_id"`
	Status      string    `json:"status"`
	DueAt       time.Time `json:"due_at"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"max_attempts"`
	LeaseID     string    `json:"lease_id,omitempty"`
	LeaseUntil  time.Time `json:"lease_until,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	MemoryMB    int64     `json:"memory_mb"`
	Version     int       `json:"version"`
}
type Schedule struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Expression string    `json:"expression"`
	Timezone   string    `json:"timezone"`
	Kind       string    `json:"kind"`
	Paused     bool      `json:"paused"`
	NextRun    time.Time `json:"next_run"`
	Version    int       `json:"version"`
}
type Event struct {
	ID                   string    `json:"event_id"`
	TenantID             string    `json:"tenant_id"`
	Type                 string    `json:"event_type"`
	AggregateID          string    `json:"aggregate_id"`
	SchemaVersion        int       `json:"schema_version"`
	ConfigurationVersion int       `json:"configuration_version"`
	CorrelationID        string    `json:"correlation_id"`
	OccurredAt           time.Time `json:"occurred_at"`
	Published            bool      `json:"published"`
	DeliveryLeaseID string `json:"delivery_lease_id,omitempty"`
	DeliveryLeaseUntil time.Time `json:"delivery_lease_until,omitempty"`
}
type Audit struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Actor         string    `json:"actor"`
	Action        string    `json:"action"`
	ResourceID    string    `json:"resource_id"`
	CorrelationID string    `json:"correlation_id"`
	At            time.Time `json:"at"`
}
