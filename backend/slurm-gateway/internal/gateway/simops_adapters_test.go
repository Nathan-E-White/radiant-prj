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
	worker.Runtime = "docker"
	worker.RuntimeID = "container-postgres-roundtrip"
	worker.Endpoint = "http://gateway/internal/simops/ingest"
	worker.Labels = map[string]string{"simops.runtime_adapter": "docker-sdk"}
	worker.UpdatedAt = time.Now().UTC().Add(time.Second)
	command.State = SimopsStopped
	command.Message = "stopped by lifecycle compensation"
	command.UpdatedAt = worker.UpdatedAt
	if err := store.SaveLaunch(runID, []SimopsWorkerRecord{worker}, []SimopsSpoolCommand{command}); err != nil {
		t.Fatalf("upsert postgres launch: %v", err)
	}
	workers, err := store.ListWorkers(runID)
	if err != nil || len(workers) != 1 || workers[0].Runtime != "docker" || workers[0].RuntimeID != "container-postgres-roundtrip" {
		t.Fatalf("postgres launch identity parity failed: workers=%#v err=%v", workers, err)
	}
	commands, err := store.ListCommands(runID)
	if err != nil || len(commands) != 1 || commands[0].State != SimopsStopped {
		t.Fatalf("postgres command upsert parity failed: commands=%#v err=%v", commands, err)
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

func TestPostgresSimopsStopLedgerRetainsFailureAndCleanupFacts(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("set SIMOPS_POSTGRES_TEST_DSN to run Postgres SimOps stop-ledger integration test")
	}
	store, err := NewPostgresSimopsStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	runID := "RUN-PG-STOP-" + strings.ReplaceAll(now.Format("150405.000000"), ".", "")
	run := SimopsRunRecord{RunID: runID, ScenarioID: "scheduler-drift", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "resident", RuntimeLimitSec: 1, SubmittedBy: "postgres-test", IngestToken: "token", State: SimopsRunState{Phase: SimopsRunRunning, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, Cleanup: SimopsCleanupNotRequested}, CreatedAt: now, UpdatedAt: now}
	worker := SimopsWorkerRecord{RunID: runID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, Lifecycle: SimopsStreaming, LaunchMode: "resident", State: SimopsWorkerState{Phase: SimopsWorkerExecuting, LaunchDisposition: SimopsLaunchLaunched, ActiveExecution: SimopsExecutionActive, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, FailureStage: SimopsFailureNone, Cleanup: SimopsCleanupNotRequested}, UpdatedAt: now}
	if _, created, err := store.CreateRun(run, []SimopsWorkerRecord{worker}, nil); err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	for _, cleanup := range []SimopsCleanupDisposition{SimopsCleanupRequested, SimopsCleanupInProgress} {
		outcome, err := store.ApplyStopTransition(SimopsStopTransition{RunID: runID, Cleanup: cleanup, OccurredAt: now})
		if err != nil || outcome.Run.State.Phase != SimopsRunStopping {
			t.Fatalf("cleanup=%s outcome=%#v err=%v", cleanup, outcome, err)
		}
	}
	outcome, err := store.ApplyStopTransition(SimopsStopTransition{RunID: runID, Cleanup: SimopsCleanupFailed, CleanupFailure: "delete rejected", CleanupConfirmed: true, OccurredAt: now})
	if err != nil || outcome.Run.State.Phase != SimopsRunFailed || outcome.Intent != nil {
		t.Fatalf("cleanup failure outcome=%#v err=%v", outcome, err)
	}
	if len(outcome.Workers) != 1 || outcome.Workers[0].State.FailureStage != SimopsFailureCleanup {
		t.Fatalf("worker failure stage=%#v", outcome.Workers)
	}
	failedRun := run
	failedRun.RunID = runID + "-TERMINAL"
	failedRun.IdempotencyKey = ""
	failedWorker := worker
	failedWorker.RunID = failedRun.RunID
	if _, created, err := store.CreateRun(failedRun, []SimopsWorkerRecord{failedWorker}, nil); err != nil || !created {
		t.Fatalf("create failed-run fixture: created=%v err=%v", created, err)
	}
	if err := store.UpdateWorkerObservedLifecycle(ObservedWorkerLifecycle{RunID: failedRun.RunID, WorkerID: failedWorker.WorkerID, State: ObservedWorkerFailed, Runtime: "contract", ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	outcome, err = store.ApplyStopTransition(SimopsStopTransition{RunID: failedRun.RunID, Cleanup: SimopsCleanupCompleted, OccurredAt: now})
	if err != nil || outcome.Run.State.Phase != SimopsRunFailed || outcome.Intent != nil {
		t.Fatalf("pre-existing failure cannot publish stopped: outcome=%#v err=%v", outcome, err)
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
