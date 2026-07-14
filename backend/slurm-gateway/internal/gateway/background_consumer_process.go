package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type BackgroundConsumerProcessConfig struct {
	Name            string
	Address         string
	MetricsPrefix   string
	Metrics         *SimopsConsumerMetrics
	ShutdownTimeout time.Duration
	ReadyDetails    func() map[string]any
	Consume         func(context.Context) error
}

type BackgroundConsumerShutdownError struct {
	Process string
	Cause   error
}

func (e *BackgroundConsumerShutdownError) Error() string {
	return fmt.Sprintf("background consumer %s exceeded its shutdown bound: %v", e.Process, e.Cause)
}

func (e *BackgroundConsumerShutdownError) Unwrap() error {
	return e.Cause
}

type BackgroundConsumerProcess struct {
	cfg     BackgroundConsumerProcessConfig
	started chan struct{}
	mu      sync.RWMutex
	url     string
}

func NewBackgroundConsumerProcess(cfg BackgroundConsumerProcessConfig) (*BackgroundConsumerProcess, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, fmt.Errorf("background consumer process requires a name")
	}
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("background consumer process %s requires an address", cfg.Name)
	}
	if strings.TrimSpace(cfg.MetricsPrefix) == "" {
		return nil, fmt.Errorf("background consumer process %s requires a metrics prefix", cfg.Name)
	}
	if cfg.Metrics == nil {
		return nil, fmt.Errorf("background consumer process %s requires metrics", cfg.Name)
	}
	if cfg.ShutdownTimeout <= 0 {
		return nil, fmt.Errorf("background consumer process %s requires a positive shutdown timeout", cfg.Name)
	}
	if cfg.Consume == nil {
		return nil, fmt.Errorf("background consumer process %s requires a consumer", cfg.Name)
	}
	return &BackgroundConsumerProcess{cfg: cfg, started: make(chan struct{})}, nil
}

func (p *BackgroundConsumerProcess) Started() <-chan struct{} {
	return p.started
}

func (p *BackgroundConsumerProcess) URL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.url
}

func (p *BackgroundConsumerProcess) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", p.cfg.Address)
	if err != nil {
		return fmt.Errorf("start background consumer %s listener: %w", p.cfg.Name, err)
	}
	p.mu.Lock()
	p.url = "http://" + listener.Addr().String()
	p.mu.Unlock()
	close(p.started)

	processCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &http.Server{
		Handler:           p.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	consumerResult := make(chan error, 1)
	serverResult := make(chan error, 1)
	go func() { consumerResult <- p.cfg.Consume(processCtx) }()
	go func() { serverResult <- server.Serve(listener) }()

	select {
	case consumerErr := <-consumerResult:
		if ctx.Err() != nil && (consumerErr == nil || errors.Is(consumerErr, context.Canceled)) {
			return p.shutdown(server, consumerResult, true)
		}
		if consumerErr == nil {
			consumerErr = fmt.Errorf("background consumer %s exited unexpectedly", p.cfg.Name)
		}
		p.recordFailure(consumerErr)
		cancel()
		return errors.Join(consumerErr, p.shutdown(server, consumerResult, true))
	case serverErr := <-serverResult:
		if errors.Is(serverErr, http.ErrServerClosed) && ctx.Err() != nil {
			return p.shutdown(server, consumerResult, false)
		}
		if serverErr == nil {
			serverErr = fmt.Errorf("background consumer %s HTTP server exited unexpectedly", p.cfg.Name)
		}
		p.recordFailure(serverErr)
		cancel()
		return errors.Join(serverErr, p.shutdown(server, consumerResult, false))
	case <-ctx.Done():
		cancel()
		return p.shutdown(server, consumerResult, false)
	}
}

func (p *BackgroundConsumerProcess) shutdown(server *http.Server, consumerResult <-chan error, consumerDone bool) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), p.cfg.ShutdownTimeout)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Shutdown(shutdownCtx) }()

	for !consumerDone || serverDone != nil {
		select {
		case <-consumerResult:
			consumerDone = true
		case err := <-serverDone:
			serverDone = nil
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("shut down background consumer %s HTTP server: %w", p.cfg.Name, err)
			}
		case <-shutdownCtx.Done():
			_ = server.Close()
			err := &BackgroundConsumerShutdownError{Process: p.cfg.Name, Cause: shutdownCtx.Err()}
			p.recordFailure(err)
			return err
		}
	}
	return nil
}

func (p *BackgroundConsumerProcess) recordFailure(err error) {
	p.cfg.Metrics.MarkBrokerConnected(false)
	p.cfg.Metrics.SetLastError(err)
}

func (p *BackgroundConsumerProcess) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeBackgroundConsumerJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		snapshot := p.cfg.Metrics.Snapshot()
		status := http.StatusOK
		state := "ready"
		if !snapshot.Ready() {
			status = http.StatusServiceUnavailable
			state = "starting"
		}
		payload := map[string]any{}
		if p.cfg.ReadyDetails != nil {
			for key, value := range p.cfg.ReadyDetails() {
				payload[key] = value
			}
		}
		payload["status"] = state
		payload["metrics"] = snapshot
		writeBackgroundConsumerJSON(w, status, payload)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(p.cfg.Metrics.Snapshot().Prometheus(p.cfg.MetricsPrefix)))
	})
	return mux
}

func writeBackgroundConsumerJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
