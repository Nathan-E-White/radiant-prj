package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestReactorTelemetryManagerRegistersBoundedStableWorkerSet(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := NewInMemoryReactorTelemetryStore()
	runtime := &recordingReactorTelemetryRuntime{}
	workbench := NewWorkbenchController(DefaultConfig().Workbench, NewInMemoryWorkbenchStore(), nil)
	manager := NewReactorTelemetryManager(ReactorTelemetryConfig{
		MaxReactorsPerSession: 4,
		WorkersPerSet:         3,
		SessionTTL:            24 * time.Hour,
		MeasuredRetention:     24 * time.Hour,
		CleanupTimeout:        5 * time.Minute,
		ReconcileInterval:     30 * time.Second,
		IngestBaseURL:         "http://gateway:8080",
		CredentialSigningKey:  "test-only-signing-key",
	}, store, runtime, workbench)
	manager.now = func() time.Time { return now }

	request := RegisterDynamicReactorRequest{
		GameSessionID:  "session-opaque-a",
		ReactorID:      "reactor-opaque-a",
		IdempotencyKey: "register-opaque-a",
	}
	set, created, err := manager.RegisterDynamicReactor(context.Background(), request)
	if err != nil {
		t.Fatalf("register dynamic reactor: %v", err)
	}
	if !created {
		t.Fatal("first registration must report created")
	}
	if set.Lifecycle != ReactorTelemetryActive || len(set.Workers) != 3 {
		t.Fatalf("unexpected worker set: %#v", set)
	}
	if set.ExpiresAt.Sub(now) != 24*time.Hour || set.MeasuredRetentionSec != int64((24*time.Hour)/time.Second) {
		t.Fatalf("expected bounded session and measured retention, got %#v", set)
	}
	if len(runtime.launches) != 1 || len(runtime.launches[0].Workers) != 3 {
		t.Fatalf("expected one three-worker launch, got %#v", runtime.launches)
	}

	seenSources := map[string]struct{}{}
	for _, worker := range runtime.launches[0].Workers {
		if worker.GameSessionID != request.GameSessionID || worker.ReactorID != request.ReactorID {
			t.Fatalf("worker identity lost reactor scope: %#v", worker)
		}
		if worker.Gateway.IngestBaseURL != "http://gateway:8080" || worker.Gateway.IngestToken == "" {
			t.Fatalf("worker missing gateway-only credential: %#v", worker.Gateway)
		}
		if worker.BrokerURL != "" || worker.DatabaseURL != "" || worker.LakeURL != "" || worker.ContainerSocket != "" || worker.ClusterCredential != "" {
			t.Fatalf("ordinary resident worker received forbidden data-plane/runtime credentials: %#v", worker)
		}
		if _, duplicate := seenSources[worker.SourceID]; duplicate {
			t.Fatalf("duplicate source identity %q", worker.SourceID)
		}
		seenSources[worker.SourceID] = struct{}{}
	}

	recovered, recoveredCreated, err := manager.RegisterDynamicReactor(context.Background(), request)
	if err != nil {
		t.Fatalf("recover registration: %v", err)
	}
	if recoveredCreated || recovered.SetID != set.SetID || len(runtime.launches) != 1 {
		t.Fatalf("idempotent registration created a parallel lifecycle: %#v", recovered)
	}

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restarted := NewReactorTelemetryManager(manager.cfg, store, restartedRuntime, workbench)
	restarted.now = manager.now
	afterRestart, afterRestartCreated, err := restarted.RegisterDynamicReactor(context.Background(), request)
	if err != nil {
		t.Fatalf("recover after manager restart: %v", err)
	}
	if afterRestartCreated || afterRestart.SetID != set.SetID || len(restartedRuntime.launches) != 0 {
		t.Fatalf("restart did not recover stable worker identity: %#v", afterRestart)
	}
	if err := restarted.ReconcileActive(context.Background()); err != nil {
		t.Fatalf("reconcile active workers after restart: %v", err)
	}
	if len(restartedRuntime.launches) != 1 || restartedRuntime.launches[0].SetID != set.SetID {
		t.Fatalf("restart reconciliation changed set identity: %#v", restartedRuntime.launches)
	}
}

