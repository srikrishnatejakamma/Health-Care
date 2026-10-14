package main

import (
	"context"
	"errors"
	grpcadapter "healthcare/internal/adapters/grpc"
	"healthcare/internal/adapters/rest"
	"healthcare/internal/application"
	"healthcare/internal/bootstrap"
	"healthcare/internal/framework/configuration"
	"healthcare/internal/framework/workflow"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func run() error {
	if err := configuration.LoadDotEnv(".env"); err != nil {
		return err
	}
	c, err := configuration.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	store, err := bootstrap.Store(startup, c)
	if err != nil {
		return err
	}
	defer store.Close()
	if c.DemoSeed {
		if err = bootstrap.Seed(startup, store); err != nil {
			return err
		}
	}
	auth, err := bootstrap.Auth(c, store)
	if err != nil {
		return err
	}
	svc := application.New(store)
	runner := workflow.New(svc, c.Workers, c.MemoryMB)
	workerCtx, workerStop := context.WithCancel(ctx)
	defer workerStop()
	var wg sync.WaitGroup
	if c.WorkerEnabled {
		wg.Add(1)
		go func() { defer wg.Done(); runner.Run(workerCtx) }()
	}
	api := rest.New(svc, auth, runner)
	server := &http.Server{Addr: c.Address, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	result := make(chan error, 2)
	var rpc *grpc.Server
	if c.GRPCAddress != "" {
		listener,e:=net.Listen("tcp",c.GRPCAddress);if e!=nil{workerStop();wg.Wait();return e}
		defer listener.Close()
		var options []grpc.ServerOption
		if c.TLSCert!=""{creds,e:=credentials.NewServerTLSFromFile(c.TLSCert,c.TLSKey);if e!=nil{workerStop();wg.Wait();return e};options=append(options,grpc.Creds(creds))}
		rpc=grpcadapter.New(svc,auth,options...)
		go func(){result<-rpc.Serve(listener)}()
	}
	go func() {
		slog.Info("healthcare API started", "address", c.Address, "store", c.Store, "worker_enabled", c.WorkerEnabled)
		if c.TLSCert != "" {
			result <- server.ListenAndServeTLS(c.TLSCert, c.TLSKey)
		} else {
			result <- server.ListenAndServe()
		}
	}()
	select {
	case err = <-result:
		stop()
	case <-ctx.Done():
	}
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), c.ShutdownTimeout)
	defer shutdownCancel()
	shutdownErr := server.Shutdown(shutdown)
	if rpc!=nil{done:=make(chan struct{});go func(){rpc.GracefulStop();close(done)}();select{case <-done:case <-shutdown.Done():rpc.Stop()}}
	workerStop()
	wg.Wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("startup or shutdown failed", "error", err)
		os.Exit(1)
	}
}
