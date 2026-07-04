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
		log.Fatalf("invalid stream gateway configuration: %v", err)
	}

	addr := getenv("SIMOPS_STREAM_GATEWAY_ADDR", ":9443")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":          "ready",
			"protocol":        "moq-webtransport",
			"redpanda_topic":  cfg.Simops.RedpandaTopic,
			"redpanda_broker": cfg.Simops.RedpandaBrokers,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP simops_stream_gateway_ready Stream gateway readiness state.\n# TYPE simops_stream_gateway_ready gauge\nsimops_stream_gateway_ready 1\n"))
	})
	mux.HandleFunc("/moq/simops", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":    "MoQ/WebTransport adapter boundary is configured but requires the WebTransport/MoQ implementation module",
			"protocol": "moq-webtransport",
		})
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("simops stream gateway contract service listening on %s for %s", addr, cfg.Simops.MoQWebTransportURL)
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
