package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestProjectTelemetryEventMapsFrameAndRedpandaOffset(t *testing.T) {
	frame := testTelemetryFrame(t, "RUN-DATA-PLANE", "scheduler-01", 7)
	raw, _ := json.Marshal(frame)
	event := SimopsEvent{
		RunID:      frame.RunID,
		WorkerID:   frame.WorkerID,
		EventType:  SimopsEventWorkerTelemetry,
		Frame:      raw,
		OccurredAt: time.Date(2026, 7, 5, 12, 0, 1, 0, time.UTC),
	}

	projection, ok, err := ProjectTelemetryEvent("simops.telemetry.v1", 2, 42, event)
	if err != nil {
		t.Fatalf("project event: %v", err)
	}
	if !ok {
		t.Fatalf("expected worker telemetry projection")
	}
	if projection.RunID != frame.RunID || projection.WorkerID != frame.WorkerID {
		t.Fatalf("unexpected projection identity %#v", projection)
	}
	if projection.Sequence != 7 || projection.PayloadType != "scheduler.sample" {
		t.Fatalf("unexpected telemetry shape %#v", projection)
	}
	if projection.Quality != "nominal" || !projection.SourceLagMs.Valid || projection.SourceLagMs.Float64 != 3.5 {
		t.Fatalf("unexpected stream quality %#v", projection)
	}
	if projection.RedpandaTopic != "simops.telemetry.v1" || projection.RedpandaPartition != 2 || projection.RedpandaOffset != 42 {
		t.Fatalf("redpanda coordinates missing from projection %#v", projection)
	}
}

func TestRunTimescaleTelemetryConsumerWritesProjectionAndCommits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	frame := testTelemetryFrame(t, "RUN-CONSUME", "storage-01", 1)
	raw, _ := json.Marshal(frame)
	event := SimopsEvent{RunID: frame.RunID, WorkerID: frame.WorkerID, EventType: SimopsEventWorkerTelemetry, Frame: raw, OccurredAt: time.Now().UTC()}
	payload, _ := json.Marshal(event)
	reader := &fakeSimopsKafkaReader{
		messages: []SimopsBrokerMessage{{
			Topic:     "simops.telemetry.v1",
			Partition: 0,
			Offset:    9,
			Value:     payload,
		}},
		afterCommit: cancel,
	}
	store := &capturingProjectionStore{}
	cfg := DefaultConfig().Simops
	cfg.TimescaleConsumerGroup = "test-timescale"
	metrics := NewSimopsConsumerMetrics()

	if err := RunTimescaleTelemetryConsumer(ctx, cfg, reader, store, metrics); err != nil {
		t.Fatalf("run consumer: %v", err)
	}
	if len(store.projections) != 1 {
		t.Fatalf("expected one projection, got %d", len(store.projections))
	}
	if store.consumerNames[0] != "test-timescale" {
		t.Fatalf("unexpected consumer name %q", store.consumerNames[0])
	}
	if len(reader.committed) != 1 || reader.committed[0].Offset != 9 {
		t.Fatalf("expected offset commit, got %#v", reader.committed)
	}
	snapshot := metrics.Snapshot()
	if snapshot.FramesWritten != 1 || snapshot.LastConsumedOffset != 9 {
		t.Fatalf("unexpected metrics %#v", snapshot)
	}
}

func TestMoQTrackRouterRoutesLifecycleTelemetryQualityAndArtifacts(t *testing.T) {
	router := NewSimopsMoQTrackRouter()
	frame := testTelemetryFrame(t, "RUN-TRACKS", "fabric-01", 3)
	raw, _ := json.Marshal(frame)

	if _, err := router.ApplyEvent(SimopsEvent{RunID: "RUN-TRACKS", EventType: SimopsEventRunLifecycle, Lifecycle: SimopsStreaming, OccurredAt: time.Now().UTC()}, 1); err != nil {
		t.Fatalf("route lifecycle: %v", err)
	}
	if _, err := router.ApplyEvent(SimopsEvent{RunID: "RUN-TRACKS", WorkerID: "fabric-01", EventType: SimopsEventWorkerTelemetry, Frame: raw, OccurredAt: time.Now().UTC()}, 2); err != nil {
		t.Fatalf("route telemetry: %v", err)
	}
	if _, err := router.ApplyEvent(SimopsEvent{RunID: "RUN-TRACKS", EventType: SimopsArtifactIntentEventType, Frame: json.RawMessage(`{"artifact_id":"iceberg-telemetry-RUN-TRACKS"}`), OccurredAt: time.Now().UTC()}, 3); err != nil {
		t.Fatalf("route artifact: %v", err)
	}

	got := router.Snapshot()
	names := make(map[string]struct{}, len(got))
	for _, message := range got {
		names[message.Track] = struct{}{}
	}
	for _, want := range []string{"lifecycle", "workers/fabric-01/telemetry", "workers/fabric-01/quality", "artifacts"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing MoQ track %q from %#v", want, got)
		}
	}
}

