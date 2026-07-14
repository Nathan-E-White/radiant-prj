package simopsdocker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

func TestEngineClientTranslatesMobyResultEnvelopes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1.55/images/image-ok/json":
			_, _ = fmt.Fprint(w, `{"Id":"image-1"}`)
		case "/v1.55/images/image-fail/json":
			http.Error(w, `{"message":"inspect failed"}`, http.StatusInternalServerError)
		case "/v1.55/containers/json":
			_, _ = fmt.Fprint(w, `[{"Id":"container-list-1","State":"running"}]`)
		case "/v1.55/containers/container-inspect-1/json":
			_, _ = fmt.Fprint(w, `{"Id":"container-inspect-1","State":{"Status":"running"}}`)
		case "/v1.55/containers/create":
			_, _ = fmt.Fprint(w, `{"Id":"container-create-1","Warnings":["warning-1"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHTTPClient(server.Client()),
		dockerclient.WithHost(server.URL),
		dockerclient.WithVersion("1.55"),
	)
	if err != nil {
		t.Fatalf("create Moby client: %v", err)
	}
	engine := engineClient{client: client}

	imageInfo, err := engine.ImageInspect(context.Background(), "image-ok")
	if err != nil || imageInfo.ID != "image-1" {
		t.Fatalf("translate image inspect result: info=%#v err=%v", imageInfo, err)
	}
	if _, err := engine.ImageInspect(context.Background(), "image-fail"); err == nil {
		t.Fatal("expected image inspect error to propagate")
	}

	listed, err := engine.ContainerList(
		context.Background(),
		dockerclient.ContainerListOptions{All: true},
	)
	if err != nil || len(listed) != 1 || listed[0].ID != "container-list-1" {
		t.Fatalf("translate container list result: listed=%#v err=%v", listed, err)
	}

	inspected, err := engine.ContainerInspect(context.Background(), "container-inspect-1")
	if err != nil || inspected.ID != "container-inspect-1" || inspected.State == nil || inspected.State.Status != container.StateRunning {
		t.Fatalf("translate container inspect result: inspected=%#v err=%v", inspected, err)
	}

	created, err := engine.ContainerCreate(
		context.Background(),
		&container.Config{Image: "worker:test"},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		nil,
		"worker-1",
	)
	if err != nil || created.ID != "container-create-1" || len(created.Warnings) != 1 {
		t.Fatalf("translate container create result: created=%#v err=%v", created, err)
	}
}
