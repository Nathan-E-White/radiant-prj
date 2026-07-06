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
		log.Fatalf("invalid workbench iceberg writer configuration: %v", err)
	}
	writer, err := gateway.NewWorkbenchIcebergWriter(cfg.Workbench)
	if err != nil {
		log.Fatalf("initialize workbench iceberg writer: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metrics := gateway.NewSimopsConsumerMetrics()
	go runIcebergConsumer(ctx, metrics, "scada", func() error {
		return consumeScada(ctx, cfg.Workbench, writer, metrics)
	})
	go runIcebergConsumer(ctx, metrics, "results", func() error {
		return consumeResults(ctx, cfg.Workbench, writer, metrics)
	})
	go runIcebergConsumer(ctx, metrics, "twin", func() error {
		return consumeTwin(ctx, cfg.Workbench, writer, metrics)
	})

	addr := getenv("WORKBENCH_ICEBERG_WRITER_ADDR", ":9490")
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
			"status":    state,
			"tables":    []string{"scada.measured_frames", "simops.simulated_results", "digital_twin.state_values"},
			"warehouse": cfg.Workbench.IcebergWarehouse,
			"metrics":   snapshot,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("workbench_iceberg_writer")))
	})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("workbench iceberg writer listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func consumeScada(ctx context.Context, cfg gateway.WorkbenchConfig, writer *gateway.WorkbenchIcebergWriter, metrics *gateway.SimopsConsumerMetrics) error {
	reader, err := gateway.NewWorkbenchKafkaReader(cfg, cfg.ScadaTopic, cfg.IcebergConsumerGroup+"-scada")
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		projection, err := gateway.ProjectScadaFrame(msg.Topic, msg.Partition, msg.Offset, msg.Value)
		if err != nil {
			return err
		}
		if err := writer.AppendScada(ctx, projection); err != nil {
			return err
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
		metrics.MarkConsumed(msg.Offset)
		metrics.IncFramesWritten(1)
	}
}

func consumeResults(ctx context.Context, cfg gateway.WorkbenchConfig, writer *gateway.WorkbenchIcebergWriter, metrics *gateway.SimopsConsumerMetrics) error {
	reader, err := gateway.NewWorkbenchKafkaReader(cfg, cfg.ResultsTopic, cfg.IcebergConsumerGroup+"-results")
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		projection, err := gateway.ProjectSimopsResultFrame(msg.Topic, msg.Partition, msg.Offset, msg.Value)
		if err != nil {
			return err
		}
		if err := writer.AppendResult(ctx, projection); err != nil {
			return err
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
		metrics.MarkConsumed(msg.Offset)
		metrics.IncFramesWritten(1)
	}
}

func consumeTwin(ctx context.Context, cfg gateway.WorkbenchConfig, writer *gateway.WorkbenchIcebergWriter, metrics *gateway.SimopsConsumerMetrics) error {
	reader, err := gateway.NewWorkbenchKafkaReader(cfg, cfg.TwinStateTopic, cfg.IcebergConsumerGroup+"-twin")
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		projection, err := gateway.ProjectTwinState(msg.Topic, msg.Partition, msg.Offset, msg.Value)
		if err != nil {
			return err
		}
		if err := writer.AppendTwin(ctx, projection); err != nil {
			return err
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
		metrics.MarkConsumed(msg.Offset)
		metrics.IncFramesWritten(1)
	}
}

func runIcebergConsumer(ctx context.Context, metrics *gateway.SimopsConsumerMetrics, label string, run func() error) {
	if err := run(); err != nil && ctx.Err() == nil {
		log.Printf("workbench iceberg %s consumer stopped: %v", label, err)
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
