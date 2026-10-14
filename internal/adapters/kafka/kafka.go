package kafka

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/security"
	"io"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type Config struct {
	Brokers                                                                      []string
	Topic, InputTopic, DeadLetterTopic, GroupID, ServiceUser, Username, Password string
	TLS                                                                          bool
}

func (c Config) Validate() error {
	if len(c.Brokers) == 0 || c.Topic == "" {
		return errors.New("Kafka brokers and output topic are required")
	}
	if c.InputTopic != "" && (c.DeadLetterTopic == "" || c.GroupID == "" || c.ServiceUser == "" || c.InputTopic == c.DeadLetterTopic || c.InputTopic == c.Topic) {
		return errors.New("Kafka input requires distinct output/dead-letter topics, group ID and registered service user")
	}
	if (c.Username == "") != (c.Password == "") {
		return errors.New("Kafka username and password must be configured together")
	}
	return nil
}
func (c Config) security() (*tls.Config, sasl.Mechanism) {
	var t *tls.Config
	if c.TLS {
		t = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	var mechanism sasl.Mechanism
	if c.Username != "" {
		mechanism = plain.Mechanism{Username: c.Username, Password: c.Password}
	}
	return t, mechanism
}

type Producer struct{ writer *kafkago.Writer }

func NewProducer(c Config, topic string) *Producer {
	t, mechanism := c.security()
	return &Producer{writer: &kafkago.Writer{Addr: kafkago.TCP(c.Brokers...), Topic: topic, Balancer: &kafkago.Hash{}, RequiredAcks: kafkago.RequireAll, Async: false, WriteTimeout: 10 * time.Second, ReadTimeout: 10 * time.Second, Transport: &kafkago.Transport{TLS: t, SASL: mechanism}, AllowAutoTopicCreation: false}}
}
func (p *Producer) Publish(ctx context.Context, key string, b []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{Key: []byte(key), Value: b})
}
func (p *Producer) Close() error { return p.writer.Close() }

type OutcomeEvent struct {
	TenantID      string `json:"tenant_id"`
	EventID       string `json:"event_id"`
	EventType     string `json:"event_type"`
	SchemaVersion int    `json:"schema_version"`
	WorkflowID    string `json:"workflow_id"`
	Step          string `json:"step"`
	Outcome       string `json:"outcome"`
	CorrelationID string `json:"correlation_id"`
}

// ApplyOutcome validates a tenant membership for the configured service identity.
// ACLs must restrict the input topic to trusted producers; payload tenant_id alone is not authentication.
func ApplyOutcome(ctx context.Context, s *application.Service, user string, b []byte) error {
	if len(b) > 1<<20 {
		return domain.ErrInvalid
	}
	var e OutcomeEvent
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil {
		return domain.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return domain.ErrInvalid
	}
	if e.SchemaVersion != 1 || e.EventType != "workflow.outcome" || strings.TrimSpace(e.EventID) == "" || len(e.EventID) > 128 || e.TenantID == "" || e.WorkflowID == "" {
		return domain.ErrInvalid
	}
	a := security.Auth{Store: s.Store}
	tenantCtx, err := a.Resolve(ctx, security.Claims{RegisteredClaims: registered(user)}, e.TenantID, e.CorrelationID)
	if err != nil {
		return err
	}
	_, err = s.IntegrationOutcome(tenantCtx, e.EventID, e.WorkflowID, application.OutcomeInput{Step: e.Step, Outcome: e.Outcome})
	return err
}
func NewReader(c Config) *kafkago.Reader {
	t, mechanism := c.security()
	return kafkago.NewReader(kafkago.ReaderConfig{Brokers: c.Brokers, Topic: c.InputTopic, GroupID: c.GroupID, MinBytes: 1, MaxBytes: 1 << 20, QueueCapacity: 16, CommitInterval: 0, Dialer: &kafkago.Dialer{Timeout: 10 * time.Second, TLS: t, SASLMechanism: mechanism}})
}

// Consume processes serially, so committing one offset cannot skip an unfinished earlier message.
func Consume(ctx context.Context, s *application.Service, c Config) error {
	reader := NewReader(c)
	defer reader.Close()
	dlq := NewProducer(c, c.DeadLetterTopic)
	defer dlq.Close()
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		var applyErr error
		for attempt := 0; attempt < 5; attempt++ {
			applyErr = ApplyOutcome(ctx, s, c.ServiceUser, msg.Value)
			if applyErr == nil || permanent(applyErr) {
				break
			}
			timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if applyErr != nil {
			dead, err := json.Marshal(map[string]any{"topic": msg.Topic, "partition": msg.Partition, "offset": msg.Offset, "reason": reason(applyErr), "original": json.RawMessage(validJSON(msg.Value))})
			if err != nil {
				return err
			}
			if err = dlq.Publish(ctx, string(msg.Key), dead); err != nil {
				return fmt.Errorf("dead-letter delivery failed: %w", err)
			}
		}
		if err = reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}
func permanent(err error) bool {
	return errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict)
}
func reason(err error) string {
	switch {
	case errors.Is(err, domain.ErrInactive):
		return "tenant_inactive"
	case errors.Is(err, domain.ErrForbidden):
		return "access_denied"
	case permanent(err):
		return "invalid_event"
	default:
		return "processing_failed"
	}
}
func validJSON(b []byte) []byte {
	if json.Valid(b) {
		return b
	}
	encoded, _ := json.Marshal(string(b))
	return encoded
}
