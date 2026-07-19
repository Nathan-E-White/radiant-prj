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
		return fmt.Errorf("invalid workbench iceberg writer configuration: %w", err)
	}
	writer, err := gateway.NewWorkbenchIcebergWriter(cfg.Workbench)
	if err != nil {
		return fmt.Errorf("initialize workbench iceberg writer: %w", err)
	}
	metrics := gateway.NewSimopsConsumerMetrics()
	metrics.RequireBrokerConnections(
		string(gateway.ProjectionStreamMeasuredState),
		string(gateway.ProjectionStreamSimulatedResultState),
		string(gateway.ProjectionStreamTwinState),
	)
	addr := getenv("WORKBENCH_ICEBERG_WRITER_ADDR", ":9490")
	process, err := gateway.NewBackgroundConsumerProcess(gateway.BackgroundConsumerProcessConfig{
		Name: "workbench-iceberg-writer", Address: addr, MetricsPrefix: "workbench_iceberg_writer",
		Metrics: metrics, ShutdownTimeout: 5 * time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{
				"tables":    []string{"scada.measured_frames", "simops.simulated_results", "digital_twin.state_values"},
				"warehouse": cfg.Workbench.IcebergWarehouse,
			}
		},
		Consumers: []gateway.BackgroundConsumer{
			gateway.BackgroundConsumer{Name: "measured-state", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchScadaIcebergConsumer(ctx, cfg.Workbench, nil, writer, metrics)
			}},
			gateway.BackgroundConsumer{Name: "simulated-result-state", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchResultIcebergConsumer(ctx, cfg.Workbench, nil, writer, metrics)
			}},
			gateway.BackgroundConsumer{Name: "twin-state-and-lineage", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchTwinIcebergConsumer(ctx, cfg.Workbench, nil, writer, metrics)
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("configure workbench iceberg writer process: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("workbench iceberg writer listening on %s", addr)
	return process.Run(ctx)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
