package rest

import (
	"encoding/json"
	"fmt"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

func(s *Server)overview(w http.ResponseWriter,r *http.Request){days:=7;if raw:=r.URL.Query().Get("days");raw!=""{var err error;days,err=strconv.Atoi(raw);if err!=nil{s.fail(w,r,domain.ErrInvalid);return}};result,err:=s.Service.Overview(r.Context(),days,r.URL.Query().Get("facility"));if err!=nil{s.fail(w,r,err);return};write(w,200,result)}
func(s *Server)operations(w http.ResponseWriter,r *http.Request){scope,_:=tenancy.From(r.Context());if !scope.Has("tenant_admin","auditor"){s.fail(w,r,domain.ErrForbidden);return};var m runtime.MemStats;runtime.ReadMemStats(&m);status:="healthy";if s.Service.Store.Ping(r.Context())!=nil{status="unavailable"};workers:=map[string]any{"configured":0,"completed":0,"failed":0,"reserved_mb":0};if s.Runner!=nil{workers=map[string]any{"configured":s.Runner.Workers,"completed":s.Runner.Completed.Load(),"failed":s.Runner.Failed.Load(),"reserved_mb":s.Runner.Memory.Used()}};var pause uint64;if m.NumGC>0{pause=m.PauseNs[(m.NumGC+255)%256]};write(w,200,map[string]any{"scope":"Shared API process","store":map[string]string{"status":status},"databases":[]map[string]string{{"name":"Primary store","status":status,"description":"Configured orchestration repository"},{"name":"SQL Server","status":"not_configured","description":"Optional billing adapter; no connection bound to this process"},{"name":"MongoDB","status":"not_configured","description":"Optional metadata adapter; no connection bound to this process"},{"name":"Redis","status":"not_configured","description":"Optional cache adapter; no connection bound to this process"}},"workers":workers,"memory":map[string]any{"heap_bytes":m.HeapAlloc,"heap_sys_bytes":m.HeapSys,"gc_cycles":m.NumGC,"gc_pause_ns":pause,"goroutines":runtime.NumGoroutine(),"soft_limit_bytes":debug.SetMemoryLimit(-1)},"capabilities":map[string]bool{"designer_publish":false,"external_inventory":false,"claims_submission":false,"callbacks":false,"feature_flags":false}})}

// Stream sends only cache-invalidating hints, never clinical records. Bearer
// credentials stay in an Authorization header and are revalidated every tick.
func(s *Server)stream(w http.ResponseWriter,r *http.Request){if _,ok:=w.(http.Flusher);!ok{s.fail(w,r,domain.ErrInvalid);return};_ = http.NewResponseController(w).SetWriteDeadline(time.Time{});w.Header().Set("Content-Type","text/event-stream");w.Header().Set("X-Accel-Buffering","no");w.Header().Set("Connection","keep-alive");w.WriteHeader(200);ticker:=time.NewTicker(15*time.Second);defer ticker.Stop();send:=func()bool{scope,_:=tenancy.From(r.Context());b,_:=json.Marshal(map[string]any{"tenant_id":scope.TenantID,"at":time.Now().UTC()});_,err:=fmt.Fprintf(w,"event: refresh\ndata: %s\n\n",b);if err!=nil{return false};w.(http.Flusher).Flush();return true};if !send(){return};for{select{case <-r.Context().Done():return;case <-ticker.C:parts:=r.Header.Get("Authorization");if len(parts)<8{return};claims,err:=s.Auth.Validate(parts[7:]);if err!=nil{return};scope,_:=tenancy.From(r.Context());if _,err=s.Auth.Resolve(r.Context(),claims,scope.TenantID,scope.CorrelationID);err!=nil{return};if !send(){return}}}}