func TestReactorTelemetryManagerEnforcesSessionAndWorkerCaps(t *testing.T) {
	cfg := DefaultReactorTelemetryConfig()
	cfg.MaxReactorsPerSession = 4
	cfg.WorkersPerSet = 3
	manager := NewReactorTelemetryManager(cfg, NewInMemoryReactorTelemetryStore(), &recordingReactorTelemetryRuntime{}, nil)

	for index := 1; index <= 4; index++ {
		set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
			GameSessionID: "bounded-session", ReactorID: reactorID(index), IdempotencyKey: reactorID(index),
		})
		if err != nil {
			t.Fatalf("register reactor %d: %v", index, err)
		}
		if len(set.Workers) > 3 {
			t.Fatalf("worker set exceeded ADR cap: %d", len(set.Workers))
		}
	}
	_, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "bounded-session", ReactorID: "reactor-5", IdempotencyKey: "reactor-5",
	})
	if !errors.Is(err, ErrReactorTelemetrySessionCap) {
		t.Fatalf("expected session cap error, got %v", err)
	}

	invalid := cfg
	invalid.WorkersPerSet = 4
	if err := invalid.Validate(); err == nil {
		t.Fatal("configuration must not raise the accepted three-worker cap")
	}
}

func TestReactorTelemetryWorkerPublishesReactorScopedMeasuredState(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	controller := NewWorkbenchController(DefaultConfig().Workbench, store, nil)
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), NewInMemoryReactorTelemetryStore(), &recordingReactorTelemetryRuntime{}, controller)
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-measured", ReactorID: "reactor-measured", IdempotencyKey: "register-measured",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	worker := set.Workers[0]
	frames := BuildReactorTelemetryFrames(worker, 1, time.Date(2026, 7, 14, 9, 1, 0, 0, time.UTC))
	if len(frames) == 0 {
		t.Fatal("worker must emit at least one measured frame")
	}
	for index, frame := range frames {
		if frame.SourceID != worker.SourceID || frame.ReactorID != set.ReactorID || frame.ValueBasis != WorkbenchValueMeasured {
			t.Fatalf("frame lost source, reactor, or Value Basis identity: %#v", frame)
		}
		if err := controller.validateScadaFrame(frame); err != nil {
			t.Fatalf("accepted worker frame failed Workbench validation: %v", err)
		}
		raw, _ := jsonMarshal(frame)
		projection, err := ProjectScadaFrame("scada.telemetry.v1", 0, int64(index+1), raw)
		if err != nil {
			t.Fatalf("project frame: %v", err)
		}
		if _, err := store.SaveScadaProjection("reactor-test", projection); err != nil {
			t.Fatalf("save measured projection: %v", err)
		}
	}
	measured, err := controller.Measured()
	if err != nil {
		t.Fatalf("read measured state: %v", err)
	}
	if len(measured) != len(frames) {
		t.Fatalf("expected %d measured frames, got %d", len(frames), len(measured))
	}
	for _, frame := range measured {
		if frame.ReactorID != "reactor-measured" || frame.ValueBasis != WorkbenchValueMeasured {
			t.Fatalf("Workbench returned incorrectly scoped measured state: %#v", frame)
		}
	}
}

func TestReactorTelemetryManagerRevokesBeforeBoundedCleanup(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	store := NewInMemoryReactorTelemetryStore()
	runtime := &recordingReactorTelemetryRuntime{stopErr: errors.New("runtime unavailable")}
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, runtime, nil)
	manager.now = func() time.Time { return now }
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-remove", ReactorID: "reactor-remove", IdempotencyKey: "register-remove",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	removed, err := manager.RemoveDynamicReactor(context.Background(), RemoveDynamicReactorRequest{
		GameSessionID: "session-remove", ReactorID: "reactor-remove", IdempotencyKey: "remove-reactor",
	})
	if err == nil {
		t.Fatal("cleanup failure must remain visible")
	}
	if removed.Lifecycle != ReactorTelemetryCleanupFailed || !removed.CredentialsRevoked {
		t.Fatalf("expected revoked retryable cleanup state, got %#v", removed)
	}
	if removed.CleanupDeadline.Sub(now) != 5*time.Minute {
		t.Fatalf("cleanup deadline is not bounded: %s", removed.CleanupDeadline)
	}
	if len(runtime.stops) != 1 || runtime.stops[0] != set.SetID {
		t.Fatalf("runtime cleanup did not target stable set identity: %#v", runtime.stops)
	}
	for _, worker := range set.Workers {
		if manager.AuthorizeSourceCredential(worker.Gateway.IngestToken, worker.SourceID, set.ReactorID) {
			t.Fatalf("credential for %s remained valid after removal began", worker.SourceID)
		}
	}

	runtime.stopErr = nil
	retried, err := manager.RemoveDynamicReactor(context.Background(), RemoveDynamicReactorRequest{
		GameSessionID: "session-remove", ReactorID: "reactor-remove", IdempotencyKey: "remove-reactor",
	})
	if err != nil || retried.Lifecycle != ReactorTelemetryRemoved {
		t.Fatalf("retry did not complete bounded cleanup: %#v, %v", retried, err)
	}
}

