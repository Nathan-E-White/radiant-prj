package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSimopsRunLifecyclePersistsPartialLaunchAndCompensationOutcome(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	store := NewInMemorySimopsStore()
	spooler := &partialFailureSimopsSpooler{now: now}
	lifecycle := NewSimopsRunLifecyclePolicy(
		testRunConnectionProfileConfig(),
		store,
		spooler,
		MemorySimopsEventLog{Store: store},
		IcebergArtifactPlanner{},
	)
	lifecycle.SetNow(func() time.Time { return now })

	run := SimopsRunRecord{
		RunID:       "RUN-PARTIAL-LAUNCH",
		ScenarioID:  "scheduler-drift",
		Lifecycle:   SimopsStarting,
		LaunchMode:  "auto",
		SubmittedBy: "react-backend-client",
		IngestToken: "test-token",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{
		SimopsWorkerScheduler,
		SimopsWorkerStorage,
	})
	if err == nil {
		t.Fatal("expected partial launch failure")
	}
	var lifecycleErr *SimopsRunLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.Stage != SimopsRunStageRuntimeLaunch {
		t.Fatalf("expected runtime-launch lifecycle error, got %T %v", err, err)
	}
	if outcome.Run.Lifecycle != SimopsFailed {
		t.Fatalf("expected durable failed run outcome, got %#v", outcome.Run)
	}
	if spooler.stops != 1 {
		t.Fatalf("expected one compensation stop, got %d", spooler.stops)
	}

	workers, listErr := store.ListWorkers(run.RunID)
	if listErr != nil {
		t.Fatalf("list workers: %v", listErr)
	}
	workersByID := make(map[string]SimopsWorkerRecord, len(workers))
	for _, worker := range workers {
		workersByID[worker.WorkerID] = worker
	}
	if got := workersByID["scheduler-01"]; got.Lifecycle != SimopsStopped || got.RuntimeID != "runtime-scheduler" {
		t.Fatalf("expected launched worker to retain runtime identity and stopped compensation outcome, got %#v", got)
	}
	if got := workersByID["storage-01"]; got.Lifecycle != SimopsFailed || got.RuntimeID != "" {
		t.Fatalf("expected unlaunched worker to have explicit failed outcome, got %#v", got)
	}

	events, listErr := store.ListEvents(run.RunID)
	if listErr != nil {
		t.Fatalf("list events: %v", listErr)
	}
	if len(events) != 1 || events[0].EventType != "run.lifecycle.failure" || events[0].Lifecycle != SimopsFailed {
		t.Fatalf("expected one durable lifecycle failure event, got %#v", events)
	}
	if string(events[0].Frame) == "" {
		t.Fatalf("expected failure event details, got %#v", events[0])
	}
}

func TestSimopsRunLifecycleReturnsTypedInitialPersistenceFailureWithoutLaunching(t *testing.T) {
	base := NewInMemorySimopsStore()
	store := &failingLifecycleStore{SimopsStore: base, failCreate: true}
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})

	_, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStagePersistence)
	if spooler.starts != 0 || spooler.stops != 0 {
		t.Fatalf("initial persistence failure must not touch runtime, got starts=%d stops=%d", spooler.starts, spooler.stops)
	}
}

func TestSimopsRunLifecycleRejectsSilentPartialLaunch(t *testing.T) {
	store := NewInMemorySimopsStore()
	spooler := &silentPartialLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler, SimopsWorkerStorage})
	assertLifecycleErrorStage(t, err, SimopsRunStageRuntimeLaunch)
	if spooler.stops != 1 {
		t.Fatalf("expected silent partial launch to be compensated, got %d stops", spooler.stops)
	}
	workers, listErr := store.ListWorkers(run.RunID)
	if listErr != nil || len(workers) != 2 {
		t.Fatalf("expected both planned worker outcomes, workers=%#v err=%v", workers, listErr)
	}
	for _, worker := range workers {
		if worker.WorkerID == "scheduler-01" && worker.Lifecycle != SimopsStopped {
			t.Fatalf("expected returned worker stopped, got %#v", worker)
		}
		if worker.WorkerID == "storage-01" && worker.Lifecycle != SimopsFailed {
			t.Fatalf("expected missing worker failed, got %#v", worker)
		}
	}
	if outcome.Run.Lifecycle != SimopsFailed {
		t.Fatalf("expected failed Run, got %#v", outcome.Run)
	}
}

