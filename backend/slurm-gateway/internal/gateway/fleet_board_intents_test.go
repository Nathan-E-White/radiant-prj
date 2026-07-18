package gateway

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestFleetBoardDynamicReactorIntentModuleTranslatesRegistration(t *testing.T) {
	manager := &recordingDynamicReactorIntentManager{
		registerSet: ReactorTelemetryWorkerSet{SetID: "set-1", ReactorID: "reactor-1", Lifecycle: ReactorTelemetryActive},
		created:     true,
	}
	module := NewFleetBoardIntentModule(manager, nil)

	result, handled := module.ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{
		Intent: "registerDynamicReactor", GameSessionID: "session-1", ReactorID: "reactor-1", IdempotencyKey: "register-1",
	})

	if !handled || result.Status != http.StatusAccepted {
		t.Fatalf("expected accepted named intent, got handled=%t result=%#v", handled, result)
	}
	body, ok := result.Body.(RegisterDynamicReactorOutcome)
	if !ok || !body.Created || body.WorkerSet.SetID != "set-1" {
		t.Fatalf("unexpected registration outcome: %#v", result.Body)
	}
	if manager.registerRequest != (RegisterDynamicReactorRequest{GameSessionID: "session-1", ReactorID: "reactor-1", IdempotencyKey: "register-1"}) {
		t.Fatalf("intent translation changed identity: %#v", manager.registerRequest)
	}
}

func TestFleetBoardDynamicReactorIntentModuleTranslatesRemoval(t *testing.T) {
	manager := &recordingDynamicReactorIntentManager{
		removeSet: ReactorTelemetryWorkerSet{SetID: "set-1", ReactorID: "reactor-1", Lifecycle: ReactorTelemetryRemoved},
	}
	module := NewFleetBoardIntentModule(manager, nil)

	result, handled := module.ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{
		Intent: "removeDynamicReactor", GameSessionID: "session-1", ReactorID: "reactor-1", IdempotencyKey: "remove-1",
	})

	if !handled || result.Status != http.StatusOK {
		t.Fatalf("expected successful removal, got handled=%t result=%#v", handled, result)
	}
	body, ok := result.Body.(RemoveDynamicReactorOutcome)
	if !ok || body.WorkerSet.Lifecycle != ReactorTelemetryRemoved {
		t.Fatalf("unexpected removal outcome: %#v", result.Body)
	}
	if manager.removeRequest != (RemoveDynamicReactorRequest{GameSessionID: "session-1", ReactorID: "reactor-1", IdempotencyKey: "remove-1"}) {
		t.Fatalf("intent translation changed identity: %#v", manager.removeRequest)
	}
}

func TestFleetBoardDynamicReactorIntentModuleConcentratesExpiryReconciliation(t *testing.T) {
	manager := &recordingDynamicReactorIntentManager{reconcileErr: context.DeadlineExceeded}
	result := NewFleetBoardIntentModule(manager, nil).ReconcileDynamicReactors(context.Background())
	if manager.reconcileCalls != 1 || result == nil || result.Status != http.StatusBadGateway {
		t.Fatalf("expected stable reconciliation failure, calls=%d result=%#v", manager.reconcileCalls, result)
	}
	outcome, ok := result.Body.(FleetBoardIntentError)
	if !ok || outcome.Code != "reactor_telemetry_expiry_cleanup_failed" {
		t.Fatalf("unexpected reconciliation outcome: %#v", result.Body)
	}
}

