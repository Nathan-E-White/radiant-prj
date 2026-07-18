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
	module := NewFleetBoardIntentModule(manager)

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
	module := NewFleetBoardIntentModule(manager)

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
	result := NewFleetBoardIntentModule(manager).ReconcileDynamicReactors(context.Background())
	if manager.reconcileCalls != 1 || result == nil || result.Status != http.StatusBadGateway {
		t.Fatalf("expected stable reconciliation failure, calls=%d result=%#v", manager.reconcileCalls, result)
	}
	outcome, ok := result.Body.(FleetBoardIntentError)
	if !ok || outcome.Code != "reactor_telemetry_expiry_cleanup_failed" {
		t.Fatalf("unexpected reconciliation outcome: %#v", result.Body)
	}
}

func TestFleetBoardDynamicReactorIntentModuleTreatsTypedNilManagerAsUnavailable(t *testing.T) {
	var manager *ReactorTelemetryManager
	module := NewFleetBoardIntentModule(manager)
	if result := module.ReconcileDynamicReactors(context.Background()); result != nil {
		t.Fatalf("typed nil manager attempted reconciliation: %#v", result)
	}
	result, handled := module.ExecuteDynamicReactor(context.Background(), validDynamicReactorIntent("registerDynamicReactor"))
	if !handled || result.Status != http.StatusNotFound {
		t.Fatalf("typed nil manager was not classified as unavailable: handled=%t result=%#v", handled, result)
	}
}

func TestFleetBoardDynamicReactorIntentModuleRecoversRegistrationAfterManagerRestart(t *testing.T) {
	store := NewInMemoryReactorTelemetryStore()
	failedRuntime := &recordingReactorTelemetryRuntime{startErr: errors.New("runtime unavailable")}
	failedManager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, failedRuntime, nil)
	request := validDynamicReactorIntent("registerDynamicReactor")

	failed, handled := NewFleetBoardIntentModule(failedManager).ExecuteDynamicReactor(context.Background(), request)
	if !handled || failed.Status != http.StatusBadGateway {
		t.Fatalf("expected visible failed registration, handled=%t result=%#v", handled, failed)
	}
	failedSet, err := store.GetWorkerSet(request.GameSessionID, request.ReactorID)
	if err != nil || failedSet.Lifecycle != ReactorTelemetryLaunchFailed {
		t.Fatalf("failed registration was not recoverable: set=%#v err=%v", failedSet, err)
	}

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restartedManager := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, restartedRuntime, nil)
	recovered, handled := NewFleetBoardIntentModule(restartedManager).ExecuteDynamicReactor(context.Background(), request)
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
	module := NewFleetBoardIntentModule(manager)
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
	restartedModule := NewFleetBoardIntentModule(restartedManager)
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
			result, handled := NewFleetBoardIntentModule(test.manager).ExecuteDynamicReactor(context.Background(), test.request)
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
	result, handled := NewFleetBoardIntentModule(&recordingDynamicReactorIntentManager{}).ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{Intent: "requestArtifactForge"})
	if handled || result.Status != 0 || result.Body != nil {
		t.Fatalf("dynamic-reactor module absorbed a different named intent: %#v handled=%t", result, handled)
	}
}

func TestFleetBoardDynamicReactorIntentModuleRejectsInvalidIdentityBeforeManager(t *testing.T) {
	manager := &recordingDynamicReactorIntentManager{}
	result, handled := NewFleetBoardIntentModule(manager).ExecuteDynamicReactor(context.Background(), FleetBoardIntentRequest{
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