func TestReactorTelemetryManagerBoundsUncooperativeRuntimeCleanup(t *testing.T) {
	cfg := DefaultReactorTelemetryConfig()
	cfg.CleanupTimeout = 20 * time.Millisecond
	store := NewInMemoryReactorTelemetryStore()
	manager := NewReactorTelemetryManager(cfg, store, blockingCleanupRuntime{}, nil)
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-blocking-cleanup", ReactorID: "reactor-blocking-cleanup", IdempotencyKey: "register-blocking-cleanup",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	started := time.Now()
	failed, err := manager.RemoveDynamicReactor(context.Background(), RemoveDynamicReactorRequest{
		GameSessionID: set.GameSessionID, ReactorID: set.ReactorID, IdempotencyKey: "remove-blocking-cleanup",
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocking cleanup did not expose its deadline: set=%#v err=%v", failed, err)
	}
	if time.Since(started) > time.Second || failed.Lifecycle != ReactorTelemetryCleanupFailed {
		t.Fatalf("cleanup did not enter retryable failure within the configured bound: %#v", failed)
	}
}

func TestReactorTelemetryManagerReconcilesInterruptedAndFailedCleanup(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	store := NewInMemoryReactorTelemetryStore()
	firstRuntime := &recordingReactorTelemetryRuntime{stopErr: errors.New("runtime unavailable")}
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, firstRuntime, nil)
	manager.now = func() time.Time { return now }
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-cleanup-reconcile", ReactorID: "reactor-cleanup-reconcile", IdempotencyKey: "register-cleanup-reconcile",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := manager.RemoveDynamicReactor(context.Background(), RemoveDynamicReactorRequest{
		GameSessionID: set.GameSessionID, ReactorID: set.ReactorID, IdempotencyKey: "remove-cleanup-reconcile",
	}); err == nil {
		t.Fatal("expected first cleanup attempt to fail")
	}

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restarted := NewReactorTelemetryManager(manager.cfg, store, restartedRuntime, nil)
	restarted.now = manager.now
	if err := restarted.ReconcileActive(context.Background()); err != nil {
		t.Fatalf("reconcile cleanup after restart: %v", err)
	}
	reconciled, err := store.GetWorkerSet(set.GameSessionID, set.ReactorID)
	if err != nil || reconciled.Lifecycle != ReactorTelemetryRemoved {
		t.Fatalf("cleanup was not completed after restart: %#v err=%v", reconciled, err)
	}
	if len(restartedRuntime.stops) != 1 || restartedRuntime.stops[0] != set.SetID {
		t.Fatalf("restart cleanup did not target stable set: %#v", restartedRuntime.stops)
	}

	interrupted := set
	interrupted.Lifecycle = ReactorTelemetryCleanup
	interrupted.CredentialsRevoked = true
	interrupted.CleanupDeadline = now.Add(5 * time.Minute)
	if err := store.SaveWorkerSet(interrupted); err != nil {
		t.Fatalf("persist interrupted cleanup: %v", err)
	}
	if err := restarted.ReconcileExpired(context.Background()); err != nil {
		t.Fatalf("periodic reconcile interrupted cleanup: %v", err)
	}
	if len(restartedRuntime.stops) != 2 {
		t.Fatalf("periodic reconcile did not retry interrupted cleanup: %#v", restartedRuntime.stops)
	}
}

