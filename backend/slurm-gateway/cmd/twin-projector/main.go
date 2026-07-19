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
		return fmt.Errorf("invalid twin projector configuration: %w", err)
	}
	store, err := gateway.NewPostgresWorkbenchStore(cfg.Workbench.PostgresDSN)
	if err != nil {
		return fmt.Errorf("initialize workbench postgres store: %w", err)
	}
	eventLog, err := gateway.NewRedpandaWorkbenchEventLog(cfg.Workbench)
	if err != nil {
		return fmt.Errorf("initialize workbench redpanda event log: %w", err)
	}
	metrics := gateway.NewSimopsConsumerMetrics()
	addr := getenv("TWIN_PROJECTOR_ADDR", ":9480")
	process, err := gateway.NewBackgroundConsumerProcess(gateway.BackgroundConsumerProcessConfig{
		Name: "twin-projector", Address: addr, MetricsPrefix: "twin_projector",
		Metrics: metrics, ShutdownTimeout: 5 * time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{
				"input_topics": []string{cfg.Workbench.ScadaTopic, cfg.Workbench.ResultsTopic},
				"output_topic": cfg.Workbench.TwinStateTopic,
			}
		},
		Consumers: []gateway.BackgroundConsumer{{Name: "twin-state-and-lineage", SkipReadiness: true, Consume: func(ctx context.Context) error {
			return gateway.RunTwinProjector(ctx, cfg.Workbench, nil, nil, store, eventLog, metrics)
		}}},
	})
	if err != nil {
		return fmt.Errorf("configure twin projector process: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("twin projector listening on %s", addr)
	return process.Run(ctx)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
