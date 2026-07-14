package simopsdocker

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"

	"radiant/slurm-gateway/internal/gateway"
)

func TestReactorTelemetryRuntimeLaunchesGatewayOnlyWorkers(t *testing.T) {
	client := &fakeDockerClient{
		image:          image.InspectResponse{ID: "image-telemetry"},
		createSequence: []container.CreateResponse{{ID: "container-1"}, {ID: "container-2"}},
	}
	runtime := ReactorTelemetryRuntime{
		Client:  client,
		Image:   "radiant-scada-standins:test",
		Network: "radiant-simops-local",
	}
	launch := gateway.ReactorTelemetryLaunch{SetID: "set-stable", Workers: []gateway.ReactorTelemetryWorker{
		telemetryWorker("worker-1", "source-1", 0, "token-1"),
		telemetryWorker("worker-2", "source-2", 1, "token-2"),
	}}
	if err := runtime.StartWorkerSet(context.Background(), launch); err != nil {
		t.Fatalf("start worker set: %v", err)
	}
	if client.createCalls != 2 {
		t.Fatalf("expected two bounded workers, got %d", client.createCalls)
	}
	if client.createdConfig.Image != "radiant-scada-standins:test" {
		t.Fatalf("unexpected image: %#v", client.createdConfig)
	}
	command := strings.Join(client.createdConfig.Cmd, " ")
	for _, expected := range []string{"--source-id source-2", "--reactor-id reactor-a", "--worker-index 1", "--ingest-base-url http://gateway:8080", "--ingest-token token-2", "--max-frames 86400"} {
		if !strings.Contains(command, expected) {
			t.Fatalf("worker command missing %q: %s", expected, command)
		}
	}
	if len(client.createdConfig.Env) != 0 {
		t.Fatalf("worker environment received unexpected credentials: %#v", client.createdConfig.Env)
	}
	labels := client.createdConfig.Labels
	if labels["radiant.worker.role"] != "resident-source" || labels["radiant.reactor-telemetry.set-id"] != "set-stable" || labels["radiant.reactor-id"] != "reactor-a" {
		t.Fatalf("runtime labels lost worker-set identity: %#v", labels)
	}
	if string(client.createdHostConfig.NetworkMode) != "radiant-simops-local" || client.createdHostConfig.Privileged {
		t.Fatalf("unexpected worker host authority: %#v", client.createdHostConfig)
	}
	if !slices.Contains(client.createdHostConfig.ExtraHosts, "host.docker.internal:host-gateway") {
		t.Fatalf("worker cannot reach the gateway host on Linux Docker: %#v", client.createdHostConfig.ExtraHosts)
	}
	if client.createdHostConfig.RestartPolicy.Name != container.RestartPolicyOnFailure || client.createdHostConfig.RestartPolicy.MaximumRetryCount != 10 {
		t.Fatalf("startup recovery is not bounded: %#v", client.createdHostConfig.RestartPolicy)
	}
}

func TestReactorTelemetryRuntimeStopsOnlyTargetWorkerSet(t *testing.T) {
	client := &fakeDockerClient{listed: []container.Summary{{ID: "container-1"}, {ID: "container-2"}}}
	runtime := ReactorTelemetryRuntime{Client: client, Image: "radiant-scada-standins:test"}
	if err := runtime.StopWorkerSet(context.Background(), "set-stable"); err != nil {
		t.Fatalf("stop worker set: %v", err)
	}
	if len(client.stopped) != 2 || len(client.removed) != 2 {
		t.Fatalf("expected targeted stop and removal, stopped=%#v removed=%#v", client.stopped, client.removed)
	}
	want := filters.NewArgs(
		filters.Arg("label", "radiant.worker.role=resident-source"),
		filters.Arg("label", "radiant.reactor-telemetry.set-id=set-stable"),
	)
	gotLabels := client.listOptions.Filters.Get("label")
	wantLabels := want.Get("label")
	slices.Sort(gotLabels)
	slices.Sort(wantLabels)
	if !slices.Equal(gotLabels, wantLabels) {
		t.Fatalf("cleanup filter could affect another set: got %#v want %#v", client.listOptions.Filters.Get("label"), want.Get("label"))
	}
}

func telemetryWorker(workerID, sourceID string, index int, token string) gateway.ReactorTelemetryWorker {
	return gateway.ReactorTelemetryWorker{
		WorkerID: workerID, SourceID: sourceID, WorkerIndex: index,
		GameSessionID: "session-a", ReactorID: "reactor-a",
		MaxFrames: 86400,
		Gateway:   gateway.ReactorTelemetryGatewayProfile{IngestBaseURL: "http://gateway:8080", IngestToken: token},
	}
}