func TestMoQTrackHubPublishesToActualSubscribers(t *testing.T) {
	hub := NewSimopsMoQTrackHub()
	ch, cancel, id := hub.Subscribe(1)
	if id == 0 {
		t.Fatalf("expected subscriber id")
	}
	if got := hub.SubscriberCount(); got != 1 {
		t.Fatalf("expected one subscriber, got %d", got)
	}
	message := SimopsMoQTrackMessage{
		Track:      "workers/scheduler-01/telemetry",
		RunID:      "RUN-HUB",
		WorkerID:   "scheduler-01",
		EventType:  SimopsEventWorkerTelemetry,
		Payload:    json.RawMessage(`{"ok":true}`),
		OccurredAt: time.Now().UTC(),
		Offset:     44,
	}
	hub.PublishMoQTracks([]SimopsMoQTrackMessage{message})
	select {
	case got := <-ch:
		if got.Track != message.Track || got.Offset != message.Offset {
			t.Fatalf("unexpected message %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for hub message")
	}
	wire := NewSimopsMoQWireMessage(message)
	if wire.Namespace != "radiant/simops/RUN-HUB" || wire.Protocol != "moq-webtransport" {
		t.Fatalf("unexpected wire envelope %#v", wire)
	}
	cancel()
	if got := hub.SubscriberCount(); got != 0 {
		t.Fatalf("expected subscriber removal, got %d", got)
	}
}

func testTelemetryFrame(t *testing.T, runID string, workerID string, sequence uint64) SimopsTelemetryFrame {
	t.Helper()
	return SimopsTelemetryFrame{
		SchemaVersion: "simops.telemetry.v1",
		RunID:         runID,
		ScenarioID:    "scheduler-drift",
		WorkerID:      workerID,
		WorkerKind:    SimopsWorkerScheduler,
		Sequence:      sequence,
		EmittedAt:     "2026-07-05T12:00:00.123456Z",
		ReceivedAt:    "2026-07-05T12:00:00.223456Z",
		PayloadType:   "scheduler.sample",
		StreamQuality: json.RawMessage(`{"quality":"nominal","sourceLagMs":3.5,"collectorLagMs":4.5,"droppedFrameCount":2}`),
		Payload:       json.RawMessage(`{"queue_depth":12}`),
	}
}

type capturingProjectionStore struct {
	consumerNames []string
	projections   []SimopsTelemetryProjection
	err           error
}

func (s *capturingProjectionStore) SaveProjection(_ context.Context, consumerName string, projection SimopsTelemetryProjection) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	s.consumerNames = append(s.consumerNames, consumerName)
	s.projections = append(s.projections, projection)
	return true, nil
}

type fakeSimopsKafkaReader struct {
	messages    []SimopsBrokerMessage
	committed   []SimopsBrokerMessage
	afterCommit func()
	closed      bool
}

func (r *fakeSimopsKafkaReader) FetchMessage(ctx context.Context) (SimopsBrokerMessage, error) {
	if len(r.messages) == 0 {
		<-ctx.Done()
		return SimopsBrokerMessage{}, ctx.Err()
	}
	message := r.messages[0]
	r.messages = r.messages[1:]
	return message, nil
}

func (r *fakeSimopsKafkaReader) CommitMessages(_ context.Context, msgs ...SimopsBrokerMessage) error {
	if len(msgs) == 0 {
		return errors.New("commit requires message")
	}
	r.committed = append(r.committed, msgs...)
	if r.afterCommit != nil {
		r.afterCommit()
	}
	return nil
}

func (r *fakeSimopsKafkaReader) Close() error {
	r.closed = true
	return nil
}
