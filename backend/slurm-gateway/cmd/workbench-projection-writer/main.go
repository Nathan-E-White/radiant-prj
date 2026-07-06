package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid workbench projection writer configuration: %v", err)
	}
	store, err := gateway.NewPostgresWorkbenchStore(cfg.Workbench.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize workbench postgres store: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metrics := gateway.NewSimopsConsumerMetrics()
	go runConsumer(ctx, metrics, "scada", func() error {
		return gateway.RunWorkbenchScadaProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
	})
	go runConsumer(ctx, metrics, "results", func() error {
		return gateway.RunWorkbenchResultProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
	})
	go runConsumer(ctx, metrics, "twin", func() error {
		return gateway.RunWorkbenchTwinProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
	})

	addr := getenv("WORKBENCH_PROJECTION_WRITER_ADDR", ":9470")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		snapshot := metrics.Snapshot()
		status := http.StatusOK
		state := "ready"
		if !snapshot.Ready() {
			status = http.StatusServiceUnavailable
			state = "starting"
		}
		writeJSON(w, status, map[string]any{
			"status":  state,
			"topics":  []string{cfg.Workbench.ScadaTopic, cfg.Workbench.ResultsTopic, cfg.Workbench.TwinStateTopic},
			"metrics": snapshot,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("workbench_projection_writer")))
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("workbench projection writer listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runConsumer(ctx context.Context, metrics *gateway.SimopsConsumerMetrics, label string, run func() error) {
	if err := run(); err != nil && ctx.Err() == nil {
		log.Printf("workbench projection %s consumer stopped: %v", label, err)
		metrics.MarkBrokerConnected(false)
		metrics.SetLastError(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
