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

type backgroundConsumerGroup struct {
	cancel    context.CancelFunc
	results   <-chan *BackgroundConsumerError
	remaining int
}

func validateBackgroundConsumers(consumers []BackgroundConsumer) error {
	if len(consumers) == 0 {
		return fmt.Errorf("background consumer group requires at least one consumer")
	}
	for _, consumer := range consumers {
		if strings.TrimSpace(consumer.Name) == "" || consumer.Consume == nil {
			return fmt.Errorf("background consumer group requires named consumers")
		}
	}
	return nil
}

func startBackgroundConsumers(ctx context.Context, consumers []BackgroundConsumer) (*backgroundConsumerGroup, error) {
	if err := validateBackgroundConsumers(consumers); err != nil {
		return nil, err
	}
	groupCtx, cancel := context.WithCancel(ctx)
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
	return &backgroundConsumerGroup{cancel: cancel, results: results, remaining: len(consumers)}, nil
}

func RunBackgroundConsumers(ctx context.Context, consumers ...BackgroundConsumer) error {
	group, err := startBackgroundConsumers(ctx, consumers)
	if err != nil {
		return err
	}
	defer group.cancel()

	var firstFailure error
	done := ctx.Done()
	for group.remaining > 0 {
		select {
		case result := <-group.results:
			group.remaining--
			if firstFailure == nil && ctx.Err() == nil && result.Cause != nil && !errors.Is(result.Cause, context.Canceled) {
				firstFailure = result
				group.cancel()
			}
		case <-done:
			group.cancel()
			done = nil
		}
	}
	if firstFailure != nil {
		return firstFailure
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
	if err := validateBackgroundConsumers(cfg.Consumers); err != nil {
		return nil, fmt.Errorf("background consumer process %s: %w", cfg.Name, err)
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

	group, err := startBackgroundConsumers(context.Background(), p.cfg.Consumers)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("start background consumer %s group: %w", p.cfg.Name, err)
	}
	defer group.cancel()
	server := &http.Server{
		Handler:           p.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverResult := make(chan error, 1)
	go func() { serverResult <- server.Serve(listener) }()

	select {
	case consumerErr := <-group.results:
		group.remaining--
		if ctx.Err() != nil {
			group.cancel()
			return p.shutdown(server, group)
		}
		p.recordFailure(consumerErr)
		group.cancel()
		return errors.Join(consumerErr, p.shutdown(server, group))
	case serverErr := <-serverResult:
		if errors.Is(serverErr, http.ErrServerClosed) && ctx.Err() != nil {
			group.cancel()
			return p.shutdown(server, group)
		}
		if serverErr == nil {
			serverErr = fmt.Errorf("background consumer %s HTTP server exited unexpectedly", p.cfg.Name)
		}
		p.recordFailure(serverErr)
		group.cancel()
		return errors.Join(serverErr, p.shutdown(server, group))
	case <-ctx.Done():
		group.cancel()
		return p.shutdown(server, group)
	}
}

func (p *BackgroundConsumerProcess) shutdown(server *http.Server, group *backgroundConsumerGroup) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), p.cfg.ShutdownTimeout)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Shutdown(shutdownCtx) }()

	var shutdownErr error
	for group.remaining > 0 || serverDone != nil {
		select {
		case result := <-group.results:
			group.remaining--
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
		if p.cfg.Metrics.Snapshot().LastError != "" {
			writeBackgroundConsumerJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "failed"})
			return
		}
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