func TestSimopsRunLifecycleRecoversStrandedStartingRunOnIdempotentRetry(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 30, 0, 0, time.UTC)
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{
		RunID: "RUN-STRANDED", ScenarioID: "scheduler-drift", Lifecycle: SimopsStarting,
		LaunchMode: "auto", IdempotencyKey: "retry-key", SubmittedBy: "react-backend-client",
		IngestToken: "test-token", CreatedAt: now.Add(-2 * incompleteStartStaleAfter), UpdatedAt: now.Add(-incompleteStartStaleAfter),
	}
	planned := plannedWorkerRecords(run, []SimopsWorkerKind{SimopsWorkerScheduler, SimopsWorkerStorage}, now)
	if _, created, err := store.CreateRun(run, planned, nil); err != nil || !created {
		t.Fatalf("seed stranded Run created=%v err=%v", created, err)
	}
	spooler := &recoveringLifecycleSpooler{observations: []ObservedWorkerLifecycle{
		{RunID: run.RunID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, State: ObservedWorkerActive, Runtime: "docker", RuntimeID: "container-scheduler", ObservedAt: now},
		{RunID: run.RunID, WorkerID: "storage-01", WorkerKind: SimopsWorkerStorage, State: ObservedWorkerMissing, Runtime: "docker", ObservedAt: now},
	}}
	lifecycle := NewSimopsRunLifecyclePolicy(testRunConnectionProfileConfig(), store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})
	lifecycle.SetNow(func() time.Time { return now })

	retryRecord := run
	retryRecord.RunID = "RUN-RETRY-SHOULD-NOT-REPLACE"
	outcome, err := lifecycle.Start(context.Background(), retryRecord, []SimopsWorkerKind{SimopsWorkerFabric})
	assertLifecycleErrorStage(t, err, SimopsRunStageIncompleteRecovery)
	if outcome.Run.RunID != run.RunID || outcome.Run.Lifecycle != SimopsFailed || spooler.stops != 1 {
		t.Fatalf("unexpected recovered outcome=%#v stops=%d", outcome, spooler.stops)
	}
	workers, _ := store.ListWorkers(run.RunID)
	byID := map[string]SimopsWorkerRecord{}
	for _, worker := range workers {
		byID[worker.WorkerID] = worker
	}
	if byID["scheduler-01"].Lifecycle != SimopsStopped || byID["scheduler-01"].RuntimeID != "container-scheduler" {
		t.Fatalf("expected observed runtime worker stopped with identity, got %#v", byID["scheduler-01"])
	}
	if byID["storage-01"].Lifecycle != SimopsFailed {
		t.Fatalf("expected missing worker failed, got %#v", byID["storage-01"])
	}

	replayed, err := lifecycle.Start(context.Background(), retryRecord, []SimopsWorkerKind{SimopsWorkerFabric})
	if err != nil || replayed.Run.Lifecycle != SimopsFailed || spooler.stops != 1 {
		t.Fatalf("terminal retry must replay without another compensation: outcome=%#v stops=%d err=%v", replayed, spooler.stops, err)
	}
}

func TestSimopsRunLifecycleReplaysFreshStartingRunWithoutRecovery(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 32, 0, 0, time.UTC)
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{
		RunID: "RUN-IN-FLIGHT", ScenarioID: "scheduler-drift", Lifecycle: SimopsStarting,
		LaunchMode: "auto", IdempotencyKey: "in-flight-key", SubmittedBy: "react-backend-client",
		IngestToken: "test-token", CreatedAt: now, UpdatedAt: now,
	}
	planned := plannedWorkerRecords(run, []SimopsWorkerKind{SimopsWorkerScheduler}, now)
	if _, created, err := store.CreateRun(run, planned, nil); err != nil || !created {
		t.Fatalf("seed in-flight Run created=%v err=%v", created, err)
	}
	spooler := &recoveringLifecycleSpooler{}
	lifecycle := NewSimopsRunLifecyclePolicy(testRunConnectionProfileConfig(), store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})
	lifecycle.SetNow(func() time.Time { return now })

	retry := run
	retry.RunID = "RUN-DUPLICATE-SHOULD-NOT-BE-CREATED"
	outcome, err := lifecycle.Start(context.Background(), retry, []SimopsWorkerKind{SimopsWorkerStorage})
	if err != nil || outcome.Created || outcome.Run.RunID != run.RunID || outcome.Run.Lifecycle != SimopsStarting {
		t.Fatalf("fresh duplicate must replay in-flight Run: outcome=%#v err=%v", outcome, err)
	}
	if spooler.syncs != 0 || spooler.stops != 0 {
		t.Fatalf("fresh duplicate must not observe or compensate runtime: syncs=%d stops=%d", spooler.syncs, spooler.stops)
	}
}

