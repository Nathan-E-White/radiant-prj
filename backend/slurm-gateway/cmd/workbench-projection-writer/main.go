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
		return fmt.Errorf("invalid workbench projection writer configuration: %w", err)
	}
	store, err := gateway.NewPostgresWorkbenchStore(cfg.Workbench.PostgresDSN)
	if err != nil {
		return fmt.Errorf("initialize workbench postgres store: %w", err)
	}
	metrics := gateway.NewSimopsConsumerMetrics()
	addr := getenv("WORKBENCH_PROJECTION_WRITER_ADDR", ":9470")
	process, err := gateway.NewBackgroundConsumerProcess(gateway.BackgroundConsumerProcessConfig{
		Name: "workbench-projection-writer", Address: addr, MetricsPrefix: "workbench_projection_writer",
		Metrics: metrics, ShutdownTimeout: 5 * time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{"topics": []string{cfg.Workbench.ScadaTopic, cfg.Workbench.ResultsTopic, cfg.Workbench.TwinStateTopic}}
		},
		Consumers: []gateway.BackgroundConsumer{
			gateway.BackgroundConsumer{Name: "measured-state", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchScadaProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
			}},
			gateway.BackgroundConsumer{Name: "simulated-result-state", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchResultProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
			}},
			gateway.BackgroundConsumer{Name: "twin-state-and-lineage", Consume: func(ctx context.Context) error {
				return gateway.RunWorkbenchTwinProjectionConsumer(ctx, cfg.Workbench, nil, store, metrics)
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("configure workbench projection writer process: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("workbench projection writer listening on %s", addr)
	return process.Run(ctx)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
