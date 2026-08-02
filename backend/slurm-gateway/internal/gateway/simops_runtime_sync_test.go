package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSimopsControllerSyncsObservedWorkerLifecycleThroughProfileInterface(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	spooler := &syncingSimopsSpooler{
		observations: []ObservedWorkerLifecycle{{
			RunID:     "RUN-SYNC-001",
			WorkerID:  "scheduler-01",
			State:     ObservedWorkerActive,
			Runtime:   "docker",
			RuntimeID: "container-active",
			Reason:    "running",
			Message:   "container is running",
		}},
	}
	controller, err := NewDefaultSimopsControllerWithSpooler(cfg, spooler)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	controller.runID = func() string { return "RUN-SYNC-001" }
	controller.now = fixedGatewayRuntimeNow

	_, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID:  "scheduler-drift",
		WorkerKinds: []string{"scheduler"},
		LaunchMode:  "auto",
	}, "react-backend-client")
	if err != nil {
		t.Fatalf("create run status=%d: %v", status, err)
	}

	response, status, err := controller.GetRun("RUN-SYNC-001")
	if err != nil {
		t.Fatalf("get run status=%d: %v", status, err)
	}
	if spooler.profileSyncs != 1 {
		t.Fatalf("expected one profile sync, got %d", spooler.profileSyncs)
	}
	if len(spooler.syncProfiles) != 1 || spooler.syncProfiles[0].WorkerID != "scheduler-01" {
		t.Fatalf("unexpected sync profiles %#v", spooler.syncProfiles)
	}
	if len(response.Workers) != 1 {
		t.Fatalf("expected one worker, got %#v", response.Workers)
	}
	worker := response.Workers[0]
	if worker.ObservedLifecycle != ObservedWorkerActive {
		t.Fatalf("expected active observed lifecycle, got %#v", worker)
	}
	if worker.RuntimeID != "container-active" || worker.ObservedReason != "running" {
		t.Fatalf("expected runtime observation details, got %#v", worker)
	}
	if worker.Lifecycle != SimopsStarting {
		t.Fatalf("expected sync not to derive generic worker lifecycle from observed runtime state, got %q", worker.Lifecycle)
	}
}

func TestSimopsControllerGetRunReturnsStoredRunWhenRuntimeSyncFails(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	spooler := &syncingSimopsSpooler{syncErr: errors.New("docker unavailable")}
	controller, err := NewDefaultSimopsControllerWithSpooler(cfg, spooler)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	controller.runID = func() string { return "RUN-SYNC-ERROR" }
	controller.now = fixedGatewayRuntimeNow

	_, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID:  "scheduler-drift",
		WorkerKinds: []string{"scheduler"},
		LaunchMode:  "auto",
	}, "react-backend-client")
	if err != nil {
		t.Fatalf("create run status=%d: %v", status, err)
	}

	response, status, err := controller.GetRun("RUN-SYNC-ERROR")
	if err != nil {
		t.Fatalf("expected stored run response despite sync error, status=%d err=%v", status, err)
	}
	if status != 200 {
		t.Fatalf("expected status 200, got %d", status)
	}
	if spooler.profileSyncs != 1 {
		t.Fatalf("expected one attempted sync, got %d", spooler.profileSyncs)
	}
	if len(response.Workers) != 1 || response.Workers[0].ObservedLifecycle != "" {
		t.Fatalf("expected stored worker without observed lifecycle, got %#v", response.Workers)
	}
}

func TestContractSimopsSpoolerSyncReportsWorkerRecordsPresent(t *testing.T) {
	now := fixedGatewayRuntimeNow()
	spooler := ContractSimopsSpooler{Mode: "resident", Now: func() time.Time { return now }}
	observations, err := spooler.SyncRun(context.Background(), SimopsRunRecord{RunID: "RUN-CONTRACT-SYNC"}, []SimopsWorkerRecord{{
		RunID:      "RUN-CONTRACT-SYNC",
		WorkerID:   "scheduler-01",
		WorkerKind: SimopsWorkerScheduler,
		Labels:     map[string]string{"simops.mode": "resident"},
	}})
	if err != nil {
		t.Fatalf("sync run: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %#v", observations)
	}
	got := observations[0]
	if got.State != ObservedWorkerActive || got.Runtime != "contract" || got.Reason != "contract-runtime" {
		t.Fatalf("unexpected contract runtime observation %#v", got)
	}
	if !got.ObservedAt.Equal(now) {
		t.Fatalf("expected observed time %s, got %s", now, got.ObservedAt)
	}
}