func TestSimopsControllerIdempotentRetryRecoversStartingRunAtCapacity(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 35, 0, 0, time.UTC)
	cfg := testRunConnectionProfileConfig()
	cfg.MaxActiveRuns = 1
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{
		RunID: "RUN-CONTROLLER-STRANDED", ScenarioID: "scheduler-drift", Lifecycle: SimopsStarting,
		Source: "frontend", WorkScript: "scheduler-drift", LaunchMode: "auto", RuntimeLimitSec: 120,
		IdempotencyKey: "controller-retry", SubmittedBy: "react-backend-client", IngestToken: "test-token",
		CreatedAt: now.Add(-2 * incompleteStartStaleAfter), UpdatedAt: now.Add(-incompleteStartStaleAfter),
	}
	planned := plannedWorkerRecords(run, []SimopsWorkerKind{SimopsWorkerScheduler}, now)
	if _, created, err := store.CreateRun(run, planned, nil); err != nil || !created {
		t.Fatalf("seed controller stranded Run created=%v err=%v", created, err)
	}
	spooler := &recoveringLifecycleSpooler{observations: []ObservedWorkerLifecycle{{
		RunID: run.RunID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, State: ObservedWorkerMissing, Runtime: "docker", ObservedAt: now,
	}}}
	controller := NewSimopsController(cfg, store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{}, nil, nil)
	controller.now = func() time.Time { return now }
	controller.runID = func() string { return "RUN-NEW-SHOULD-NOT-BE-CREATED" }

	_, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID: "scheduler-drift", WorkerKinds: []string{"scheduler"}, LaunchMode: "auto", IdempotencyKey: "controller-retry",
	}, "react-backend-client")
	assertLifecycleErrorStage(t, err, SimopsRunStageIncompleteRecovery)
	if status != 500 || spooler.stops != 1 {
		t.Fatalf("expected bounded recovery instead of capacity rejection: status=%d stops=%d err=%v", status, spooler.stops, err)
	}
	stored, getErr := store.GetRun(run.RunID)
	if getErr != nil || stored.Lifecycle != SimopsFailed {
		t.Fatalf("expected stranded Run terminated: run=%#v err=%v", stored, getErr)
	}
	if _, getErr := store.GetRun("RUN-NEW-SHOULD-NOT-BE-CREATED"); !errors.Is(getErr, ErrSimopsRunNotFound) {
		t.Fatalf("retry must not create a replacement Run, err=%v", getErr)
	}
}

func TestSimopsRunLifecycleBoundsDetachedRecovery(t *testing.T) {
	store := NewInMemorySimopsStore()
	spooler := &blockingRecoveryLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	cfg := testRunConnectionProfileConfig()
	cfg.LifecycleRecoveryTimeout = 10 * time.Millisecond
	lifecycle := NewSimopsRunLifecyclePolicy(cfg, store, spooler, MemorySimopsEventLog{Store: store}, invalidLifecycleArtifactPlanner{})
	now := time.Date(2026, 7, 14, 3, 45, 0, 0, time.UTC)
	lifecycle.SetNow(func() time.Time { return now })
	run := SimopsRunRecord{RunID: "RUN-BOUNDED-RECOVERY", ScenarioID: "scheduler-drift", Lifecycle: SimopsStarting, LaunchMode: "auto", SubmittedBy: "test", CreatedAt: now, UpdatedAt: now}

	started := time.Now()
	_, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	if time.Since(started) > time.Second {
		t.Fatalf("bounded recovery exceeded one second: %v", time.Since(started))
	}
	var lifecycleErr *SimopsRunLifecycleError
	if !errors.As(err, &lifecycleErr) || !errors.Is(lifecycleErr.CompensationError, context.DeadlineExceeded) || !errors.Is(lifecycleErr.RecoveryError, context.DeadlineExceeded) {
		t.Fatalf("expected typed deadline evidence, got %T %v", err, err)
	}
}

