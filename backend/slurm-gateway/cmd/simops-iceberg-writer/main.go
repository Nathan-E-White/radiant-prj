package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid iceberg writer configuration: %v", err)
	}

	addr := getenv("SIMOPS_ICEBERG_WRITER_ADDR", ":9460")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":             "ready",
			"catalog":            cfg.Simops.IcebergCatalog,
			"warehouse":          cfg.Simops.IcebergWarehouse,
			"s3_endpoint":        cfg.Simops.IcebergS3Endpoint,
			"redpanda_topic":     cfg.Simops.RedpandaTopic,
			"writer_mode":        cfg.Simops.IcebergWriterMode,
			"implementation_gap": "Iceberg Rust batch commit worker is an external adapter target for this service",
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP simops_iceberg_writer_ready Iceberg writer readiness state.\n# TYPE simops_iceberg_writer_ready gauge\nsimops_iceberg_writer_ready 1\n"))
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("simops iceberg writer contract service listening on %s for warehouse %s", addr, cfg.Simops.IcebergWarehouse)
	log.Fatal(server.ListenAndServe())
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
