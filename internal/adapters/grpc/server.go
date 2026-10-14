package grpcadapter

import (
	"context"
	"encoding/json"
	"errors"
	"healthcare/internal/application"
	"healthcare/internal/domain"
	"healthcare/internal/framework/security"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Service interface{GetWorkflow(context.Context,*wrapperspb.StringValue)(*structpb.Struct,error);GetPatient(context.Context,*wrapperspb.StringValue)(*structpb.Struct,error)}
type server struct{service *application.Service;auth *security.Auth}
func New(s *application.Service,a *security.Auth,options ...grpc.ServerOption)*grpc.Server{impl:=&server{service:s,auth:a};options=append(options,grpc.UnaryInterceptor(impl.intercept),grpc.MaxRecvMsgSize(1<<20),grpc.MaxSendMsgSize(2<<20));g:=grpc.NewServer(options...);g.RegisterService(&grpc.ServiceDesc{ServiceName:"healthcare.v1.HealthcareService",HandlerType:(*Service)(nil),Methods:[]grpc.MethodDesc{{MethodName:"GetWorkflow",Handler:handler("GetWorkflow")},{MethodName:"GetPatient",Handler:handler("GetPatient")}},Metadata:"api/healthcare.proto"},impl);return g}
func(s *server)intercept(ctx context.Context,req any,info *grpc.UnaryServerInfo,next grpc.UnaryHandler)(any,error){md,_:=metadata.FromIncomingContext(ctx);auth:=md.Get("authorization");if len(auth)!=1{return nil,status.Error(codes.Unauthenticated,"authentication_required")};parts:=strings.Fields(auth[0]);if len(parts)!=2||!strings.EqualFold(parts[0],"Bearer"){return nil,status.Error(codes.Unauthenticated,"invalid_token")};claims,err:=s.auth.Validate(parts[1]);if err!=nil{return nil,status.Error(codes.Unauthenticated,"invalid_token")};selected:="";if values:=md.Get("x-tenant-id");len(values)>1{return nil,status.Error(codes.PermissionDenied,"conflicting_tenant_selectors")}else if len(values)==1{selected=values[0]};ctx,err=s.auth.Resolve(ctx,claims,selected,domain.ID());if err!=nil{return nil,rpcError(err)};ctx,cancel:=context.WithTimeout(ctx,15*time.Second);defer cancel();if err=s.service.AdmitRequest(ctx);err!=nil{return nil,rpcError(err)};return next(ctx,req)}
func(s *server)get(ctx context.Context,kind string,id *wrapperspb.StringValue)(*structpb.Struct,error){if id.GetValue()==""{return nil,status.Error(codes.InvalidArgument,"resource ID required")};b,err:=s.service.Get(ctx,kind,id.Value);if err!=nil{return nil,rpcError(err)};var fields map[string]any;if err=json.Unmarshal(b,&fields);err!=nil{return nil,rpcError(err)};result,err:=structpb.NewStruct(fields);return result,rpcError(err)}
func(s *server)GetWorkflow(ctx context.Context,id *wrapperspb.StringValue)(*structpb.Struct,error){return s.get(ctx,"workflows",id)}
func(s *server)GetPatient(ctx context.Context,id *wrapperspb.StringValue)(*structpb.Struct,error){return s.get(ctx,"patients",id)}
func rpcError(err error)error{if err==nil{return nil};switch{case errors.Is(err,domain.ErrNotFound):return status.Error(codes.NotFound,"not_found");case errors.Is(err,domain.ErrForbidden),errors.Is(err,domain.ErrInactive):return status.Error(codes.PermissionDenied,"access_denied");case errors.Is(err,domain.ErrQuota):return status.Error(codes.ResourceExhausted,"quota_exceeded");case errors.Is(err,domain.ErrInvalid):return status.Error(codes.InvalidArgument,"invalid_request");case errors.Is(err,context.DeadlineExceeded):return status.Error(codes.DeadlineExceeded,"deadline_exceeded");default:return status.Error(codes.Internal,"internal_error")}}
// The contract uses well-known protobuf types; this small registration shim avoids
// generated message code while remaining compatible with clients generated from the .proto.
func handler(method string)func(any,context.Context,func(any)error,grpc.UnaryServerInterceptor)(any,error){return func(srv any,ctx context.Context,decode func(any)error,interceptor grpc.UnaryServerInterceptor)(any,error){in:=new(wrapperspb.StringValue);if err:=decode(in);err!=nil{return nil,err};invoke:=func(ctx context.Context,req any)(any,error){if method=="GetWorkflow"{return srv.(Service).GetWorkflow(ctx,req.(*wrapperspb.StringValue))};return srv.(Service).GetPatient(ctx,req.(*wrapperspb.StringValue))};if interceptor==nil{return invoke(ctx,in)};return interceptor(ctx,in,&grpc.UnaryServerInfo{Server:srv,FullMethod:"/healthcare.v1.HealthcareService/"+method},invoke)}}