func TestSimopsRunLifecycleHTTPStatusSeparatesAdapterAndControlFailures(t *testing.T) {
	tests := []struct {
		stage SimopsRunLifecycleStage
		want  int
	}{
		{stage: SimopsRunStageRuntimeLaunch, want: 502},
		{stage: SimopsRunStageEventPublication, want: 502},
		{stage: SimopsRunStagePersistence, want: 500},
		{stage: SimopsRunStageArtifactPersistence, want: 500},
	}
	for _, test := range tests {
		err := &SimopsRunLifecycleError{Stage: test.stage, Cause: errors.New("forced failure")}
		if got := simopsLifecycleHTTPStatus(err); got != test.want {
			t.Fatalf("stage %s: expected HTTP %d, got %d", test.stage, test.want, got)
		}
	}
}

func TestSimopsRunLifecycleCompensatesLaunchPersistenceFailure(t *testing.T) {
	base := NewInMemorySimopsStore()
	store := &failingLifecycleStore{SimopsStore: base, failSaveLaunch: 1}
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageLaunchPersistence)
	assertFailedLifecycleOutcome(t, base, outcome, SimopsStopped, SimopsRunStageLaunchPersistence)
	if spooler.stops != 1 {
		t.Fatalf("expected compensation after launch persistence failure, got %d stops", spooler.stops)
	}
}

func TestSimopsRunLifecycleCompensatesStreamingTransitionFailure(t *testing.T) {
	base := NewInMemorySimopsStore()
	store := &failingLifecycleStore{SimopsStore: base, failStreamingTransition: true}
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageStreamingTransition)
	assertFailedLifecycleOutcome(t, base, outcome, SimopsStopped, SimopsRunStageStreamingTransition)
}

func TestSimopsRunLifecycleRejectsInvalidArtifactPlan(t *testing.T) {
	store := NewInMemorySimopsStore()
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, invalidLifecycleArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageArtifactPlanning)
	assertFailedLifecycleOutcome(t, store, outcome, SimopsStopped, SimopsRunStageArtifactPlanning)
	artifacts, listErr := store.ListArtifacts(run.RunID)
	if listErr != nil || len(artifacts) != 0 {
		t.Fatalf("invalid plan must not persist an artifact, artifacts=%#v err=%v", artifacts, listErr)
	}
}

func TestSimopsRunLifecycleCompensatesArtifactPersistenceFailure(t *testing.T) {
	base := NewInMemorySimopsStore()
	store := &failingLifecycleStore{SimopsStore: base, failArtifact: true}
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, IcebergArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageArtifactPersistence)
	assertFailedLifecycleOutcome(t, base, outcome, SimopsStopped, SimopsRunStageArtifactPersistence)
}

func TestSimopsRunLifecyclePersistsEventPublicationFailureAndFailsArtifact(t *testing.T) {
	store := NewInMemorySimopsStore()
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, failingLifecycleEventLog{}, IcebergArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageEventPublication)
	assertFailedLifecycleOutcome(t, store, outcome, SimopsStopped, SimopsRunStageEventPublication)
	artifacts, listErr := store.ListArtifacts(run.RunID)
	if listErr != nil || len(artifacts) != 1 || artifacts[0].Status != SimopsArtifactStatusFailed {
		t.Fatalf("expected planned artifact to receive explicit failed disposition, artifacts=%#v err=%v", artifacts, listErr)
	}
}

