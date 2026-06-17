package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nano-vllm/go-serving/internal/api"
	"github.com/nano-vllm/go-serving/internal/backend"
	"github.com/nano-vllm/go-serving/internal/balancer"
	"github.com/nano-vllm/go-serving/internal/config"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	backends := make([]*backend.Backend, len(cfg.Backends))
	for i, bc := range cfg.Backends {
		backends[i] = backend.NewBackend(bc)
	}
	pool := backend.NewPool(backends, cfg.Pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.StartHealthChecks(ctx, cfg.HealthCheck)

	bal := balancer.New(cfg.LoadBalancer.Strategy)

	mux := http.NewServeMux()
	api.NewHandler(pool, bal).RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      logging(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		slog.Info("shutting down")
		cancel()
		srv.Shutdown(context.Background())
	}()

	slog.Info("server started",
		"addr", addr,
		"strategy", cfg.LoadBalancer.Strategy,
		"backends", len(backends),
	)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rw.status)
	})
}
