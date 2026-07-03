package main

import (
	"log"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid gateway configuration: %v", err)
	}

	app, err := gateway.NewDefaultGateway(cfg)
	if err != nil {
		log.Fatalf("failed to initialize gateway: %v", err)
	}

	server := gateway.NewHTTPServer(cfg, app.Handler())

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