func TestFleetBoardDynamicReactorIntentModuleRecoversRegistrationAfterManagerRestart(t *testing.T) {
	store := NewInMemoryReactorTelemetryStore()
	failedRuntime := &recordingReactorTelemetryRuntime{startErr: errors.New("runtime unavailable")}
	failedManager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, failedRuntime, nil)
	request := validDynamicReactorIntent("registerDynamicReactor")

	failed, handled := NewFleetBoardIntentModule(failedManager, nil).ExecuteDynamicReactor(context.Background(), request)
	if !handled || failed.Status != http.StatusBadGateway {
		t.Fatalf("expected visible failed registration, handled=%t result=%#v", handled, failed)
	}
	failedSet, err := store.GetWorkerSet(request.GameSessionID, request.ReactorID)
	if err != nil || failedSet.Lifecycle != ReactorTelemetryLaunchFailed {
		t.Fatalf("failed registration was not recoverable: set=%#v err=%v", failedSet, err)
	}

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restartedManager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, restartedRuntime, nil)
	recovered, handled := NewFleetBoardIntentModule(restartedManager, nil).ExecuteDynamicReactor(context.Background(), request)
	if !handled || recovered.Status != http.StatusOK {
		t.Fatalf("expected recovered registration, handled=%t result=%#v", handled, recovered)
	}
	outcome := recovered.Body.(RegisterDynamicReactorOutcome)
	if outcome.Created || outcome.WorkerSet.SetID != failedSet.SetID || outcome.WorkerSet.Lifecycle != ReactorTelemetryActive {
		t.Fatalf("restart created a parallel registration lifecycle: %#v", outcome)
	}
}

func TestFleetBoardDynamicReactorIntentModuleRecoversRemovalAfterManagerRestart(t *testing.T) {
	store := NewInMemoryReactorTelemetryStore()
	failedRuntime := &recordingReactorTelemetryRuntime{stopErr: context.DeadlineExceeded}
	manager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, failedRuntime, nil)
	module := NewFleetBoardIntentModule(manager, nil)
	registered, _ := module.ExecuteDynamicReactor(context.Background(), validDynamicReactorIntent("registerDynamicReactor"))
	set := registered.Body.(RegisterDynamicReactorOutcome).WorkerSet
	remove := FleetBoardIntentRequest{
		Intent: "removeDynamicReactor", GameSessionID: set.GameSessionID, ReactorID: set.ReactorID, IdempotencyKey: "remove-after-restart",
	}

	failed, handled := module.ExecuteDynamicReactor(context.Background(), remove)
	if !handled || failed.Status != http.StatusBadGateway {
		t.Fatalf("expected visible failed cleanup, handled=%t result=%#v", handled, failed)
	}
	failedSet, err := store.GetWorkerSet(set.GameSessionID, set.ReactorID)
	if err != nil || !failedSet.CredentialsRevoked || failedSet.Lifecycle != ReactorTelemetryCleanupFailed {
		t.Fatalf("failed cleanup lost retry state: set=%#v err=%v", failedSet, err)
	}

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restartedManager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, restartedRuntime, nil)
	restartedModule := NewFleetBoardIntentModule(restartedManager, nil)
	if result := restartedModule.ReconcileDynamicReactors(context.Background()); result != nil {
		t.Fatalf("restart cleanup reconciliation failed: %#v", result)
	}
	recovered, handled := restartedModule.ExecuteDynamicReactor(context.Background(), remove)
	if !handled || recovered.Status != http.StatusOK {
		t.Fatalf("expected recovered removal, handled=%t result=%#v", handled, recovered)
	}
	outcome := recovered.Body.(RemoveDynamicReactorOutcome)
	if outcome.WorkerSet.SetID != set.SetID || outcome.WorkerSet.Lifecycle != ReactorTelemetryRemoved || len(restartedRuntime.stops) != 1 {
		t.Fatalf("restart created a parallel removal lifecycle: outcome=%#v stops=%#v", outcome, restartedRuntime.stops)
	}
}