func TestReactorTelemetryManagerRetriesFailedLaunchWithoutChangingIdentity(t *testing.T) {
	runtime := &recordingReactorTelemetryRuntime{startErr: errors.New("runtime unavailable")}
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), NewInMemoryReactorTelemetryStore(), runtime, nil)
	request := RegisterDynamicReactorRequest{
		GameSessionID: "session-retry", ReactorID: "reactor-retry", IdempotencyKey: "register-retry",
	}
	failed, created, err := manager.RegisterDynamicReactor(context.Background(), request)
	if err == nil || !created || failed.Lifecycle != ReactorTelemetryLaunchFailed {
		t.Fatalf("expected visible launch failure, set=%#v created=%v err=%v", failed, created, err)
	}
	runtime.startErr = nil
	recovered, recoveredCreated, err := manager.RegisterDynamicReactor(context.Background(), request)
	if err != nil || recoveredCreated || recovered.Lifecycle != ReactorTelemetryActive || recovered.SetID != failed.SetID {
		t.Fatalf("retry created a parallel or failed lifecycle: set=%#v created=%v err=%v", recovered, recoveredCreated, err)
	}
	if len(runtime.launches) != 2 {
		t.Fatalf("retry did not relaunch the failed set: %#v", runtime.launches)
	}
}

func TestReactorTelemetryManagerAuthorizesOnlyDuringPersistedLaunchWindow(t *testing.T) {
	var manager *ReactorTelemetryManager
	authorizedDuringStart := false
	runtime := &recordingReactorTelemetryRuntime{onStart: func(launch ReactorTelemetryLaunch) {
		worker := launch.Workers[0]
		authorizedDuringStart = manager.AuthorizeSourceCredential(worker.Gateway.IngestToken, worker.SourceID, worker.ReactorID)
	}}
	manager = NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), NewInMemoryReactorTelemetryStore(), runtime, nil)
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-start-auth", ReactorID: "reactor-start-auth", IdempotencyKey: "register-start-auth",
	})
	if err != nil || !authorizedDuringStart {
		t.Fatalf("source-bound credential was unavailable during persisted starting lifecycle: set=%#v err=%v", set, err)
	}
	persisted, err := manager.store.GetWorkerSet(set.GameSessionID, set.ReactorID)
	if err != nil {
		t.Fatalf("read persisted set: %v", err)
	}
	for _, worker := range persisted.Workers {
		if worker.Gateway.IngestToken != "" {
			t.Fatal("control store persisted a plaintext source credential")
		}
	}
}

func TestReactorTelemetryManagerExpiresCredentialsAndReconcilesCleanup(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	runtime := &recordingReactorTelemetryRuntime{}
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), NewInMemoryReactorTelemetryStore(), runtime, nil)
	manager.now = func() time.Time { return now }
	set, _, err := manager.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-expire", ReactorID: "reactor-expire", IdempotencyKey: "register-expire",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	worker := set.Workers[0]
	now = set.ExpiresAt.Add(time.Second)
	if manager.AuthorizeSourceCredential(worker.Gateway.IngestToken, worker.SourceID, set.ReactorID) {
		t.Fatal("expired session credential remained authorized")
	}
	if err := manager.ReconcileExpired(context.Background()); err != nil {
		t.Fatalf("reconcile expiry: %v", err)
	}
	reconciled, err := manager.store.GetWorkerSet(set.GameSessionID, set.ReactorID)
	if err != nil || reconciled.Lifecycle != ReactorTelemetryRemoved || !reconciled.CredentialsRevoked {
		t.Fatalf("expiry did not complete cleanup: %#v err=%v", reconciled, err)
	}
	if len(runtime.stops) != 1 || runtime.stops[0] != set.SetID {
		t.Fatalf("expiry cleanup targeted the wrong set: %#v", runtime.stops)
	}
}

