//go:build dockerintegration

package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	dockerclient "github.com/moby/moby/client"

	"radiant/slurm-gateway/internal/gateway"
	"radiant/slurm-gateway/internal/simopsdocker"
)

func TestDockerReactorTelemetryWorkerSetPublishesMeasuredStateAndCleansUp(t *testing.T) {
	image := os.Getenv("REACTOR_TELEMETRY_TEST_IMAGE")
	if image == "" {
		t.Skip("set REACTOR_TELEMETRY_TEST_IMAGE to run the Docker Reactor Telemetry integration test")
	}
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	runtime, err := simopsdocker.NewReactorTelemetryRuntime(image, "bridge")
	if err != nil {
		t.Fatalf("create Docker runtime: %v", err)
	}
	cfg := gateway.DefaultConfig()
	cfg.RequireClientCert = false
	cfg.Simops.Enabled = false
	cfg.Workbench.Store = "memory"
	cfg.Workbench.EventLog = "memory"
	cfg.ReactorTelemetry = gateway.DefaultReactorTelemetryConfig()
	cfg.ReactorTelemetry.Enabled = true
	cfg.ReactorTelemetry.Runtime = "docker"
	cfg.ReactorTelemetry.ControlStore = "memory"
	cfg.ReactorTelemetry.WorkerImage = image
	cfg.ReactorTelemetry.WorkerNetwork = "bridge"
	cfg.ReactorTelemetry.IngestBaseURL = fmt.Sprintf("http://host.docker.internal:%d", port)
	cfg.ReactorTelemetry.CredentialSigningKey = "docker-integration-only-signing-key"
	app, err := gateway.NewDefaultGatewayWithRuntimes(cfg, nil, runtime)
	if err != nil {
		_ = listener.Close()
		t.Fatalf("create gateway: %v", err)
	}
	server := &http.Server{Handler: app.Handler()}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	register := map[string]string{
		"intent": "registerDynamicReactor", "gameSessionId": "docker-session",
		"reactorId": "docker-reactor", "idempotencyKey": "docker-register",
	}
	status, body := postJSON(t, baseURL+"/api/fleet-board/intents", register)
	if status != http.StatusAccepted {
		t.Fatalf("register status=%d body=%s", status, body)
	}
	var response struct {
		WorkerSet gateway.ReactorTelemetryWorkerSet `json:"workerSet"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = postJSONNoFail(baseURL+"/api/fleet-board/intents", map[string]string{
			"intent": "removeDynamicReactor", "gameSessionId": "docker-session",
			"reactorId": "docker-reactor", "idempotencyKey": "docker-cleanup",
		})
	})
	if len(response.WorkerSet.Workers) != 3 {
		t.Fatalf("expected bounded three-worker set, got %#v", response.WorkerSet)
	}

	deadline := time.Now().Add(30 * time.Second)
	var measured struct {
		Frames []gateway.ScadaTelemetryFrame `json:"frames"`
	}
	for time.Now().Before(deadline) {
		res, err := http.Get(baseURL + "/api/simulator-workbench/measured")
		if err == nil {
			_ = json.NewDecoder(res.Body).Decode(&measured)
			_ = res.Body.Close()
			if len(measured.Frames) == 6 {
				break
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if len(measured.Frames) != 6 {
		t.Fatalf("expected six reactor-scoped measured tags, got %#v", measured.Frames)
	}
	for _, frame := range measured.Frames {
		if frame.ReactorID != "docker-reactor" || frame.ValueBasis != gateway.WorkbenchValueMeasured {
			t.Fatalf("container publication lost reactor identity or Value Basis: %#v", frame)
		}
	}

	status, body = postJSON(t, baseURL+"/api/fleet-board/intents", map[string]string{
		"intent": "removeDynamicReactor", "gameSessionId": "docker-session",
		"reactorId": "docker-reactor", "idempotencyKey": "docker-remove",
	})
	if status != http.StatusOK {
		t.Fatalf("remove status=%d body=%s", status, body)
	}
	remaining, err := runtime.Client.ContainerList(context.Background(), dockerclient.ContainerListOptions{All: true, Filters: make(dockerclient.Filters).
		Add("label", "radiant.worker.role=resident-source").
		Add("label", "radiant.reactor-telemetry.set-id="+response.WorkerSet.SetID)})
	if err != nil {
		t.Fatalf("list removed worker set: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("worker set cleanup left containers behind: %#v", remaining)
	}
}

func postJSON(t *testing.T, url string, payload any) (int, []byte) {
	t.Helper()
	status, body := postJSONNoFail(url, payload)
	return status, body
}

func postJSONNoFail(url string, payload any) (int, []byte) {
	raw, _ := json.Marshal(payload)
	res, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer res.Body.Close()
	var body bytes.Buffer
	_, _ = body.ReadFrom(res.Body)
	return res.StatusCode, body.Bytes()
}
