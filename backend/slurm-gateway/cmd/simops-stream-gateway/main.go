package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	cfg, err := gateway.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("invalid stream gateway configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := getenv("SIMOPS_STREAM_GATEWAY_ADDR", ":9443")
	router := gateway.NewSimopsMoQTrackRouter()
	hub := gateway.NewSimopsMoQTrackHub()
	metrics := gateway.NewSimopsConsumerMetrics()
	go func() {
		if err := gateway.RunMoQTrackConsumer(ctx, cfg.Simops, nil, router, metrics, hub); err != nil {
			log.Printf("moq track consumer stopped: %v", err)
			metrics.MarkBrokerConnected(false)
			metrics.SetLastError(err)
		}
	}()

	tlsConfig, fingerprint, err := localWebTransportTLSConfig()
	if err != nil {
		log.Fatalf("prepare webtransport tls: %v", err)
	}
	wtMux := http.NewServeMux()
	h3Server := &http3.Server{
		Addr:      addr,
		Handler:   wtMux,
		TLSConfig: http3.ConfigureTLSConfig(tlsConfig),
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
		},
	}
	webtransport.ConfigureHTTP3Server(h3Server)
	wtServer := &webtransport.Server{
		ApplicationProtocols: []string{"moq-webtransport", "simops.telemetry.v1"},
		H3:                   h3Server,
		CheckOrigin:          func(*http.Request) bool { return true },
	}
	wtMux.HandleFunc("/moq/simops", handleWebTransportTracks(ctx, wtServer, router, hub, metrics))
	wtMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "transport": "webtransport"})
	})
	wtMux.HandleFunc("/certificate.sha256", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, fingerprint+"\n")
	})
	go func() {
		if err := wtServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("simops webtransport gateway stopped: %v", err)
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
		}
	}()

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
			"status":          state,
			"protocol":        "moq-webtransport",
			"consumer_group":  cfg.Simops.MoQConsumerGroup,
			"redpanda_topic":  cfg.Simops.RedpandaTopic,
			"redpanda_broker": cfg.Simops.RedpandaBrokers,
			"tracks_buffered": len(router.Snapshot()),
			"metrics":         snapshot,
		})
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.Snapshot().Prometheus("simops_moq_gateway")))
	})
	mux.HandleFunc("/moq/simops", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":    "connect to this endpoint with WebTransport over HTTP/3/QUIC",
			"protocol": "moq-webtransport",
			"cert":     "/certificate.sha256",
		})
	})
	mux.HandleFunc("/certificate.sha256", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, fingerprint+"\n")
	})
	mux.HandleFunc("/debug/simops/tracks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"protocol": "moq-webtransport",
			"tracks":   router.Snapshot(),
		})
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = wtServer.Close()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("simops moq gateway listening on tcp/udp %s for %s", addr, cfg.Simops.MoQWebTransportURL)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func handleWebTransportTracks(ctx context.Context, server *webtransport.Server, router *gateway.SimopsMoQTrackRouter, hub *gateway.SimopsMoQTrackHub, metrics *gateway.SimopsConsumerMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := server.Upgrade(w, r)
		if err != nil {
			log.Printf("webtransport upgrade failed: %v", err)
			return
		}
		ch, cancel, subscriberID := hub.Subscribe(128)
		defer func() {
			cancel()
			metrics.SetSubscriberCount(hub.SubscriberCount())
		}()
		metrics.SetSubscriberCount(hub.SubscriberCount())
		log.Printf("simops webtransport subscriber %d connected from %s", subscriberID, r.RemoteAddr)

		for _, message := range router.Snapshot() {
			if err := sendTrackMessage(ctx, session, message); err != nil {
				metrics.IncWriteFailures()
				metrics.SetLastError(err)
				_ = session.CloseWithError(1, err.Error())
				return
			}
		}
		for {
			select {
			case <-ctx.Done():
				_ = session.CloseWithError(0, "server shutdown")
				return
			case <-session.Context().Done():
				return
			case message, ok := <-ch:
				if !ok {
					return
				}
				if err := sendTrackMessage(ctx, session, message); err != nil {
					metrics.IncWriteFailures()
					metrics.SetLastError(err)
					_ = session.CloseWithError(1, err.Error())
					return
				}
			}
		}
	}
}

func sendTrackMessage(ctx context.Context, session *webtransport.Session, message gateway.SimopsMoQTrackMessage) error {
	payload, err := json.Marshal(gateway.NewSimopsMoQWireMessage(message))
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	stream, err := session.OpenUniStreamSync(ctx)
	if err != nil {
		return err
	}
	if _, err := stream.Write(payload); err != nil {
		_ = stream.Close()
		return err
	}
	return stream.Close()
}

func localWebTransportTLSConfig() (*tls.Config, string, error) {
	cert, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "radiant-simops-local-webtransport",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &cert.PublicKey, cert)
	if err != nil {
		return nil, "", err
	}
	fingerprint := sha256.Sum256(der)
	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  cert,
		}},
		NextProtos: []string{"h3"},
		MinVersion: tls.VersionTLS13,
	}, hex.EncodeToString(fingerprint[:]), nil
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