func TestContractSimopsSpoolerUsesRunConnectionProfilesForRuntimeContract(t *testing.T) {
	now := fixedGatewayRuntimeNow()
	cfg := testRunConnectionProfileConfig()
	run := SimopsRunRecord{
		RunID: "RUN-CONTRACT-PROFILE", ScenarioID: "scheduler-drift", LaunchMode: "auto",
		IngestToken: "ingest-token", Lifecycle: SimopsStreaming,
	}
	profiles, err := BuildRunWorkerConnectionProfiles(cfg, run, []SimopsWorkerKind{SimopsWorkerScheduler, SimopsWorkerStorage})
	if err != nil {
		t.Fatalf("build profiles: %v", err)
	}
	spooler := ContractSimopsSpooler{Mode: "auto", Now: func() time.Time { return now }}

	workers, commands, err := spooler.StartRunProfiles(context.Background(), run, profiles)
	if err != nil {
		t.Fatalf("start profiles: %v", err)
	}
	if len(workers) != 2 || len(commands) != 2 {
		t.Fatalf("expected two contract workers and commands, workers=%#v commands=%#v", workers, commands)
	}
	if workers[0].Runtime != "contract" || workers[0].RuntimeID != "contract://RUN-CONTRACT-PROFILE/scheduler-01" {
		t.Fatalf("expected stable contract runtime identity, got %#v", workers[0])
	}
	if workers[0].Endpoint != profiles[0].Gateway.IngestURL {
		t.Fatalf("expected profile ingest URL, got %#v", workers[0])
	}
	if workers[0].Labels["simops.runtime"] != "contract" || workers[0].Labels["simops.runtime_adapter"] != "contract" || workers[0].Labels["simops.worker_id"] != "scheduler-01" {
		t.Fatalf("expected profile labels plus contract runtime labels, got %#v", workers[0].Labels)
	}
	wantMetadata := map[string]string{
		"runtime_adapter": "contract",
		"runtime_id":      "contract://RUN-CONTRACT-PROFILE/scheduler-01",
		"requested_state": "starting",
		"worker_id":       "scheduler-01",
		"worker_kind":     "scheduler",
	}
	if commands[0].CommandID != "RUN-CONTRACT-PROFILE-scheduler-01-start" || commands[0].WorkerID != "scheduler-01" || commands[0].Mode != "auto" || commands[0].State != SimopsStarting {
		t.Fatalf("expected stable contract command fields, got %#v", commands[0])
	}
	if !mapsEqual(commands[0].Metadata, wantMetadata) {
		t.Fatalf("expected requested runtime metadata %#v, got %#v", wantMetadata, commands[0].Metadata)
	}

	observations, err := spooler.SyncRunProfiles(context.Background(), run, profiles)
	if err != nil {
		t.Fatalf("sync profiles: %v", err)
	}
	if len(observations) != 2 || observations[0].State != ObservedWorkerActive || observations[0].RuntimeID != workers[0].RuntimeID {
		t.Fatalf("expected active contract observations, got %#v", observations)
	}
	if observations[0].Runtime != "contract" || observations[0].Reason != "contract-runtime" || observations[0].Labels["simops.worker_id"] != "scheduler-01" {
		t.Fatalf("expected active contract observation metadata, got %#v", observations[0])
	}

	completeRun := run
	completeRun.Lifecycle = SimopsComplete
	observations, err = spooler.SyncRunProfiles(context.Background(), completeRun, profiles[:1])
	if err != nil {
		t.Fatalf("sync complete profiles: %v", err)
	}
	if observations[0].State != ObservedWorkerSucceeded || observations[0].Reason != "contract-terminal" {
		t.Fatalf("expected terminal success mapping, got %#v", observations[0])
	}

	failedRun := run
	failedRun.Lifecycle = SimopsFailed
	observations, err = spooler.SyncRunProfiles(context.Background(), failedRun, profiles[:1])
	if err != nil {
		t.Fatalf("sync failed profiles: %v", err)
	}
	if observations[0].State != ObservedWorkerFailed || observations[0].Reason != "contract-retryable-failure" {
		t.Fatalf("expected stable retryable failure mapping, got %#v", observations[0])
	}

	stoppedRun := run
	stoppedRun.Lifecycle = SimopsStopped
	if err := spooler.StopRunProfiles(context.Background(), stoppedRun.RunID, profiles); err != nil {
		t.Fatalf("stop profiles: %v", err)
	}
	if err := spooler.CleanupRunProfiles(context.Background(), stoppedRun.RunID, profiles); err != nil {
		t.Fatalf("cleanup profiles: %v", err)
	}
	observations, err = spooler.SyncRunProfiles(context.Background(), stoppedRun, profiles[:1])
	if err != nil {
		t.Fatalf("sync stopped profiles: %v", err)
	}
	if observations[0].State != ObservedWorkerStopped || observations[0].Reason != "contract-stopped" {
		t.Fatalf("expected stopped contract mapping, got %#v", observations[0])
	}
}