func TestSimopsRunLifecycleRecordsFailedCompensation(t *testing.T) {
	store := NewInMemorySimopsStore()
	spooler := &trackingLifecycleSpooler{delegate: ContractSimopsSpooler{Mode: "auto"}, failStop: true}
	lifecycle, run := newLifecycleTestPolicy(store, spooler, MemorySimopsEventLog{Store: store}, invalidLifecycleArtifactPlanner{})

	outcome, err := lifecycle.Start(context.Background(), run, []SimopsWorkerKind{SimopsWorkerScheduler})
	assertLifecycleErrorStage(t, err, SimopsRunStageArtifactPlanning)
	var lifecycleErr *SimopsRunLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.CompensationError == nil {
		t.Fatalf("expected compensation failure in lifecycle error, got %v", err)
	}
	assertFailedLifecycleOutcome(t, store, outcome, SimopsFailed, SimopsRunStageArtifactPlanning)
	events, _ := store.ListEvents(run.RunID)
	if len(events) != 1 || !stringsContainAll(string(events[0].Frame), `"compensation":"failed"`, "stop failed") {
		t.Fatalf("expected durable failed compensation detail, got %#v", events)
	}
}

func newLifecycleTestPolicy(store SimopsStore, spooler SimopsSpooler, eventLog SimopsEventLog, artifact SimopsArtifactSink) (*SimopsRunLifecyclePolicy, SimopsRunRecord) {
	now := time.Date(2026, 7, 14, 3, 15, 0, 0, time.UTC)
	lifecycle := NewSimopsRunLifecyclePolicy(testRunConnectionProfileConfig(), store, spooler, eventLog, artifact)
	lifecycle.SetNow(func() time.Time { return now })
	return lifecycle, SimopsRunRecord{
		RunID: "RUN-LIFECYCLE-FAILURE", ScenarioID: "scheduler-drift", Lifecycle: SimopsStarting,
		LaunchMode: "auto", SubmittedBy: "react-backend-client", IngestToken: "test-token", CreatedAt: now, UpdatedAt: now,
	}
}

func assertLifecycleErrorStage(t *testing.T, err error, want SimopsRunLifecycleStage) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected lifecycle failure at %s", want)
	}
	var lifecycleErr *SimopsRunLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.Stage != want {
		t.Fatalf("expected lifecycle stage %s, got %T %v", want, err, err)
	}
}

func assertFailedLifecycleOutcome(t *testing.T, store SimopsStore, outcome SimopsRunLifecycleOutcome, workerLifecycle SimopsLifecycle, stage SimopsRunLifecycleStage) {
	t.Helper()
	if outcome.Run.Lifecycle != SimopsFailed {
		t.Fatalf("expected failed Run outcome, got %#v", outcome.Run)
	}
	workers, err := store.ListWorkers(outcome.Run.RunID)
	if err != nil || len(workers) != 1 || workers[0].Lifecycle != workerLifecycle {
		t.Fatalf("unexpected durable worker outcome: workers=%#v err=%v", workers, err)
	}
	commands, err := store.ListCommands(outcome.Run.RunID)
	if err != nil || len(commands) != 1 || commands[0].State != workerLifecycle {
		t.Fatalf("unexpected durable command outcome: commands=%#v err=%v", commands, err)
	}
	events, err := store.ListEvents(outcome.Run.RunID)
	if err != nil || len(events) != 1 || events[0].EventType != "run.lifecycle.failure" || !stringsContainAll(string(events[0].Frame), `"stage":"`+string(stage)+`"`) {
		t.Fatalf("unexpected durable failure event: events=%#v err=%v", events, err)
	}
}

func stringsContainAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}

type failingLifecycleStore struct {
	SimopsStore
	failCreate              bool
	failSaveLaunch          int
	failStreamingTransition bool
	failArtifact            bool
}

func (s *failingLifecycleStore) CreateRun(record SimopsRunRecord, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) (SimopsRunRecord, bool, error) {
	if s.failCreate {
		return SimopsRunRecord{}, false, errors.New("create Run failed")
	}
	return s.SimopsStore.CreateRun(record, workers, commands)
}

func (s *failingLifecycleStore) SaveLaunch(runID string, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) error {
	if s.failSaveLaunch > 0 {
		s.failSaveLaunch--
		return errors.New("save launch failed")
	}
	return s.SimopsStore.SaveLaunch(runID, workers, commands)
}

