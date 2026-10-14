package mongodb

import (
	"context"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/tenancy"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Documents struct{client *mongo.Client;collection *mongo.Collection}
type Document struct{ID string `json:"id" bson:"id"`;TenantID string `json:"tenant_id" bson:"tenant_id"`;PatientID string `json:"patient_id" bson:"patient_id"`;FacilityID string `json:"facility_id" bson:"facility_id"`;ObjectReference string `json:"object_reference" bson:"object_reference"`;ContentType string `json:"content_type" bson:"content_type"`;SHA256 string `json:"sha256" bson:"sha256"`}
func Open(ctx context.Context,uri,dbName string)(*Documents,error){if dbName==""{return nil,domain.ErrInvalid};client,err:=mongo.Connect(options.Client().ApplyURI(uri).SetMaxPoolSize(16).SetMinPoolSize(1).SetConnectTimeout(5*time.Second));if err!=nil{return nil,err};ctx,cancel:=context.WithTimeout(ctx,10*time.Second);defer cancel();if err=client.Ping(ctx,nil);err!=nil{client.Disconnect(context.WithoutCancel(ctx));return nil,err};return &Documents{client:client,collection:client.Database(dbName).Collection("clinical_documents")},nil}
func(d *Documents)Close(ctx context.Context)error{return d.client.Disconnect(ctx)}
// EnsureIndexes belongs to provisioning, not routine request processing.
func(d *Documents)EnsureIndexes(ctx context.Context)error{_,err:=d.collection.Indexes().CreateOne(ctx,mongo.IndexModel{Keys:bson.D{{Key:"tenant_id",Value:1},{Key:"id",Value:1}},Options:options.Index().SetUnique(true)});return err}
func(d *Documents)Put(ctx context.Context,v Document)error{s,err:=tenancy.From(ctx);if err!=nil{return err};if !s.Has("tenant_admin","clinical","integration","worker")||v.TenantID!=s.TenantID||!s.Facility(v.FacilityID){return domain.ErrForbidden};if v.ID==""||v.PatientID==""||v.FacilityID==""||len(v.SHA256)!=64||v.ObjectReference==""{return domain.ErrInvalid};ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();_,err=d.collection.ReplaceOne(ctx,bson.D{{Key:"tenant_id",Value:s.TenantID},{Key:"id",Value:v.ID}},v,options.Replace().SetUpsert(true));return err}
func(d *Documents)Get(ctx context.Context,id string)(Document,error){var v Document;s,err:=tenancy.From(ctx);if err!=nil{return v,err};if !s.Has("tenant_admin","clinical","integration","worker"){return v,domain.ErrForbidden};ctx,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel();err=d.collection.FindOne(ctx,bson.D{{Key:"tenant_id",Value:s.TenantID},{Key:"id",Value:id}}).Decode(&v);if errors.Is(err,mongo.ErrNoDocuments){return v,domain.ErrNotFound};if err==nil&&!s.Facility(v.FacilityID){return Document{},domain.ErrNotFound};return v,err}