func mapsEqual(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range right {
		if left[key] != value {
			return false
		}
	}
	return true
}

func TestWorkerTelemetryDoesNotOverwriteObservedRuntimeLifecycle(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	spooler := &syncingSimopsSpooler{
		observations: []ObservedWorkerLifecycle{{
			RunID:     "RUN-SYNC-TELEMETRY",
			WorkerID:  "scheduler-01",
			State:     ObservedWorkerFailed,
			Runtime:   "docker",
			RuntimeID: "container-failed",
			Reason:    "exit-code",
			ExitCode:  intPtr(17),
		}},
	}
	controller, err := NewDefaultSimopsControllerWithSpooler(cfg, spooler)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	controller.runID = func() string { return "RUN-SYNC-TELEMETRY" }
	controller.now = fixedGatewayRuntimeNow

	_, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID:  "scheduler-drift",
		WorkerKinds: []string{"scheduler"},
		LaunchMode:  "auto",
	}, "react-backend-client")
	if err != nil {
		t.Fatalf("create run status=%d: %v", status, err)
	}
	if _, status, err := controller.GetRun("RUN-SYNC-TELEMETRY"); err != nil {
		t.Fatalf("initial sync status=%d: %v", status, err)
	}

	record, err := controller.store.GetRun("RUN-SYNC-TELEMETRY")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	frames, status, err := controller.Ingest(
		context.Background(),
		"RUN-SYNC-TELEMETRY",
		record.IngestToken,
		strings.NewReader(telemetryBatch("RUN-SYNC-TELEMETRY", "scheduler-01")),
	)
	if err != nil {
		t.Fatalf("ingest status=%d: %v", status, err)
	}
	if frames != 1 {
		t.Fatalf("expected one ingested frame, got %d", frames)
	}

	response, status, err := controller.GetRun("RUN-SYNC-TELEMETRY")
	if err != nil {
		t.Fatalf("get run status=%d: %v", status, err)
	}
	worker := response.Workers[0]
	if worker.Frames != 1 {
		t.Fatalf("expected telemetry frame count to update, got %#v", worker)
	}
	if worker.ObservedLifecycle != ObservedWorkerFailed || worker.ObservedReason != "exit-code" {
		t.Fatalf("expected telemetry to preserve observed runtime failure, got %#v", worker)
	}
	if worker.Lifecycle != SimopsStreaming {
		t.Fatalf("expected telemetry lifecycle to update independently from observed runtime lifecycle, got %#v", worker)
	}
	if worker.ObservedExitCode == nil || *worker.ObservedExitCode != 17 {
		t.Fatalf("expected observed exit code to survive telemetry ingest, got %#v", worker)
	}
}

func TestInMemoryStoreUpdatesObservedWorkerLifecycleWithoutIncrementingFrames(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{
		RunID:      "RUN-STORE-SYNC",
		ScenarioID: "scheduler-drift",
		Lifecycle:  SimopsStreaming,
		CreatedAt:  fixedGatewayRuntimeNow(),
		UpdatedAt:  fixedGatewayRuntimeNow(),
	}
	worker := SimopsWorkerRecord{
		RunID:      run.RunID,
		WorkerID:   "scheduler-01",
		WorkerKind: SimopsWorkerScheduler,
		Lifecycle:  SimopsStarting,
		LaunchMode: "auto",
		UpdatedAt:  fixedGatewayRuntimeNow(),
	}
	if _, _, err := store.CreateRun(run, []SimopsWorkerRecord{worker}, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	observedAt := fixedGatewayRuntimeNow().Add(time.Minute)
	if err := store.UpdateWorkerObservedLifecycle(ObservedWorkerLifecycle{
		RunID:      run.RunID,
		WorkerID:   "scheduler-01",
		WorkerKind: SimopsWorkerScheduler,
		State:      ObservedWorkerSucceeded,
		Runtime:    "docker",
		RuntimeID:  "container-success",
		Reason:     "exited",
		Message:    "exit code 0",
		ObservedAt: observedAt,
	}); err != nil {
		t.Fatalf("update observed lifecycle: %v", err)
	}
	if err := store.UpdateWorkerFrames(run.RunID, "scheduler-01", SimopsStreaming, 1); err != nil {
		t.Fatalf("update frames: %v", err)
	}

	workers, err := store.ListWorkers(run.RunID)
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected one worker, got %#v", workers)
	}
	got := workers[0]
	if got.Frames != 1 {
		t.Fatalf("expected frame count update, got %#v", got)
	}
	if got.ObservedLifecycle != ObservedWorkerSucceeded || got.RuntimeID != "container-success" {
		t.Fatalf("expected observed lifecycle to survive frame update, got %#v", got)
	}
	if got.Lifecycle != SimopsStreaming {
		t.Fatalf("expected frame update to preserve legacy worker lifecycle behavior, got %q", got.Lifecycle)
	}
}