func TestFleetBoardArtifactForgeIntentModuleTranslatesExplicitRequest(t *testing.T) {
	manager := &recordingArtifactForgeIntentManager{
		record:  ArtifactForgeRecord{RequestID: "forge-1", Decision: ArtifactForgeAwaitingRun},
		created: true,
	}
	module := NewFleetBoardIntentModule(nil, manager)
	request := FleetBoardIntentRequest{
		Intent: "requestArtifactForge", GameSessionID: "session-1", ReactorID: "reactor-1",
		SimulationJobID: "local-job-1", SimulationJobState: "completed", SimulationRecipe: ArtifactForgeSchedulerDriftRecipe,
		IdempotencyKey: "forge-click-1",
	}

	result, handled := module.ExecuteArtifactForge(context.Background(), request, "fleet-player")
	body, ok := result.Body.(ArtifactForgeRecord)
	if !handled || result.Status != http.StatusAccepted || !ok || body.RequestID != manager.record.RequestID || body.Decision != manager.record.Decision {
		t.Fatalf("expected accepted Artifact Forge outcome, handled=%t result=%#v", handled, result)
	}
	want := ArtifactForgeRequest{
		GameSessionID: "session-1", ReactorID: "reactor-1", SimulationJobID: "local-job-1",
		SimulationJobState: "completed", SimulationRecipe: ArtifactForgeSchedulerDriftRecipe, IdempotencyKey: "forge-click-1",
	}
	if manager.request != want || manager.identity != "fleet-player" {
		t.Fatalf("Artifact Forge translation changed domain identity: request=%#v identity=%q", manager.request, manager.identity)
	}
}

func TestFleetBoardArtifactForgeIntentModuleClassifiesStableOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		manager    ArtifactForgeIntentManager
		request    FleetBoardIntentRequest
		wantStatus int
		wantCode   string
	}{
		{name: "manager unavailable", request: validArtifactForgeIntent(), wantStatus: http.StatusNotFound, wantCode: "artifact_forge_disabled"},
		{name: "invalid identity", manager: &recordingArtifactForgeIntentManager{}, request: FleetBoardIntentRequest{Intent: "requestArtifactForge"}, wantStatus: http.StatusUnprocessableEntity, wantCode: "artifact_forge_rejected"},
		{name: "manager error", manager: &recordingArtifactForgeIntentManager{err: errors.New("store unavailable")}, request: validArtifactForgeIntent(), wantStatus: http.StatusUnprocessableEntity, wantCode: "artifact_forge_rejected"},
		{name: "intent rejected", manager: &recordingArtifactForgeIntentManager{record: ArtifactForgeRecord{Decision: ArtifactForgeIntentRejected}, created: true}, request: validArtifactForgeIntent(), wantStatus: http.StatusUnprocessableEntity},
		{name: "duplicate", manager: &recordingArtifactForgeIntentManager{record: ArtifactForgeRecord{Decision: ArtifactForgeAwaitingRun}}, request: validArtifactForgeIntent(), wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, handled := NewFleetBoardIntentModule(nil, test.manager).ExecuteArtifactForge(context.Background(), test.request, "fleet-player")
			if !handled || result.Status != test.wantStatus {
				t.Fatalf("expected handled status %d, got handled=%t result=%#v", test.wantStatus, handled, result)
			}
			if test.wantCode != "" {
				outcome, ok := result.Body.(FleetBoardIntentError)
				if !ok || outcome.Code != test.wantCode {
					t.Fatalf("expected code %q, got %#v", test.wantCode, result.Body)
				}
			}
		})
	}
}

func TestFleetBoardArtifactForgeIntentModulePreservesEligibilityOutcome(t *testing.T) {
	for _, decision := range []ArtifactForgeDecision{
		ArtifactForgeRunFailed,
		ArtifactForgeTelemetryIneligible,
		ArtifactForgeArtifactIncomplete,
		ArtifactForgeLineageMissing,
		ArtifactForgeOutcomeApplied,
	} {
		t.Run(string(decision), func(t *testing.T) {
			manager := &recordingArtifactForgeIntentManager{record: ArtifactForgeRecord{Decision: decision}}
			result, handled := NewFleetBoardIntentModule(nil, manager).ExecuteArtifactForge(context.Background(), validArtifactForgeIntent(), "fleet-player")
			if !handled || result.Status != http.StatusOK || result.Body.(ArtifactForgeRecord).Decision != decision {
				t.Fatalf("intent mediation rewrote Artifact Forge authority: handled=%t result=%#v", handled, result)
			}
		})
	}
}

