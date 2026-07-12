package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRedpandaEventLogPublishesAndPersistsEvent(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{
		RunID:       "RUN-REDPANDA-001",
		ScenarioID:  "scheduler-drift",
		Lifecycle:   SimopsStreaming,
		Source:      "frontend",
		WorkScript:  "scheduler-drift",
		LaunchMode:  "auto",
		SubmittedBy: "test",
		IngestToken: "token",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	writer := &capturingKafkaWriter{}
	log := &RedpandaEventLog{Topic: "simops.telemetry.v1", Store: store, Writer: writer}

	event := SimopsEvent{RunID: run.RunID, WorkerID: "scheduler-01", EventType: "worker.telemetry", OccurredAt: time.Now().UTC()}
	if err := log.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(writer.messages) != 1 {
		t.Fatalf("expected one kafka message, got %d", len(writer.messages))
	}
	if string(writer.messages[0].Key) != "RUN-REDPANDA-001|scheduler-01" {
		t.Fatalf("unexpected kafka key %q", string(writer.messages[0].Key))
	}
	var payload SimopsEvent
	if err := json.Unmarshal(writer.messages[0].Value, &payload); err != nil {
		t.Fatalf("decode kafka payload: %v", err)
	}
	if payload.EventType != "worker.telemetry" {
		t.Fatalf("unexpected payload %#v", payload)
	}
	events, err := store.ListEvents(run.RunID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected persisted event, got %d", len(events))
	}
}

func TestRedpandaEventLogReturnsWriterFailure(t *testing.T) {
	log := &RedpandaEventLog{Topic: "simops.telemetry.v1", Writer: &capturingKafkaWriter{err: errors.New("broker down")}}

	if err := log.Publish(context.Background(), SimopsEvent{RunID: "RUN-REDPANDA-FAIL", EventType: "run.lifecycle"}); err == nil {
		t.Fatalf("expected writer failure")
	}
}

func TestPostgresSimopsStoreRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("set SIMOPS_POSTGRES_TEST_DSN to run Postgres SimOps store integration test")
	}
	store, err := NewPostgresSimopsStore(dsn)
	if err != nil {
		t.Fatalf("new postgres store: %v", err)
	}
	runID := "RUN-PG-ROUNDTRIP-" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", "")
	run := SimopsRunRecord{
		RunID:           runID,
		ScenarioID:      "scheduler-drift",
		Lifecycle:       SimopsStarting,
		Source:          "frontend",
		WorkScript:      "scheduler-drift",
		LaunchMode:      "auto",
		RuntimeLimitSec: 120,
		IdempotencyKey:  "pg-roundtrip",
		SubmittedBy:     "postgres-test",
		IngestToken:     "token",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	worker := SimopsWorkerRecord{RunID: runID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, Lifecycle: SimopsStarting, LaunchMode: "auto", UpdatedAt: time.Now().UTC()}
	command := SimopsSpoolCommand{CommandID: runID + "-scheduler-start", RunID: runID, WorkerID: "scheduler-01", Mode: "auto", State: SimopsStarting, Message: "started", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, created, err := store.CreateRun(run, []SimopsWorkerRecord{worker}, []SimopsSpoolCommand{command}); err != nil || !created {
		t.Fatalf("create postgres run created=%v err=%v", created, err)
	}
	if err := store.SaveEvent(SimopsEvent{RunID: runID, EventType: "run.lifecycle", Lifecycle: SimopsStreaming, OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save event: %v", err)
	}
	events, err := store.ListEvents(runID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
}

type capturingKafkaWriter struct {
	messages []SimopsBrokerMessage
	err      error
}

func (w *capturingKafkaWriter) WriteMessages(_ context.Context, msgs ...SimopsBrokerMessage) error {
	if w.err != nil {
		return w.err
	}
	w.messages = append(w.messages, msgs...)
	return nil
}