func (s *failingLifecycleStore) UpdateRunLifecycle(runID string, lifecycle SimopsLifecycle) (SimopsRunRecord, error) {
	if s.failStreamingTransition && lifecycle == SimopsStreaming {
		return SimopsRunRecord{}, errors.New("streaming transition failed")
	}
	return s.SimopsStore.UpdateRunLifecycle(runID, lifecycle)
}

func (s *failingLifecycleStore) SaveArtifact(record SimopsArtifactRecord) error {
	if s.failArtifact {
		return errors.New("save artifact failed")
	}
	return s.SimopsStore.SaveArtifact(record)
}

type trackingLifecycleSpooler struct {
	delegate ContractSimopsSpooler
	starts   int
	stops    int
	failStop bool
}

type silentPartialLifecycleSpooler struct {
	delegate ContractSimopsSpooler
	stops    int
}

type recoveringLifecycleSpooler struct {
	observations []ObservedWorkerLifecycle
	stops        int
	syncs        int
}

func (s *recoveringLifecycleSpooler) StartRun(context.Context, SimopsRunRecord, []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	return nil, nil, errors.New("stranded recovery must not relaunch")
}

func (s *recoveringLifecycleSpooler) StopRun(context.Context, string) error {
	s.stops++
	return nil
}

func (s *recoveringLifecycleSpooler) SyncRun(context.Context, SimopsRunRecord, []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	s.syncs++
	return append([]ObservedWorkerLifecycle(nil), s.observations...), nil
}

type blockingRecoveryLifecycleSpooler struct {
	delegate ContractSimopsSpooler
}

func (s *blockingRecoveryLifecycleSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	return s.delegate.StartRun(ctx, run, workers)
}

func (*blockingRecoveryLifecycleSpooler) StopRun(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingRecoveryLifecycleSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	return s.delegate.SyncRun(ctx, run, workers)
}

func (s *silentPartialLifecycleSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	records, commands, err := s.delegate.StartRun(ctx, run, workers[:1])
	return records, commands, err
}

func (s *silentPartialLifecycleSpooler) StopRun(ctx context.Context, runID string) error {
	s.stops++
	return s.delegate.StopRun(ctx, runID)
}

func (s *silentPartialLifecycleSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	return s.delegate.SyncRun(ctx, run, workers)
}

func (s *trackingLifecycleSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	s.starts++
	return s.delegate.StartRun(ctx, run, workers)
}

func (s *trackingLifecycleSpooler) StopRun(ctx context.Context, runID string) error {
	s.stops++
	if s.failStop {
		return errors.New("stop failed")
	}
	return s.delegate.StopRun(ctx, runID)
}

func (s *trackingLifecycleSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	return s.delegate.SyncRun(ctx, run, workers)
}

type invalidLifecycleArtifactPlanner struct{}

func (invalidLifecycleArtifactPlanner) PlanArtifact(run SimopsRunRecord) SimopsArtifactRecord {
	return SimopsArtifactRecord{RunID: run.RunID}
}

type failingLifecycleEventLog struct{}

func (failingLifecycleEventLog) Publish(context.Context, SimopsEvent) error {
	return fmt.Errorf("event publication failed")
}

type partialFailureSimopsSpooler struct {
	now   time.Time
	stops int
}

func (s *partialFailureSimopsSpooler) StartRun(_ context.Context, run SimopsRunRecord, _ []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	return []SimopsWorkerRecord{{
			RunID: run.RunID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler,
			Lifecycle: SimopsStarting, LaunchMode: "auto", Runtime: "test", RuntimeID: "runtime-scheduler", UpdatedAt: s.now,
		}}, []SimopsSpoolCommand{{
			CommandID: run.RunID + "-scheduler-01-start", RunID: run.RunID, WorkerID: "scheduler-01",
			Mode: "auto", State: SimopsStarting, Message: "scheduler launched", CreatedAt: s.now, UpdatedAt: s.now,
		}}, errors.New("storage worker launch failed")
}

func (s *partialFailureSimopsSpooler) StopRun(_ context.Context, _ string) error {
	s.stops++
	return nil
}

func (s *partialFailureSimopsSpooler) SyncRun(_ context.Context, _ SimopsRunRecord, _ []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	return nil, nil
}
