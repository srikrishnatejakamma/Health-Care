package database

import (
	"context"
	"encoding/json"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"sort"
	"sync"
)

// Memory is deliberately ephemeral. Production and multi-process execution use PostgreSQL.
type Memory struct { mu sync.Mutex; tenants map[string]domain.Tenant; members map[string]domain.Membership; records map[string]map[string]json.RawMessage }
func NewMemory() *Memory { return &Memory{tenants:map[string]domain.Tenant{},members:map[string]domain.Membership{},records:map[string]map[string]json.RawMessage{}} }
func (m *Memory) CreateTenant(ctx context.Context,t domain.Tenant,u domain.Membership) error { m.mu.Lock();defer m.mu.Unlock();if _,ok:=m.tenants[t.ID];ok{return domain.ErrConflict};m.tenants[t.ID]=t;m.members[t.ID+"/"+u.UserID]=u;m.records[t.ID]=map[string]json.RawMessage{};return nil }
func (m *Memory) Tenant(ctx context.Context,id string)(domain.Tenant,error){m.mu.Lock();defer m.mu.Unlock();t,ok:=m.tenants[id];if !ok{return t,domain.ErrNotFound};return t,nil}
func (m *Memory) Tenants(ctx context.Context,after string,limit int)([]domain.Tenant,error){m.mu.Lock();defer m.mu.Unlock();ids:=[]string{};for id:=range m.tenants{if id>after{ids=append(ids,id)}};sort.Strings(ids);if len(ids)>limit{ids=ids[:limit]};out:=make([]domain.Tenant,0,len(ids));for _,id:=range ids{out=append(out,m.tenants[id])};return out,nil}
func (m *Memory) UpdateTenant(ctx context.Context,t domain.Tenant)error{m.mu.Lock();defer m.mu.Unlock();old,ok:=m.tenants[t.ID];if !ok{return domain.ErrNotFound};if t.Version!=old.Version+1{return domain.ErrConflict};m.tenants[t.ID]=t;return nil}
func (m *Memory) Membership(ctx context.Context,t,u string)(domain.Membership,error){m.mu.Lock();defer m.mu.Unlock();v,ok:=m.members[t+"/"+u];if !ok{return v,domain.ErrNotFound};v.Roles=append([]string(nil),v.Roles...);v.Facilities=append([]string(nil),v.Facilities...);return v,nil}
func (m *Memory) PutMembership(ctx context.Context,u domain.Membership)error{m.mu.Lock();defer m.mu.Unlock();if _,ok:=m.tenants[u.TenantID];!ok{return domain.ErrNotFound};u.Roles=append([]string(nil),u.Roles...);u.Facilities=append([]string(nil),u.Facilities...);m.members[u.TenantID+"/"+u.UserID]=u;return nil}
func(m *Memory) Atomic(ctx context.Context,fn func(Tx)error)error{return m.run(ctx,fn,false)}
func(m *Memory) View(ctx context.Context,fn func(Tx)error)error{return m.run(ctx,fn,true)}
func(m *Memory)run(ctx context.Context,fn func(Tx)error,readOnly bool)error{s,err:=tenancy.From(ctx);if err!=nil{return err};m.mu.Lock();defer m.mu.Unlock();if err=ctx.Err();err!=nil{return err};t,ok:=m.tenants[s.TenantID];if !ok{return domain.ErrNotFound};if t.Status!="active"{return domain.ErrInactive};records:=map[string]json.RawMessage{};for k,v:=range m.records[t.ID]{records[k]=append(json.RawMessage(nil),v...)};tx:=&memoryTx{tenant:t,records:records,readOnly:readOnly};if err=fn(tx);err!=nil{return err};if err=ctx.Err();err!=nil{return err};if !readOnly{m.records[t.ID]=records};return nil}
func(m *Memory)Ping(ctx context.Context)error{return ctx.Err()}
func(m *Memory)Close(){}
type memoryTx struct { tenant domain.Tenant; records map[string]json.RawMessage;readOnly bool }
func(t *memoryTx)Tenant()domain.Tenant{return t.tenant}
func(t *memoryTx)Get(kind,id string)(json.RawMessage,error){v,ok:=t.records[kind+"/"+id];if !ok{return nil,domain.ErrNotFound};return append(json.RawMessage(nil),v...),nil}
func(t *memoryTx)List(kind,after string,limit int)([]json.RawMessage,error){ids:=[]string{};prefix:=kind+"/";for k:=range t.records{if len(k)>len(prefix)&&k[:len(prefix)]==prefix&&k[len(prefix):]>after{ids=append(ids,k)}};sort.Strings(ids);if len(ids)>limit{ids=ids[:limit]};out:=make([]json.RawMessage,0,len(ids));for _,id:=range ids{out=append(out,append(json.RawMessage(nil),t.records[id]...))};return out,nil}
func(t *memoryTx)Put(kind,id string,b json.RawMessage)error{if t.readOnly{return domain.ErrForbidden};if err:=validateRecord(t.tenant.ID,id,b);err!=nil{return err};t.records[kind+"/"+id]=append(json.RawMessage(nil),b...);return nil}
func(t *memoryTx)Delete(kind,id string)error{if t.readOnly{return domain.ErrForbidden};delete(t.records,kind+"/"+id);return nil}
func validateRecord(tenantID,id string,b json.RawMessage)error{var v struct{TenantID string `json:"tenant_id"`;ID string `json:"id"`;EventID string `json:"event_id"`};if json.Unmarshal(b,&v)!=nil||v.TenantID!=tenantID||(v.ID!=id&&v.EventID!=id){return domain.ErrForbidden};return nil}
