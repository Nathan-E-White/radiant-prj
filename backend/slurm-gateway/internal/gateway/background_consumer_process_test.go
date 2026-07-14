package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBackgroundConsumerProcessExposesLifecycleAndStopsOnCancellation(t *testing.T) {
	metrics := NewSimopsConsumerMetrics()
	consumerStarted := make(chan struct{})
	allowReady := make(chan struct{})
	process, err := NewBackgroundConsumerProcess(BackgroundConsumerProcessConfig{
		Name:            "test-writer",
		Address:         "127.0.0.1:0",
		MetricsPrefix:   "test_writer",
		Metrics:         metrics,
		ShutdownTimeout: time.Second,
		ReadyDetails: func() map[string]any {
			return map[string]any{"consumer_group": "test-group", "status": "domain-override", "metrics": "domain-override"}
		},
		Consume: func(ctx context.Context) error {
			close(consumerStarted)
			<-allowReady
			metrics.MarkBrokerConnected(true)
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("construct process: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- process.Run(ctx) }()
	waitForProcessStart(t, process)
	<-consumerStarted

	assertProcessEndpoint(t, process.URL()+"/healthz", http.StatusOK, `"status":"ok"`)
	assertProcessEndpoint(t, process.URL()+"/readyz", http.StatusServiceUnavailable, `"status":"starting"`)
	close(allowReady)
	for !metrics.Snapshot().Ready() {
		time.Sleep(time.Millisecond)
	}
	assertProcessEndpoint(t, process.URL()+"/readyz", http.StatusOK, `"status":"ready"`, `"consumer_group":"test-group"`)
	assertProcessEndpoint(t, process.URL()+"/metrics", http.StatusOK, "test_writer_broker_connected 1")

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("signal cancellation should stop cleanly: %v", err)
		}
		assertProcessStopped(t, process.URL())
	case <-time.After(2 * time.Second):
		t.Fatal("signal cancellation did not stop process")
	}
}

func TestBackgroundConsumerProcessPublishesConsumerFailureAndStops(t *testing.T) {
	metrics := NewSimopsConsumerMetrics()
	failure := errors.New("broker fetch failed")
	release := make(chan struct{})
	process, err := NewBackgroundConsumerProcess(BackgroundConsumerProcessConfig{
		Name: "failed-writer", Address: "127.0.0.1:0", MetricsPrefix: "failed_writer",
		Metrics: metrics, ShutdownTimeout: time.Second,
		Consume: func(context.Context) error {
			metrics.MarkBrokerConnected(true)
			<-release
			return failure
		},
	})
	if err != nil {
		t.Fatalf("construct process: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- process.Run(context.Background()) }()
	waitForProcessStart(t, process)
	close(release)

	if err := <-result; !errors.Is(err, failure) {
		t.Fatalf("expected consumer failure, got %v", err)
	}
	snapshot := metrics.Snapshot()
	if snapshot.BrokerConnected || snapshot.LastError != failure.Error() || snapshot.Ready() {
		t.Fatalf("expected externally visible failed metrics state, got %#v", snapshot)
	}
}

func TestBackgroundConsumerProcessBoundsUncooperativeConsumerShutdown(t *testing.T) {
	metrics := NewSimopsConsumerMetrics()
	release := make(chan struct{})
	process, err := NewBackgroundConsumerProcess(BackgroundConsumerProcessConfig{
		Name: "blocked-writer", Address: "127.0.0.1:0", MetricsPrefix: "blocked_writer",
		Metrics: metrics, ShutdownTimeout: 20 * time.Millisecond,
		Consume: func(context.Context) error {
			metrics.MarkBrokerConnected(true)
			<-release
			return nil
		},
	})
	if err != nil {
		t.Fatalf("construct process: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- process.Run(ctx) }()
	waitForProcessStart(t, process)

	started := time.Now()
	cancel()
	select {
	case err := <-result:
		var shutdownErr *BackgroundConsumerShutdownError
		if !errors.As(err, &shutdownErr) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected bounded shutdown error, got %T %v", err, err)
		}
		if time.Since(started) > time.Second {
			t.Fatalf("shutdown exceeded bound: %v", time.Since(started))
		}
	case <-time.After(time.Second):
		t.Fatal("uncooperative consumer escaped shutdown bound")
	}
	close(release)
}

func TestBackgroundConsumerProcessRejectsIncompleteConfiguration(t *testing.T) {
	valid := BackgroundConsumerProcessConfig{
		Name: "writer", Address: "127.0.0.1:0", MetricsPrefix: "writer",
		Metrics: NewSimopsConsumerMetrics(), ShutdownTimeout: time.Second,
		Consume: func(context.Context) error { return nil },
	}
	tests := []struct {
		name   string
		change func(*BackgroundConsumerProcessConfig)
		want   string
	}{
		{name: "name", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.Name = "" }, want: "name"},
		{name: "address", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.Address = "" }, want: "address"},
		{name: "metrics prefix", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.MetricsPrefix = "" }, want: "metrics prefix"},
		{name: "metrics", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.Metrics = nil }, want: "metrics"},
		{name: "shutdown timeout", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.ShutdownTimeout = 0 }, want: "shutdown timeout"},
		{name: "consumer", change: func(cfg *BackgroundConsumerProcessConfig) { cfg.Consume = nil }, want: "consumer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.change(&cfg)
			_, err := NewBackgroundConsumerProcess(cfg)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %s configuration error, got %v", test.want, err)
			}
		})
	}
}

func waitForProcessStart(t *testing.T, process *BackgroundConsumerProcess) {
	t.Helper()
	select {
	case <-process.Started():
	case <-time.After(time.Second):
		t.Fatal("process did not start listening")
	}
}

func assertProcessEndpoint(t *testing.T, url string, wantStatus int, fragments ...string) {
	t.Helper()
	response, err := http.Get(url) //nolint:gosec -- loopback test server
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("GET %s: status=%d body=%s", url, response.StatusCode, body)
	}
	for _, fragment := range fragments {
		if !strings.Contains(string(body), fragment) {
			t.Fatalf("GET %s missing %q in %s", url, fragment, body)
		}
	}
	if strings.HasSuffix(url, "/readyz") {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("ready response is not JSON: %v", err)
		}
	}
}

func assertProcessStopped(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	response, err := client.Get(url + "/healthz")
	if err == nil {
		response.Body.Close()
		t.Fatalf("expected stopped process at %s", url)
	}
}
