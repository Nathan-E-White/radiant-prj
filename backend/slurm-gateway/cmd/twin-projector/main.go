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
		log.Fatalf("invalid twin projector configuration: %v", err)
	}
	store, err := gateway.NewPostgresWorkbenchStore(cfg.Workbench.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize workbench postgres store: %v", err)
	}
	eventLog, err := gateway.NewRedpandaWorkbenchEventLog(cfg.Workbench)
	if err != nil {
		log.Fatalf("initialize workbench redpanda event log: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metrics := gateway.NewSimopsConsumerMetrics()
	go func() {
		if err := gateway.RunTwinProjector(ctx, cfg.Workbench, nil, nil, store, eventLog, metrics); err != nil && ctx.Err() == nil {
			log.Printf("twin projector stopped: %v", err)
			metrics.MarkBrokerConnected(false)
			metrics.SetLastError(err)
		}
	}()

	addr := getenv("TWIN_PROJECTOR_ADDR", ":9480")
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
			"status":       state,
			"input_topics": []string{cfg.Workbench.ScadaTopic, cfg.Workbench.ResultsTopic},
			"output_topic": cfg.Workbench.TwinStateTopic,
			"metrics":      snapshot,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("twin_projector")))
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("twin projector listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
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
