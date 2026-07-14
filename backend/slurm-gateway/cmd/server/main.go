package main

import (
	"context"
	"log"
	"time"

	"radiant/slurm-gateway/internal/gateway"
	"radiant/slurm-gateway/internal/simopsdocker"
	"radiant/slurm-gateway/internal/simopskubernetes"
)

func main() {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid gateway configuration: %v", err)
	}

	var simopsSpooler gateway.SimopsSpooler
	if cfg.Simops.Enabled && cfg.Simops.WorkerRuntime == "docker" {
		spooler, err := simopsdocker.NewSpooler(cfg.Simops)
		if err != nil {
			log.Fatalf("failed to initialize Docker SimOps runtime adapter: %v", err)
		}
		simopsSpooler = spooler
	}
	if cfg.Simops.Enabled && cfg.Simops.WorkerRuntime == "kubernetes" {
		spooler, err := simopskubernetes.NewSpooler(cfg.Simops)
		if err != nil {
			log.Fatalf("failed to initialize Kubernetes SimOps runtime adapter: %v", err)
		}
		simopsSpooler = spooler
	}
	var reactorTelemetryRuntime gateway.ReactorTelemetryRuntime
	if cfg.ReactorTelemetry.Enabled && cfg.ReactorTelemetry.Runtime == "docker" {
		runtime, err := simopsdocker.NewReactorTelemetryRuntime(cfg.ReactorTelemetry.WorkerImage, cfg.ReactorTelemetry.WorkerNetwork)
		if err != nil {
			log.Fatalf("failed to initialize Docker Reactor Telemetry runtime adapter: %v", err)
		}
		reactorTelemetryRuntime = runtime
	}

	app, err := gateway.NewDefaultGatewayWithRuntimes(cfg, simopsSpooler, reactorTelemetryRuntime)
	if err != nil {
		log.Fatalf("failed to initialize gateway: %v", err)
	}

	server := gateway.NewHTTPServer(cfg, app.Handler())
	if cfg.ReactorTelemetry.Enabled {
		go func() {
			ticker := time.NewTicker(cfg.ReactorTelemetry.ReconcileInterval)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
				err := app.ReconcileExpiredReactorTelemetry(ctx)
				cancel()
				if err != nil {
					log.Printf("reactor telemetry expiry reconciliation failed: %v", err)
				}
			}
		}()
	}

	if cfg.TLSEnabled() {
		tlsConfig, err := gateway.LoadTLSConfig(cfg)
		if err != nil {
			log.Fatalf("failed to load mTLS configuration: %v", err)
		}
		server.TLSConfig = tlsConfig
		log.Printf("slurm gateway listening with mTLS on %s in %s mode", cfg.Addr, cfg.Mode)
		log.Fatal(server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile))
	}

	log.Printf("slurm gateway listening without TLS transport on %s in %s mode; job handlers still require peer certificates unless disabled", cfg.Addr, cfg.Mode)
	log.Fatal(server.ListenAndServe())
}