func TestDataPlaneAndArtifactUpdatesDoNotMutateObservedRuntimeLifecycle(t *testing.T) {
	store := NewInMemorySimopsStore()
	now := fixedGatewayRuntimeNow()
	run := SimopsRunRecord{
		RunID:      "RUN-DATAPLANE-BOUNDARY",
		ScenarioID: "scheduler-drift",
		Lifecycle:  SimopsStreaming,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	worker := SimopsWorkerRecord{
		RunID:      run.RunID,
		WorkerID:   "scheduler-01",
		WorkerKind: SimopsWorkerScheduler,
		Lifecycle:  SimopsStarting,
		LaunchMode: "auto",
		UpdatedAt:  now,
	}
	if _, _, err := store.CreateRun(run, []SimopsWorkerRecord{worker}, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := store.UpdateWorkerObservedLifecycle(ObservedWorkerLifecycle{
		RunID:      run.RunID,
		WorkerID:   worker.WorkerID,
		WorkerKind: worker.WorkerKind,
		State:      ObservedWorkerActive,
		Runtime:    "docker",
		RuntimeID:  "container-active",
		Reason:     "running",
		Message:    "container is running",
		ObservedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("update observed lifecycle: %v", err)
	}
	if err := store.SaveEvent(SimopsEvent{
		RunID:      run.RunID,
		WorkerID:   worker.WorkerID,
		EventType:  "redpanda.health",
		Lifecycle:  SimopsFailed,
		OccurredAt: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("save event: %v", err)
	}
	artifact := SimopsArtifactRecord{
		ArtifactID: "iceberg-telemetry-" + run.RunID,
		RunID:      run.RunID,
		Kind:       "iceberg-table-partition",
		MediaType:  "application/vnd.apache.iceberg.table",
		Status:     SimopsArtifactStatusReceived,
		Location:   "s3://radiant-simops/warehouse/simops_telemetry/run_id=" + run.RunID,
		CreatedAt:  now,
	}
	if err := store.SaveArtifact(artifact); err != nil {
		t.Fatalf("save artifact: %v", err)
	}
	if err := store.UpdateArtifactStatus(run.RunID, artifact.ArtifactID, SimopsArtifactStatusFailed); err != nil {
		t.Fatalf("update artifact: %v", err)
	}

	workers, err := store.ListWorkers(run.RunID)
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	got := workers[0]
	if got.ObservedLifecycle != ObservedWorkerActive || got.Runtime != "docker" || got.RuntimeID != "container-active" {
		t.Fatalf("expected data-plane/artifact updates to leave observed runtime lifecycle untouched, got %#v", got)
	}
}

type syncingSimopsSpooler struct {
	recordingSimopsSpooler
	profileSyncs int
	syncProfiles []RunConnectionProfile
	observations []ObservedWorkerLifecycle
	syncErr      error
}

func (s *syncingSimopsSpooler) SyncRunProfiles(ctx context.Context, _ SimopsRunRecord, profiles []RunConnectionProfile) ([]ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	s.profileSyncs++
	s.syncProfiles = append([]RunConnectionProfile(nil), profiles...)
	if s.syncErr != nil {
		return nil, s.syncErr
	}
	return append([]ObservedWorkerLifecycle(nil), s.observations...), nil
}

func (s *syncingSimopsSpooler) SyncRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	profiles, err := BuildRunWorkerConnectionProfilesForRecords(testRunConnectionProfileConfig(), run, workers)
	if err != nil {
		return nil, err
	}
	return s.SyncRunProfiles(ctx, run, profiles)
}

func intPtr(value int) *int {
	return &value
}
