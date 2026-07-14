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
		return fmt.Errorf("invalid timescale writer configuration: %w", err)
	}
	store, err := gateway.NewTimescaleTelemetryStore(cfg.Simops.PostgresDSN)
	if err != nil {
		return fmt.Errorf("initialize timescale telemetry store: %w", err)
	}
	defer store.Close()

	metrics := gateway.NewSimopsConsumerMetrics()
	addr := getenv("SIMOPS_TIMESCALE_WRITER_ADDR", ":9450")
	process, err := gateway.NewBackgroundConsumerProcess(gateway.BackgroundConsumerProcessConfig{
		Name:            "simops-timescale-writer",
		Address:         addr,
		MetricsPrefix:   "simops_timescale_writer",
		Metrics:         metrics,
		ShutdownTimeout: 5 * time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{
				"consumer_group": cfg.Simops.TimescaleConsumerGroup,
				"redpanda_topic": cfg.Simops.RedpandaTopic,
			}
		},
		Consume: func(ctx context.Context) error {
			return gateway.RunTimescaleTelemetryConsumer(ctx, cfg.Simops, nil, store, metrics)
		},
	})
	if err != nil {
		return fmt.Errorf("configure timescale writer process: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("simops timescale writer listening on %s for topic %s group %s", addr, cfg.Simops.RedpandaTopic, cfg.Simops.TimescaleConsumerGroup)
	return process.Run(ctx)
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