func TestFleetBoardArtifactForgeIntentModuleRecoversDuplicateWithoutParallelRun(t *testing.T) {
	forge, runs, _ := newArtifactForgeTestRig(t)
	module := NewFleetBoardIntentModule(nil, forge)
	request := validArtifactForgeIntent()

	accepted, handled := module.ExecuteArtifactForge(context.Background(), request, "fleet-player")
	if !handled || accepted.Status != http.StatusAccepted {
		t.Fatalf("expected accepted explicit request, handled=%t result=%#v", handled, accepted)
	}
	first := accepted.Body.(ArtifactForgeRecord)
	if first.RunID == "" || first.RunID == request.SimulationJobID {
		t.Fatalf("local Simulation Job was promoted or Run association missing: %#v", first.Trace)
	}

	recovered, handled := module.ExecuteArtifactForge(context.Background(), request, "fleet-player")
	if !handled || recovered.Status != http.StatusOK {
		t.Fatalf("expected stable duplicate recovery, handled=%t result=%#v", handled, recovered)
	}
	duplicate := recovered.Body.(ArtifactForgeRecord)
	if duplicate.RequestID != first.RequestID || duplicate.RunID != first.RunID || len(runs.runs) != 1 {
		t.Fatalf("duplicate request created a parallel lifecycle: first=%#v duplicate=%#v runs=%d", first.Trace, duplicate.Trace, len(runs.runs))
	}
}

func TestFleetBoardArtifactForgeIntentModuleRejectsInvalidIdentityBeforeManager(t *testing.T) {
	manager := &recordingArtifactForgeIntentManager{}
	result, handled := NewFleetBoardIntentModule(nil, manager).ExecuteArtifactForge(context.Background(), FleetBoardIntentRequest{Intent: "requestArtifactForge"}, "fleet-player")
	if !handled || result.Status != http.StatusUnprocessableEntity || manager.calls != 0 {
		t.Fatalf("invalid request reached Artifact Forge: handled=%t calls=%d result=%#v", handled, manager.calls, result)
	}
}

func validArtifactForgeIntent() FleetBoardIntentRequest {
	return FleetBoardIntentRequest{
		Intent: "requestArtifactForge", GameSessionID: "session-1", ReactorID: "reactor-1",
		SimulationJobID: "local-job-1", SimulationJobState: "completed", SimulationRecipe: ArtifactForgeSchedulerDriftRecipe,
		IdempotencyKey: "forge-click-1",
	}
}

type recordingArtifactForgeIntentManager struct {
	request  ArtifactForgeRequest
	identity string
	record   ArtifactForgeRecord
	created  bool
	err      error
	calls    int
}

func (m *recordingArtifactForgeIntentManager) Request(_ context.Context, request ArtifactForgeRequest, identity string) (ArtifactForgeRecord, bool, error) {
	m.calls++
	m.request = request
	m.identity = identity
	return m.record, m.created, m.err
}

var _ ArtifactForgeIntentManager = (*recordingArtifactForgeIntentManager)(nil)

