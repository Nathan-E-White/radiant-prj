package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("invalid iceberg writer configuration: %w", err)
	}

	store, err := gateway.NewPostgresSimopsStore(cfg.Simops.PostgresDSN)
	if err != nil {
		return fmt.Errorf("initialize iceberg writer store: %w", err)
	}
	writer, err := gateway.NewSimopsArtifactWriter(cfg.Simops, store, time.Now)
	if err != nil {
		return fmt.Errorf("invalid iceberg writer configuration: %w", err)
	}
	eventLog := gateway.SimopsEventLog(gateway.MemorySimopsEventLog{Store: store})
	if cfg.Simops.TelemetryLog == "redpanda" {
		redpandaLog, err := gateway.NewRedpandaEventLog(cfg.Simops, store)
		if err != nil {
			return fmt.Errorf("initialize iceberg writer redpanda event log: %w", err)
		}
		eventLog = redpandaLog
	}
	processor := gateway.NewSimopsArtifactIntentProcessor(writer, eventLog, cfg.Simops.RedpandaTopic, cfg.Simops.IcebergBatchSize, time.Now)
	metrics := gateway.NewSimopsConsumerMetrics()

	addr := getenv("SIMOPS_ICEBERG_WRITER_ADDR", ":9460")
	process, err := gateway.NewBackgroundConsumerProcess(gateway.BackgroundConsumerProcessConfig{
		Name: "simops-iceberg-writer", Address: addr, MetricsPrefix: "simops_iceberg_writer",
		Metrics: metrics, ShutdownTimeout: 5 * time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{
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
				"implementation_note": "consumer owns artifact commits; manifest mode is not a real Iceberg append",
			}
		},
		Consumers: []gateway.BackgroundConsumer{{Name: "simops-telemetry", Consume: func(ctx context.Context) error {
			return gateway.RunArtifactIntentConsumer(ctx, cfg.Simops, nil, processor, metrics)
		}}},
	})
	if err != nil {
		return fmt.Errorf("configure simops iceberg writer process: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("simops iceberg writer contract service listening on %s for warehouse %s", addr, cfg.Simops.IcebergWarehouse)
	return process.Run(ctx)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
