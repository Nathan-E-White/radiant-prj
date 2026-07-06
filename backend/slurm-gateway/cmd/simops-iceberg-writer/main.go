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
		log.Fatalf("invalid iceberg writer configuration: %v", err)
	}

	store, err := gateway.NewPostgresSimopsStore(cfg.Simops.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize iceberg writer store: %v", err)
	}

	writer, err := gateway.NewSimopsArtifactWriter(cfg.Simops, store, time.Now)
	if err != nil {
		log.Fatalf("invalid iceberg writer configuration: %v", err)
	}
	eventLog := gateway.SimopsEventLog(gateway.MemorySimopsEventLog{Store: store})
	if cfg.Simops.TelemetryLog == "redpanda" {
		redpandaLog, err := gateway.NewRedpandaEventLog(cfg.Simops, store)
		if err != nil {
			log.Fatalf("initialize iceberg writer redpanda event log: %v", err)
		}
		eventLog = redpandaLog
	}
	processor := gateway.NewSimopsArtifactIntentProcessor(writer, eventLog, cfg.Simops.RedpandaTopic, cfg.Simops.IcebergBatchSize, time.Now)
	metrics := gateway.NewSimopsConsumerMetrics()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := gateway.RunArtifactIntentConsumer(ctx, cfg.Simops, nil, processor, metrics); err != nil {
			log.Printf("iceberg artifact consumer stopped: %v", err)
			metrics.MarkBrokerConnected(false)
			metrics.SetLastError(err)
		}
	}()

	addr := getenv("SIMOPS_ICEBERG_WRITER_ADDR", ":9460")
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
			"status":              state,
			"consumer_state":      state,
			"consumer_group":      cfg.Simops.IcebergConsumerGroup,
			"catalog":             cfg.Simops.IcebergCatalog,
			"warehouse":           cfg.Simops.IcebergWarehouse,
			"s3_endpoint":         cfg.Simops.IcebergS3Endpoint,
			"manifest_dir":        cfg.Simops.IcebergManifestDir,
			"rust_command":        cfg.Simops.IcebergRustCommand,
			"redpanda_topic":      cfg.Simops.RedpandaTopic,
			"writer_mode":         cfg.Simops.IcebergWriterMode,
			"batch_size":          cfg.Simops.IcebergBatchSize,
			"flush_interval":      cfg.Simops.IcebergFlushInterval.String(),
			"metrics":             snapshot,
			"implementation_note": "consumer owns artifact commits; manifest mode is not a real Iceberg append",
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("simops_iceberg_writer")))
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
	log.Printf("simops iceberg writer contract service listening on %s for warehouse %s", addr, cfg.Simops.IcebergWarehouse)
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
