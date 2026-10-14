package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Billing is an optional downstream repository. Workflow/outbox state remains in PostgreSQL.
type Billing struct{db *sql.DB}
type Record struct{ID string `json:"id"`;TenantID string `json:"tenant_id"`;OrderID string `json:"order_id"`;AmountCents int64 `json:"amount_cents"`;Currency string `json:"currency"`;Status string `json:"status"`}
func Open(ctx context.Context,dsn string,maxConnections int)(*Billing,error){if maxConnections<1||maxConnections>64{return nil,domain.ErrInvalid};db,err:=sql.Open("sqlserver",dsn);if err!=nil{return nil,err};db.SetMaxOpenConns(maxConnections);db.SetMaxIdleConns(maxConnections/2);db.SetConnMaxLifetime(time.Hour);if err=db.PingContext(ctx);err!=nil{db.Close();return nil,err};return &Billing{db:db},nil}
func(b *Billing)Close()error{return b.db.Close()}
func(b *Billing)Put(ctx context.Context,r Record)error{s,err:=tenancy.From(ctx);if err!=nil{return err};if !s.Has("billing","tenant_admin","integration","worker")||r.TenantID!=s.TenantID{return domain.ErrForbidden};if r.ID==""||r.OrderID==""||r.AmountCents<0||len(r.Currency)!=3{return domain.ErrInvalid};switch r.Status{case "draft","submitted","paid","rejected":default:return domain.ErrInvalid};ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();tx,err:=b.db.BeginTx(ctx,&sql.TxOptions{Isolation:sql.LevelSerializable});if err!=nil{return err};defer tx.Rollback();result,err:=tx.ExecContext(ctx,`UPDATE healthcare_billing WITH (UPDLOCK,HOLDLOCK) SET order_id=@order,amount_cents=@amount,currency=@currency,status=@status WHERE tenant_id=@tenant AND id=@id`,sql.Named("tenant",s.TenantID),sql.Named("id",r.ID),sql.Named("order",r.OrderID),sql.Named("amount",r.AmountCents),sql.Named("currency",r.Currency),sql.Named("status",r.Status));if err!=nil{return err};n,err:=result.RowsAffected();if err!=nil{return err};if n==0{_,err=tx.ExecContext(ctx,`INSERT INTO healthcare_billing(tenant_id,id,order_id,amount_cents,currency,status) VALUES(@tenant,@id,@order,@amount,@currency,@status)`,sql.Named("tenant",s.TenantID),sql.Named("id",r.ID),sql.Named("order",r.OrderID),sql.Named("amount",r.AmountCents),sql.Named("currency",r.Currency),sql.Named("status",r.Status));if err!=nil{return err}};return tx.Commit()}
func(b *Billing)Get(ctx context.Context,id string)(Record,error){var r Record;s,err:=tenancy.From(ctx);if err!=nil{return r,err};if !s.Has("billing","tenant_admin","integration","worker"){return r,domain.ErrForbidden};ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();err=b.db.QueryRowContext(ctx,`SELECT id,tenant_id,order_id,amount_cents,currency,status FROM healthcare_billing WHERE tenant_id=@tenant AND id=@id`,sql.Named("tenant",s.TenantID),sql.Named("id",id)).Scan(&r.ID,&r.TenantID,&r.OrderID,&r.AmountCents,&r.Currency,&r.Status);if errors.Is(err,sql.ErrNoRows){err=domain.ErrNotFound};return r,err}