func TestFleetBoardDynamicReactorIntentModuleClassifiesStableOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		manager    DynamicReactorIntentManager
		request    FleetBoardIntentRequest
		wantStatus int
		wantCode   string
	}{
		{
			name:       "manager unavailable",
			request:    FleetBoardIntentRequest{Intent: "registerDynamicReactor"},
			wantStatus: http.StatusNotFound, wantCode: "reactor_telemetry_disabled",
		},
		{
			name:       "session cap",
			manager:    &recordingDynamicReactorIntentManager{registerErr: ErrReactorTelemetrySessionCap},
			request:    validDynamicReactorIntent("registerDynamicReactor"),
			wantStatus: http.StatusConflict, wantCode: "reactor_telemetry_cap",
		},
		{
			name:       "remove missing",
			manager:    &recordingDynamicReactorIntentManager{removeErr: ErrReactorTelemetryNotFound},
			request:    validDynamicReactorIntent("removeDynamicReactor"),
			wantStatus: http.StatusNotFound, wantCode: "reactor_telemetry_cleanup_failed",
		},
		{
			name:       "cleanup retryable",
			manager:    &recordingDynamicReactorIntentManager{removeErr: context.DeadlineExceeded},
			request:    validDynamicReactorIntent("removeDynamicReactor"),
			wantStatus: http.StatusBadGateway, wantCode: "reactor_telemetry_cleanup_failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, handled := NewFleetBoardIntentModule(test.manager, nil).ExecuteDynamicReactor(context.Background(), test.request)
			if !handled || result.Status != test.wantStatus {
				t.Fatalf("expected handled status %d, got handled=%t result=%#v", test.wantStatus, handled, result)
			}
			outcome, ok := result.Body.(FleetBoardIntentError)
			if !ok || outcome.Code != test.wantCode {
				t.Fatalf("expected code %q, got %#v", test.wantCode, result.Body)
			}
		})
	}
}

func validDynamicReactorIntent(intent string) FleetBoardIntentRequest {
	return FleetBoardIntentRequest{Intent: intent, GameSessionID: "session-1", ReactorID: "reactor-1", IdempotencyKey: "intent-1"}
}

func TestFleetBoardDynamicReactorIntentModuleLeavesOtherNamedIntentsForTheirAuthority(t *testing.T) {
	result, handled := NewFleetBoardIntentModule(&recordingDynamicReactorIntentManager{}, nil).ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{Intent: "requestArtifactForge"})
	if handled || result.Status != 0 || result.Body != nil {
		t.Fatalf("dynamic-reactor module absorbed a different named intent: %#v handled=%t", result, handled)
	}
}

func TestFleetBoardDynamicReactorIntentModuleRejectsInvalidIdentityBeforeManager(t *testing.T) {
	manager := &recordingDynamicReactorIntentManager{}
	result, handled := NewFleetBoardIntentModule(manager, nil).ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{
		Intent: "registerDynamicReactor", GameSessionID: "session-1", ReactorID: "", IdempotencyKey: "register-1",
	})
	if !handled || result.Status != http.StatusBadGateway {
		t.Fatalf("expected stable invalid-intent result, got handled=%t result=%#v", handled, result)
	}
	if manager.registerCalls != 0 {
		t.Fatalf("invalid intent reached lifecycle manager %d times", manager.registerCalls)
	}
}

type recordingDynamicReactorIntentManager struct {
	registerRequest RegisterDynamicReactorRequest
	registerSet     ReactorTelemetryWorkerSet
	created         bool
	registerErr     error
	registerCalls   int
	removeRequest   RemoveDynamicReactorRequest
	removeSet       ReactorTelemetryWorkerSet
	removeErr       error
	reconcileCalls  int
	reconcileErr    error
}

func (m *recordingDynamicReactorIntentManager) RegisterDynamicReactor(_ context.Context, request RegisterDynamicReactorRequest) (ReactorTelemetryWorkerSet, bool, error) {
	m.registerCalls++
	m.registerRequest = request
	return m.registerSet, m.created, m.registerErr
}

func (m *recordingDynamicReactorIntentManager) RemoveDynamicReactor(_ context.Context, request RemoveDynamicReactorRequest) (ReactorTelemetryWorkerSet, error) {
	m.removeRequest = request
	return m.removeSet, m.removeErr
}

func (m *recordingDynamicReactorIntentManager) ReconcileExpired(context.Context) error {
	m.reconcileCalls++
	return m.reconcileErr
}

var _ DynamicReactorIntentManager = (*recordingDynamicReactorIntentManager)(nil)
