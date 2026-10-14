package integration

import (
	"context"
	"encoding/json"
	"errors"
	grpcadapter "healthcare/internal/adapters/grpc"
	kafkaadapter "healthcare/internal/adapters/kafka"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/messaging"
	"healthcare/internal/framework/tenancy"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type testSink struct {
	fail   bool
	keys   []string
	events []messaging.Envelope
}

func (s *testSink) Publish(ctx context.Context, key string, b []byte) error {
	if s.fail {
		return errors.New("broker unavailable")
	}
	var e messaging.Envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return err
	}
	s.keys = append(s.keys, key)
	s.events = append(s.events, e)
	return nil
}
func TestOutboxAcknowledgmentAndRecovery(t *testing.T) {
	f := setup(t)
	now := time.Now().UTC()
	f.svc.Now = func() time.Time { return now }
	prepare(t, f)
	sink := &testSink{fail: true}
	o := messaging.Outbox{Service: f.svc, Sink: sink}
	ctx := tenancy.Worker(context.Background(), "north-health")
	if err := o.PublishOne(ctx); err == nil {
		t.Fatal("broker error ignored")
	}
	page := call[application.Page](t, f, "GET", "/api/v1/events", f.north, "", nil, 200)
	for _, b := range page.Items {
		var e domain.Event
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Published {
			t.Fatal("unacknowledged event marked published")
		}
	}
	now = now.Add(31 * time.Second)
	sink.fail = false
	for i := 0; i < 100; i++ {
		err := o.PublishOne(ctx)
		if errors.Is(err, domain.ErrNotFound) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.events) != len(page.Items) {
		t.Fatalf("delivery mismatch: %d vs %d", len(sink.events), len(page.Items))
	}
	ids := map[string]bool{}
	for _, e := range sink.events {
		if e.TenantID != "north-health" || e.EventID == "" || e.SchemaVersion != 1 || ids[e.EventID] {
			t.Fatal("invalid or duplicate envelope", e)
		}
		ids[e.EventID] = true
	}
}
func TestKafkaInboxAndTenantAuthorization(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	call[domain.Membership](t, f, "PUT", "/api/v1/memberships/kafka-integration", f.north, "", map[string]any{"roles": []string{"integration"}, "facilities": []string{c.facility.ID}, "active": true}, 200)
	event := kafkaadapter.OutcomeEvent{TenantID: "north-health", EventID: "event-one", EventType: "workflow.outcome", SchemaVersion: 1, WorkflowID: c.order.WorkflowID, Step: "insurance", Outcome: "approved", CorrelationID: domain.ID()}
	b, _ := json.Marshal(event)
	for i := 0; i < 2; i++ {
		if err := kafkaadapter.ApplyOutcome(context.Background(), f.svc, "kafka-integration", b); err != nil {
			t.Fatal(err)
		}
	}
	event.Outcome = "rejected"
	b, _ = json.Marshal(event)
	if err := kafkaadapter.ApplyOutcome(context.Background(), f.svc, "kafka-integration", b); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("modified event replay accepted", err)
	}
	event.TenantID = "south-health"
	b, _ = json.Marshal(event)
	if err := kafkaadapter.ApplyOutcome(context.Background(), f.svc, "kafka-integration", b); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("unregistered tenant service membership accepted", err)
	}
}
func TestGRPCTenantIsolation(t *testing.T) {
	f := setup(t)
	c := prepare(t, f)
	listener := bufconn.Listen(1 << 20)
	server := grpcadapter.New(f.svc, f.auth)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); listener.Close() })
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, tc := range []struct {
		token string
		code  codes.Code
	}{{f.north, codes.OK}, {f.south, codes.NotFound}, {f.platform, codes.PermissionDenied}, {"bad", codes.Unauthenticated}} {
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tc.token, "x-tenant-id", map[bool]string{true: "north-health", false: "south-health"}[tc.token != f.south]))
		var result structpb.Struct
		err = conn.Invoke(ctx, "/healthcare.v1.HealthcareService/GetPatient", wrapperspb.String(c.patient.ID), &result)
		if status.Code(err) != tc.code {
			t.Fatalf("gRPC isolation: want %s, got %s: %v", tc.code, status.Code(err), err)
		}
	}
}
