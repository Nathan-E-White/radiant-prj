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
	Consumers       []BackgroundConsumer
}

type BackgroundConsumer struct {
	Name    string
	Consume func(context.Context) error
}

type BackgroundConsumerError struct {
	Consumer string
	Cause    error
}

func (e *BackgroundConsumerError) Error() string {
	return fmt.Sprintf("background consumer %s failed: %v", e.Consumer, e.Cause)
}

func (e *BackgroundConsumerError) Unwrap() error {
	return e.Cause
}

func RunBackgroundConsumers(ctx context.Context, consumers ...BackgroundConsumer) error {
	if len(consumers) == 0 {
		return fmt.Errorf("background consumer group requires at least one consumer")
	}
	for _, consumer := range consumers {
		if strings.TrimSpace(consumer.Name) == "" || consumer.Consume == nil {
			return fmt.Errorf("background consumer group requires named consumers")
		}
	}

	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan *BackgroundConsumerError, len(consumers))
	for _, consumer := range consumers {
		consumer := consumer
		go func() {
			err := consumer.Consume(groupCtx)
			if err == nil && groupCtx.Err() == nil {
				err = fmt.Errorf("exited unexpectedly")
			}
			results <- &BackgroundConsumerError{Consumer: consumer.Name, Cause: err}
		}()
	}

	remaining := len(consumers)
	for remaining > 0 {
		select {
		case result := <-results:
			remaining--
			if ctx.Err() == nil && result.Cause != nil && !errors.Is(result.Cause, context.Canceled) {
				cancel()
				return result
			}
		case <-ctx.Done():
			cancel()
		}
	}
	return ctx.Err()
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
	if len(cfg.Consumers) == 0 {
		return nil, fmt.Errorf("background consumer process %s requires at least one consumer", cfg.Name)
	}
	for _, consumer := range cfg.Consumers {
		if strings.TrimSpace(consumer.Name) == "" || consumer.Consume == nil {
			return nil, fmt.Errorf("background consumer process %s requires named consumers", cfg.Name)
		}
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
	consumerResult := make(chan *BackgroundConsumerError, len(p.cfg.Consumers))
	serverResult := make(chan error, 1)
	for _, consumer := range p.cfg.Consumers {
		consumer := consumer
		go func() {
			err := consumer.Consume(processCtx)
			if err == nil && processCtx.Err() == nil {
				err = fmt.Errorf("exited unexpectedly")
			}
			consumerResult <- &BackgroundConsumerError{Consumer: consumer.Name, Cause: err}
		}()
	}
	go func() { serverResult <- server.Serve(listener) }()

	select {
	case consumerErr := <-consumerResult:
		if ctx.Err() != nil {
			cancel()
			return p.shutdown(server, consumerResult, len(p.cfg.Consumers)-1)
		}
		p.recordFailure(consumerErr)
		cancel()
		return errors.Join(consumerErr, p.shutdown(server, consumerResult, len(p.cfg.Consumers)-1))
	case serverErr := <-serverResult:
		if errors.Is(serverErr, http.ErrServerClosed) && ctx.Err() != nil {
			cancel()
			return p.shutdown(server, consumerResult, len(p.cfg.Consumers))
		}
		if serverErr == nil {
			serverErr = fmt.Errorf("background consumer %s HTTP server exited unexpectedly", p.cfg.Name)
		}
		p.recordFailure(serverErr)
		cancel()
		return errors.Join(serverErr, p.shutdown(server, consumerResult, len(p.cfg.Consumers)))
	case <-ctx.Done():
		cancel()
		return p.shutdown(server, consumerResult, len(p.cfg.Consumers))
	}
}

func (p *BackgroundConsumerProcess) shutdown(server *http.Server, consumerResult <-chan *BackgroundConsumerError, consumersRemaining int) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), p.cfg.ShutdownTimeout)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Shutdown(shutdownCtx) }()

	var shutdownErr error
	for consumersRemaining > 0 || serverDone != nil {
		select {
		case result := <-consumerResult:
			consumersRemaining--
			if result.Cause != nil && !errors.Is(result.Cause, context.Canceled) && !errors.Is(result.Cause, context.DeadlineExceeded) {
				shutdownErr = errors.Join(shutdownErr, result)
			}
		case err := <-serverDone:
			serverDone = nil
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return errors.Join(shutdownErr, p.forceCloseAfterDeadline(server, shutdownCtx.Err()))
			}
			if err != nil {
				return fmt.Errorf("shut down background consumer %s HTTP server: %w", p.cfg.Name, err)
			}
		case <-shutdownCtx.Done():
			return errors.Join(shutdownErr, p.forceCloseAfterDeadline(server, shutdownCtx.Err()))
		}
	}
	return shutdownErr
}

func (p *BackgroundConsumerProcess) forceCloseAfterDeadline(server *http.Server, cause error) error {
	_ = server.Close()
	if cause == nil {
		cause = context.DeadlineExceeded
	}
	err := &BackgroundConsumerShutdownError{Process: p.cfg.Name, Cause: cause}
	p.recordFailure(err)
	return err
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
			if snapshot.LastError != "" {
				state = "failed"
			}
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
