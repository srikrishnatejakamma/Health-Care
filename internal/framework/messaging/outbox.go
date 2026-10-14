package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/tenancy"
	"time"
)

type Envelope struct {
	TenantID string `json:"tenant_id"`
	EventID string `json:"event_id"`
	EventType string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	SchemaVersion int `json:"schema_version"`
	ConfigurationVersion int `json:"configuration_version"`
	CorrelationID string `json:"correlation_id"`
	OccurredAt time.Time `json:"occurred_at"`
}
type Sink interface { Publish(context.Context,string,[]byte)error }
type Outbox struct {Service *application.Service;Sink Sink}
func(o *Outbox)PublishOne(ctx context.Context)error{
	var e domain.Event;now:=o.Service.Now()
	err:=o.Service.Store.Atomic(ctx,func(tx database.Tx)error{return database.Each[domain.Event](tx,"outbox",func(v domain.Event)error{if e.ID!=""||v.Published||v.DeliveryLeaseUntil.After(now){return nil};v.DeliveryLeaseID=domain.ID();v.DeliveryLeaseUntil=now.Add(30*time.Second);e=v;return database.Put(tx,"outbox",v.ID,v)})});if err!=nil{return err};if e.ID==""{return domain.ErrNotFound}
	b,err:=json.Marshal(Envelope{TenantID:e.TenantID,EventID:e.ID,EventType:e.Type,AggregateID:e.AggregateID,SchemaVersion:e.SchemaVersion,ConfigurationVersion:e.ConfigurationVersion,CorrelationID:e.CorrelationID,OccurredAt:e.OccurredAt});if err!=nil{return err}
	sendCtx,cancel:=context.WithTimeout(ctx,10*time.Second);defer cancel();if err=o.Sink.Publish(sendCtx,e.TenantID+":"+e.AggregateID,b);err!=nil{return err}
	// Sending and marking cannot be atomic across Kafka and PostgreSQL. A crash may
	// cause redelivery with the same event_id; consumers must deduplicate.
	return o.Service.Store.Atomic(ctx,func(tx database.Tx)error{current,err:=database.Get[domain.Event](tx,"outbox",e.ID);if err!=nil{return err};if current.DeliveryLeaseID!=e.DeliveryLeaseID{return domain.ErrConflict};current.Published=true;current.DeliveryLeaseID="";current.DeliveryLeaseUntil=time.Time{};return database.Put(tx,"outbox",current.ID,current)})
}
func(o *Outbox)Run(ctx context.Context)error{ticker:=time.NewTicker(50*time.Millisecond);defer ticker.Stop();cursor:="";for{select{case <-ctx.Done():return ctx.Err();case <-ticker.C:};tenants,err:=o.Service.Store.Tenants(ctx,cursor,1);if err!=nil{return err};if len(tenants)==0{cursor="";continue};t:=tenants[0];cursor=t.ID;if t.Status!="active"{continue};err=o.PublishOne(tenancy.Worker(ctx,t.ID));if err!=nil&&!errors.Is(err,domain.ErrNotFound)&&!errors.Is(err,domain.ErrInactive)&&!errors.Is(err,domain.ErrConflict){return err}}}
