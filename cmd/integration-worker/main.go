package main

import (
	"context"
	"errors"
	"fmt"
	kafkaadapter "healthcare/internal/adapters/kafka"
	"healthcare/internal/application"
	"healthcare/internal/bootstrap"
	"healthcare/internal/framework/configuration"
	"healthcare/internal/framework/messaging"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)
func run()error{
	if err:=configuration.LoadDotEnv(".env");err!=nil{return err};c,err:=configuration.Load();if err!=nil{return err};if c.Store!="postgres"{return errors.New("integration worker requires STORE=postgres")};k:=kafkaadapter.Config{Topic:os.Getenv("KAFKA_TOPIC"),InputTopic:os.Getenv("KAFKA_INPUT_TOPIC"),DeadLetterTopic:os.Getenv("KAFKA_DLQ_TOPIC"),GroupID:os.Getenv("KAFKA_GROUP_ID"),ServiceUser:os.Getenv("KAFKA_SERVICE_USER"),Username:os.Getenv("KAFKA_USERNAME"),Password:os.Getenv("KAFKA_PASSWORD"),TLS:os.Getenv("KAFKA_TLS")=="true"};if b:=os.Getenv("KAFKA_BROKERS");b!=""{for _,broker:=range strings.Split(b,","){k.Brokers=append(k.Brokers,strings.TrimSpace(broker))}};if err=k.Validate();err!=nil{return err};if c.Environment=="production"&&(!k.TLS||k.Username==""){return fmt.Errorf("production Kafka requires TLS and configured SASL credentials")}
	ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer stop();startup,cancel:=context.WithTimeout(ctx,15*time.Second);defer cancel();store,err:=bootstrap.Store(startup,c);if err!=nil{return err};defer store.Close();svc:=application.New(store);producer:=kafkaadapter.NewProducer(k,k.Topic);defer producer.Close();outbox:=&messaging.Outbox{Service:svc,Sink:producer};result:=make(chan error,2);running:=1;go func(){result<-outbox.Run(ctx)}();if k.InputTopic!=""{running++;go func(){result<-kafkaadapter.Consume(ctx,svc,k)}()};first:=<-result;stop();for i:=1;i<running;i++{<-result};if errors.Is(first,context.Canceled){return nil};return first
}
func main(){slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout,nil)));if err:=run();err!=nil{slog.Error("integration worker failed","error",err);os.Exit(1)}}
