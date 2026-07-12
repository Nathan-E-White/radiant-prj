package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"

	"radiant/slurm-gateway/internal/gateway"
)

func main() {
	endpoint := flag.String("endpoint", "https://127.0.0.1:9443/moq/simops", "WebTransport endpoint URL")
	runID := flag.String("run-id", "", "Run id to match")
	timeout := flag.Duration("timeout", 20*time.Second, "Probe timeout")
	caCert := flag.String("ca-cert", "", "PEM CA certificate file for the WebTransport server")
	serverName := flag.String("server-name", "", "TLS server name override for WebTransport certificate verification")
	flag.Parse()

	if strings.TrimSpace(*runID) == "" {
		fmt.Fprintln(os.Stderr, "--run-id is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := probe(ctx, *endpoint, *runID, *caCert, *serverName); err != nil {
		fmt.Fprintf(os.Stderr, "SimOps WebTransport probe failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SimOps WebTransport probe observed telemetry and quality tracks for %s.\n", *runID)
}

func probe(ctx context.Context, endpoint string, runID string, caCertFile string, serverName string) error {
	tlsConfig, err := gateway.LoadWebTransportClientTLSConfig(caCertFile, serverName)
	if err != nil {
		return err
	}
	dialer := &webtransport.Dialer{
		TLSClientConfig: tlsConfig,
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
		},
		ApplicationProtocols: []string{"moq-webtransport", "simops.telemetry.v1"},
	}
	defer dialer.Close()

	response, session, err := dialer.Dial(ctx, endpoint, http.Header{})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("unexpected WebTransport status %d", response.StatusCode)
	}
	defer session.CloseWithError(0, "")

	telemetry := false
	quality := false
	for !(telemetry && quality) {
		stream, err := session.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		payload, err := io.ReadAll(stream)
		if err != nil {
			return err
		}
		var message gateway.SimopsMoQWireMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			return fmt.Errorf("decode WebTransport track payload %q: %w", strings.TrimSpace(string(payload)), err)
		}
		if message.RunID != runID {
			continue
		}
		switch {
		case strings.HasSuffix(message.Track, "/telemetry"):
			telemetry = true
		case strings.HasSuffix(message.Track, "/quality"):
			quality = true
		}
	}
	return nil
}