func TestWorkbenchPrunesExpiredDynamicMeasuredStateButPreservesResidentConfiguration(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := NewInMemoryWorkbenchStore()
	store.now = func() time.Time { return now.Add(-2 * time.Hour) }
	controller := NewWorkbenchController(DefaultConfig().Workbench, store, nil)
	controller.now = func() time.Time { return now }
	controller.dynamicMeasuredRetention = time.Hour
	worker := ReactorTelemetryWorker{SourceID: "dynamic-source", ReactorID: "dynamic-reactor", WorkerIndex: 0}
	source := BuildReactorResidentSource(worker)
	if _, err := controller.RegisterSource(source); err != nil {
		t.Fatalf("register protected source declaration: %v", err)
	}
	frames := BuildReactorTelemetryFrames(worker, 1, now.Add(48*time.Hour))
	for index, frame := range frames {
		raw, _ := json.Marshal(frame)
		projection, _ := ProjectScadaFrame("retention", 0, int64(index+1), raw)
		if _, err := store.SaveScadaProjection("retention", projection); err != nil {
			t.Fatalf("save expired dynamic frame: %v", err)
		}
	}
	static := scadaFrameFixture()
	static.ObservedAt = now.Add(-48 * time.Hour)
	static.SampledAt = static.ObservedAt
	raw, _ := json.Marshal(static)
	projection, _ := ProjectScadaFrame("retention", 0, 50, raw)
	if _, err := store.SaveScadaProjection("retention", projection); err != nil {
		t.Fatalf("save static resident frame: %v", err)
	}

	measured, err := controller.Measured()
	if err != nil {
		t.Fatalf("read retained measured state: %v", err)
	}
	if len(measured) != 1 || measured[0].SourceID != static.SourceID {
		t.Fatalf("retention did not isolate expired dynamic frames: %#v", measured)
	}
	if _, err := store.GetResidentTag(source.Tags[0].TagID); err != nil {
		t.Fatalf("retention deleted protected source configuration: %v", err)
	}
}

func TestReactorTelemetryPeriodicReconcilePrunesMeasuredStateWithoutRead(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := NewInMemoryWorkbenchStore()
	store.now = func() time.Time { return now.Add(-2 * time.Hour) }
	controller := NewWorkbenchController(DefaultConfig().Workbench, store, nil)
	controller.now = func() time.Time { return now }
	controller.dynamicMeasuredRetention = time.Hour
	worker := ReactorTelemetryWorker{SourceID: "periodic-source", ReactorID: "periodic-reactor", WorkerIndex: 0}
	for index, frame := range BuildReactorTelemetryFrames(worker, 1, now.Add(48*time.Hour)) {
		raw, _ := json.Marshal(frame)
		projection, _ := ProjectScadaFrame("periodic-retention", 0, int64(index+1), raw)
		if _, err := store.SaveScadaProjection("periodic-retention", projection); err != nil {
			t.Fatalf("save old-ingestion frame: %v", err)
		}
	}
	app := NewGateway(DefaultConfig(), nil, nil, nil)
	app.workbench = controller
	if err := app.ReconcileFleetBoardSessions(context.Background()); err != nil {
		t.Fatalf("periodic retention reconcile: %v", err)
	}
	frames, err := store.LatestMeasuredFrames(100)
	if err != nil || len(frames) != 0 {
		t.Fatalf("periodic reconcile left expired rows until a read: %#v err=%v", frames, err)
	}
}

type recordingReactorTelemetryRuntime struct {
	launches []ReactorTelemetryLaunch
	stops    []string
	stopErr  error
	startErr error
	onStart  func(ReactorTelemetryLaunch)
}

type blockingCleanupRuntime struct{}

func (blockingCleanupRuntime) StartWorkerSet(context.Context, ReactorTelemetryLaunch) error {
	return nil
}

func (blockingCleanupRuntime) StopWorkerSet(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (r *recordingReactorTelemetryRuntime) StartWorkerSet(_ context.Context, launch ReactorTelemetryLaunch) error {
	r.launches = append(r.launches, launch)
	if r.onStart != nil {
		r.onStart(launch)
	}
	return r.startErr
}

func (r *recordingReactorTelemetryRuntime) StopWorkerSet(_ context.Context, setID string) error {
	r.stops = append(r.stops, setID)
	return r.stopErr
}

func reactorID(index int) string {
	return "reactor-" + string(rune('0'+index))
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
