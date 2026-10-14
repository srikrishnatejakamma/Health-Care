package application

import (
	"context"
	"encoding/json"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/tenancy"
	"time"
)

type DayCount struct{ Date string `json:"date"`;Orders int `json:"orders"`;Completed int `json:"completed"` }
type StatusCount struct{Name string `json:"name"`;Count int `json:"count"`}
type Overview struct{Counts map[string]int `json:"counts"`;Series []DayCount `json:"series"`;Statuses []StatusCount `json:"statuses"`;GeneratedAt time.Time `json:"generated_at"`;Days int `json:"days"`}
func(s *Service)Overview(ctx context.Context,days int,facility string)(Overview,error){
	result:=Overview{Counts:map[string]int{},Series:[]DayCount{},Statuses:[]StatusCount{},GeneratedAt:s.Now(),Days:days};scope,err:=tenancy.From(ctx);if err!=nil{return result,err};if days<1||days>90{return result,domain.ErrInvalid};if facility!=""&&facility!="all"&&!scope.Facility(facility){return result,domain.ErrForbidden}
	err=s.Store.Atomic(ctx,func(tx database.Tx)error{
		loc,e:=time.LoadLocation(tx.Tenant().Timezone);if e!=nil{return e};today:=s.Now().In(loc);positions:=map[string]int{};for i:=days-1;i>=0;i--{date:=today.AddDate(0,0,-i).Format("2006-01-02");positions[date]=len(result.Series);result.Series=append(result.Series,DayCount{Date:date})};statusCounts:=map[string]int{}
		for _,kind:=range []string{"patients","orders","workflows","appointments","jobs"}{if !scope.Has(readRoles[kind]...){continue};switch kind{case "patients":result.Counts["patients"]=0;case "orders":result.Counts["active_orders"]=0;result.Counts["completed_orders"]=0;case "workflows":result.Counts["active_workflows"]=0;result.Counts["pending_authorizations"]=0;case "appointments":result.Counts["appointments"]=0;case "jobs":result.Counts["failed_jobs"]=0;result.Counts["queued_jobs"]=0;result.Counts["running_jobs"]=0}
			if e=database.Each[json.RawMessage](tx,kind,func(raw json.RawMessage)error{var row struct{FacilityID string `json:"facility_id"`;Status string `json:"status"`;CreatedAt time.Time `json:"created_at"`;UpdatedAt time.Time `json:"updated_at"`;Outcomes map[string]string `json:"outcomes"`};if err:=json.Unmarshal(raw,&row);err!=nil{return err};if row.FacilityID!=""&&(!scope.Facility(row.FacilityID)||(facility!=""&&facility!="all"&&row.FacilityID!=facility)){return nil};terminal:=row.Status=="completed"||row.Status=="cancelled"||row.Status=="rejected";switch kind{case "patients":result.Counts["patients"]++;case "orders":if !terminal{result.Counts["active_orders"]++};if row.Status=="completed"{result.Counts["completed_orders"]++};if index,ok:=positions[row.CreatedAt.In(loc).Format("2006-01-02")];ok{result.Series[index].Orders++};case "workflows":statusCounts[row.Status]++;if !terminal{result.Counts["active_workflows"]++};if row.Status=="awaiting_approvals"&&row.Outcomes["insurance"]==""{result.Counts["pending_authorizations"]++};if row.Status=="completed"{if index,ok:=positions[row.UpdatedAt.In(loc).Format("2006-01-02")];ok{result.Series[index].Completed++}};case "appointments":if row.Status=="confirmed"{result.Counts["appointments"]++};case "jobs":switch row.Status{case "dead":result.Counts["failed_jobs"]++;case "queued":result.Counts["queued_jobs"]++;case "running":result.Counts["running_jobs"]++}};return nil});e!=nil{return e}
		};for _,status:=range []string{"awaiting_approvals","ready_to_schedule","scheduled","awaiting_billing","completed","rejected","cancelled"}{result.Statuses=append(result.Statuses,StatusCount{Name:status,Count:statusCounts[status]})};return s.auditRead(ctx,tx,"dashboard","overview")
	});return result,err
}
