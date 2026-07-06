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
		log.Fatalf("invalid timescale writer configuration: %v", err)
	}
	store, err := gateway.NewTimescaleTelemetryStore(cfg.Simops.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize timescale telemetry store: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := gateway.NewSimopsConsumerMetrics()
	go func() {
		if err := gateway.RunTimescaleTelemetryConsumer(ctx, cfg.Simops, nil, store, metrics); err != nil {
			log.Printf("timescale telemetry consumer stopped: %v", err)
			metrics.MarkBrokerConnected(false)
			metrics.SetLastError(err)
		}
	}()

	addr := getenv("SIMOPS_TIMESCALE_WRITER_ADDR", ":9450")
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
			"status":         state,
			"consumer_group": cfg.Simops.TimescaleConsumerGroup,
			"redpanda_topic": cfg.Simops.RedpandaTopic,
			"metrics":        snapshot,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("simops_timescale_writer")))
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("simops timescale writer listening on %s for topic %s group %s", addr, cfg.Simops.RedpandaTopic, cfg.Simops.TimescaleConsumerGroup)
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
