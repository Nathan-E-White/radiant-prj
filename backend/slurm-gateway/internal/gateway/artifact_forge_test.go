package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestArtifactForgeManagerDependsOnlyOnEligibilityRead(t *testing.T) {
	eligibility := &focusedArtifactForgeEligibilityStore{}
	manager := NewArtifactForgeManager(NewInMemoryArtifactForgeStore(), nil, eligibility)
	if manager.eligibility != eligibility {
		t.Fatal("Artifact Forge did not retain the focused eligibility store")
	}
}

func TestInMemoryArtifactForgeEligibilityStoreContract(t *testing.T) {
	assertArtifactForgeEligibilityStoreContract(t, NewInMemoryWorkbenchStore())
}

type artifactForgeEligibilityContractStore interface {
	ArtifactForgeEligibilityStore
	artifactForgeResultArtifactReader
	SaveResultProjection(string, SimopsResultProjection) (bool, error)
	SaveTwinStateProjection(string, TwinStateProjection) (bool, error)
}

func assertArtifactForgeEligibilityStoreContract(t *testing.T, store artifactForgeEligibilityContractStore) {
	t.Helper()
	run := SimopsRunRecord{RunID: "run-forge-contract", ScenarioID: ArtifactForgeSchedulerDriftRecipe, Lifecycle: SimopsComplete}
	request := artifactForgeRequestFixture()
	record := ArtifactForgeRecord{RunID: run.RunID, ReactorID: request.ReactorID, GameSessionID: request.GameSessionID, SimulationRecipe: request.SimulationRecipe}
	result := artifactForgeResultFrame(run.RunID, run.ScenarioID)
	projection := SimopsResultProjection{Frame: result, RedpandaTopic: "artifact-forge-contract-results", RedpandaOffset: 1}
	written, err := store.SaveResultProjection("artifact-forge-contract", projection)
	if err != nil || !written {
		t.Fatalf("save eligible result written=%v err=%v", written, err)
	}
	seedArtifactForgeEligibilityLineage(t, store, record, request, result, 2)
	evidence, err := store.ReadArtifactForgeEligibility(run)
	if err != nil || evidence.Result == nil || evidence.ExpectedValue == nil || evidence.Lineage == nil || evidence.ExpectedValue.ValueID != WorkbenchSimulatedMarginValue || evidence.Lineage.ValueBasis != WorkbenchValueSimulated {
		t.Fatalf("eligible evidence=%#v err=%v", evidence, err)
	}
	written, err = store.SaveResultProjection("artifact-forge-contract", projection)
	if err != nil || written {
		t.Fatalf("duplicate eligible result written=%v err=%v", written, err)
	}
	afterDuplicate, err := store.ReadArtifactForgeEligibility(run)
	if err != nil || afterDuplicate.Artifact.ArtifactID != evidence.Artifact.ArtifactID || afterDuplicate.Lineage.LineageID != evidence.Lineage.LineageID {
		t.Fatalf("duplicate changed evidence=%#v err=%v", afterDuplicate, err)
	}

	for index, basis := range []WorkbenchValueBasis{WorkbenchValueMeasured, WorkbenchValueImputed} {
		ineligibleRun := SimopsRunRecord{RunID: "run-forge-contract-" + string(basis), ScenarioID: ArtifactForgeSchedulerDriftRecipe, Lifecycle: SimopsComplete}
		ineligibleFrame := artifactForgeResultFrame(ineligibleRun.RunID, ineligibleRun.ScenarioID)
		ineligibleFrame.ValueBasis = basis
		written, err = store.SaveResultProjection("artifact-forge-contract", SimopsResultProjection{Frame: ineligibleFrame, RedpandaTopic: "artifact-forge-contract-results", RedpandaOffset: int64(index + 3)})
		if err != nil || !written {
			t.Fatalf("save %s result written=%v err=%v", basis, written, err)
		}
		ineligible, err := store.ReadArtifactForgeEligibility(ineligibleRun)
		if err != nil || ineligible.Result != nil || ineligible.ExpectedValue != nil || ineligible.Lineage != nil || ineligible.Artifact.Integrity == ArtifactForgeIntegrityVerified {
			t.Fatalf("%s state escaped eligibility boundary: evidence=%#v err=%v", basis, ineligible, err)
		}
	}
}

type focusedArtifactForgeEligibilityStore struct{}

func (*focusedArtifactForgeEligibilityStore) ReadArtifactForgeEligibility(SimopsRunRecord) (ArtifactForgeEligibilityEvidence, error) {
	return ArtifactForgeEligibilityEvidence{}, nil
}

func TestArtifactForgeCreatesDistinctRunAndAppliesOneEligibleOutcome(t *testing.T) {
	forge, simops, workbench := newArtifactForgeTestRig(t)
	request := artifactForgeRequestFixture()

	accepted, created, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || !created {
		t.Fatalf("request Artifact Forge: outcome=%#v created=%v err=%v", accepted, created, err)
	}
	if accepted.RunID == "" || accepted.RunID == request.SimulationJobID || accepted.Decision != ArtifactForgeAwaitingRun || accepted.Outcome != nil {
		t.Fatalf("local Simulation Job was conflated with backend Run: %#v", accepted)
	}
	if !artifactForgeHasEvent(accepted, ArtifactForgeEventIntentAccepted) || !artifactForgeHasEvent(accepted, ArtifactForgeEventRunAssociated) {
		t.Fatalf("accepted request omitted intent or Run event boundary: %#v", accepted.Events)
	}

	if _, err := simops.UpdateRunLifecycle(accepted.RunID, SimopsComplete); err != nil {
		t.Fatalf("complete Run: %v", err)
	}
	seedEligibleArtifactForgeProjection(t, workbench, accepted, request)

	applied, replayCreated, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || replayCreated || applied.Decision != ArtifactForgeOutcomeApplied || applied.Outcome == nil {
		t.Fatalf("eligible artifact did not produce outcome: outcome=%#v created=%v err=%v", applied, replayCreated, err)
	}
	if applied.Outcome.Type != ArtifactForgeSimulatedMarginCard || applied.Outcome.Version != 1 || applied.Outcome.ReactorID != request.ReactorID {
		t.Fatalf("unexpected versioned game outcome: %#v", applied.Outcome)
	}
	if applied.Trace.RunID != accepted.RunID || applied.Trace.ArtifactID == "" || applied.Trace.LineageID == "" || !artifactForgeHasEvent(applied, ArtifactForgeEventEligibilityEvaluated) || !artifactForgeHasEvent(applied, ArtifactForgeEventOutcomeApplied) {
		t.Fatalf("success is not traceable across boundaries: %#v", applied)
	}

	replayed, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || replayed.Outcome == nil || replayed.Outcome.OutcomeID != applied.Outcome.OutcomeID || artifactForgeEventCount(replayed, ArtifactForgeEventOutcomeApplied) != 1 {
		t.Fatalf("outcome consumption was not idempotent: first=%#v replay=%#v err=%v", applied.Outcome, replayed, err)
	}
}

func TestArtifactForgeAssociationFlowsThroughResultIngestAndTwinLineage(t *testing.T) {
	forge, simops, workbenchStore := newArtifactForgeTestRig(t)
	request := artifactForgeRequestFixture()
	record, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil {
		t.Fatal(err)
	}
	workbench := NewWorkbenchController(DefaultConfig().Workbench, workbenchStore, nil)
	workbench.resultLineageContext = forge.ResultLineageContext
	run, err := simops.GetRun(record.RunID)
	if err != nil {
		t.Fatal(err)
	}
	result := artifactForgeResultFrame(record.RunID, request.SimulationRecipe)
	result.Values = append([]SimopsResultValue{{
		ResultID: "result-unrelated-01", EntityID: request.ReactorID, ValueID: "unrelated.output",
		Label: "Unrelated output", Unit: "count", Value: json.RawMessage(`{"scalar":99}`), Confidence: 0.99,
	}}, result.Values...)
	if _, status, err := workbench.IngestResults(context.Background(), run, strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{result}}))); err != nil || status != 202 {
		t.Fatalf("ingest forge result status=%d err=%v", status, err)
	}
	results, err := workbenchStore.LatestResultFrames(1)
	if err != nil || len(results) != 1 {
		t.Fatalf("read enriched result: results=%#v err=%v", results, err)
	}
	state, lineage := BuildTwinStateFromData(nil, results[0], time.Now().UTC())
	if _, err := workbenchStore.SaveTwinStateProjection("artifact-forge-projector", TwinStateProjection{State: state, Lineage: lineage, LineagePresent: true, RedpandaTopic: "digital-twin.state.v1", RedpandaOffset: 2}); err != nil {
		t.Fatalf("project enriched Twin Lineage: %v", err)
	}
	if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
		t.Fatal(err)
	}

	applied, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || applied.Outcome == nil || applied.Decision != ArtifactForgeOutcomeApplied {
		t.Fatalf("server-side association did not reach outcome: record=%#v err=%v", applied, err)
	}
	if applied.Outcome.ValueID != WorkbenchSimulatedMarginValue {
		t.Fatalf("outcome selected the wrong reordered result value: %#v", applied.Outcome)
	}
}

func TestArtifactForgeEligibilityReadFailureRecoversWithoutParallelOutcome(t *testing.T) {
	forge, simops, workbench := newArtifactForgeTestRig(t)
	request := artifactForgeRequestFixture()
	record, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
		t.Fatal(err)
	}
	seedEligibleArtifactForgeProjection(t, workbench, record, request)
	forge.eligibility = failingArtifactForgeEligibilityStore{err: errors.New("eligibility read unavailable")}
	if _, _, err := forge.Request(context.Background(), request, "fleet-board-client"); err == nil {
		t.Fatal("expected eligibility read failure")
	}
	pending, err := forge.store.Find(request.GameSessionID, request.IdempotencyKey)
	if err != nil || pending.Outcome != nil || pending.Decision != ArtifactForgeAwaitingRun {
		t.Fatalf("eligibility failure consumed outcome: record=%#v err=%v", pending, err)
	}
	forge.eligibility = workbench
	recovered, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || recovered.Outcome == nil || recovered.Decision != ArtifactForgeOutcomeApplied || artifactForgeEventCount(recovered, ArtifactForgeEventOutcomeApplied) != 1 {
		t.Fatalf("eligibility recovery outcome=%#v err=%v", recovered, err)
	}
}

func TestArtifactForgeIneligiblePathsProduceVisibleNoOutcome(t *testing.T) {
	tests := []struct {
		name     string
		arrange  func(*testing.T, *ArtifactForgeManager, *InMemorySimopsStore, *InMemoryWorkbenchStore, ArtifactForgeRecord, ArtifactForgeRequest)
		decision ArtifactForgeDecision
	}{
		{
			name: "failed Run",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, _ *InMemoryWorkbenchStore, record ArtifactForgeRecord, _ ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsFailed); err != nil {
					t.Fatal(err)
				}
			},
			decision: ArtifactForgeRunFailed,
		},
		{
			name: "operational telemetry only",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, _ *InMemoryWorkbenchStore, record ArtifactForgeRecord, _ ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				for _, artifact := range mustArtifacts(t, simops, record.RunID) {
					if err := simops.UpdateArtifactStatus(record.RunID, artifact.ArtifactID, SimopsArtifactStatusCommitted); err != nil {
						t.Fatal(err)
					}
				}
			},
			decision: ArtifactForgeTelemetryIneligible,
		},
		{
			name: "incomplete result artifact",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				seedArtifactForgeResultOnly(t, workbench, record.RunID, request.SimulationRecipe)
				setArtifactForgeTestArtifact(workbench, record.RunID, func(artifact *ArtifactForgeResultArtifact) { artifact.Complete = false })
			},
			decision: ArtifactForgeArtifactIncomplete,
		},
		{
			name: "missing Lineage",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				seedArtifactForgeResultOnly(t, workbench, record.RunID, request.SimulationRecipe)
			},
			decision: ArtifactForgeLineageMissing,
		},
		{
			name: "artifact schema not allowlisted",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				seedArtifactForgeResultOnly(t, workbench, record.RunID, request.SimulationRecipe)
				setArtifactForgeTestArtifact(workbench, record.RunID, func(artifact *ArtifactForgeResultArtifact) { artifact.SchemaVersion = "simops.result.v2" })
			},
			decision: ArtifactForgeArtifactIneligible,
		},
		{
			name: "artifact integrity rejected",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				seedArtifactForgeResultOnly(t, workbench, record.RunID, request.SimulationRecipe)
				setArtifactForgeTestArtifact(workbench, record.RunID, func(artifact *ArtifactForgeResultArtifact) { artifact.Integrity = "rejected" })
			},
			decision: ArtifactForgeIntegrityFailed,
		},
		{
			name: "imputed result cannot reward",
			arrange: func(t *testing.T, _ *ArtifactForgeManager, simops *InMemorySimopsStore, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
				t.Helper()
				if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
					t.Fatal(err)
				}
				result := artifactForgeResultFrame(record.RunID, request.SimulationRecipe)
				result.ValueBasis = WorkbenchValueImputed
				if _, err := workbench.SaveResultProjection("artifact-forge-test", SimopsResultProjection{Frame: result, RedpandaTopic: "simops.results.v1", RedpandaOffset: 1}); err != nil {
					t.Fatal(err)
				}
			},
			decision: ArtifactForgeIntegrityFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			forge, simops, workbench := newArtifactForgeTestRig(t)
			request := artifactForgeRequestFixture()
			record, _, err := forge.Request(context.Background(), request, "fleet-board-client")
			if err != nil {
				t.Fatalf("initial request: %v", err)
			}
			test.arrange(t, forge, simops, workbench, record, request)

			outcome, _, err := forge.Request(context.Background(), request, "fleet-board-client")
			if err != nil || outcome.Decision != test.decision || outcome.Outcome != nil || outcome.Message == "" {
				t.Fatalf("ineligible path was not explicit: outcome=%#v err=%v", outcome, err)
			}
			if !artifactForgeHasEvent(outcome, ArtifactForgeEventEligibilityEvaluated) {
				t.Fatalf("eligibility decision missing from event trace: %#v", outcome.Events)
			}
		})
	}
}

func TestArtifactForgeRequiresEveryAssociationAndArtifactLineageLink(t *testing.T) {
	tests := []struct {
		name       string
		removeKind string
		artifact   bool
	}{
		{name: "game session", removeKind: "game-session"},
		{name: "reactor", removeKind: "fleet-reactor"},
		{name: "Run", removeKind: "simulation-run"},
		{name: "recipe", removeKind: "simulation-recipe"},
		{name: "artifact", artifact: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			forge, simops, workbench := newArtifactForgeTestRig(t)
			request := artifactForgeRequestFixture()
			record, _, err := forge.Request(context.Background(), request, "fleet-board-client")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := simops.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
				t.Fatal(err)
			}
			seedEligibleArtifactForgeProjection(t, workbench, record, request)
			lineage, err := workbench.LineageForValue(WorkbenchSimulatedMarginValue)
			if err != nil {
				t.Fatal(err)
			}
			if test.artifact {
				lineage.Artifacts = nil
			} else {
				inputs := lineage.Inputs[:0]
				for _, input := range lineage.Inputs {
					if input.SourceKind != test.removeKind {
						inputs = append(inputs, input)
					}
				}
				lineage.Inputs = inputs
			}
			snapshot, err := workbench.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := workbench.SaveTwinStateProjection("artifact-forge-invalid-lineage", TwinStateProjection{State: snapshot.Twin, Lineage: []DigitalTwinValueLineage{lineage}, LineagePresent: true, RedpandaTopic: "digital-twin.state.v1", RedpandaOffset: 3}); err != nil {
				t.Fatal(err)
			}

			outcome, _, err := forge.Request(context.Background(), request, "fleet-board-client")
			if err != nil || outcome.Decision != ArtifactForgeLineageIneligible || outcome.Outcome != nil {
				t.Fatalf("missing %s Lineage link produced outcome=%#v err=%v", test.name, outcome, err)
			}
		})
	}
}

func TestArtifactForgeIdempotencyKeyCannotChangeDomainAssociation(t *testing.T) {
	forge, simops, _ := newArtifactForgeTestRig(t)
	request := artifactForgeRequestFixture()
	if _, _, err := forge.Request(context.Background(), request, "fleet-board-client"); err != nil {
		t.Fatal(err)
	}
	request.SimulationJobID = "different-local-job"
	if _, _, err := forge.Request(context.Background(), request, "fleet-board-client"); err == nil {
		t.Fatal("idempotency key changed its local Simulation Job association")
	}
	if len(simops.runs) != 1 {
		t.Fatalf("idempotency conflict created parallel Runs: %#v", simops.runs)
	}
}

func TestArtifactForgeRecoversBothMissingRunAssociationWindows(t *testing.T) {
	t.Run("accepted request before Run creation", func(t *testing.T) {
		forge, simops, _ := newArtifactForgeTestRig(t)
		request := artifactForgeRequestFixture()
		now := time.Now().UTC()
		record := ArtifactForgeRecord{
			RequestID: artifactForgeStableID("forge", request.GameSessionID, request.IdempotencyKey), GameSessionID: request.GameSessionID,
			ReactorID: request.ReactorID, SimulationJobID: request.SimulationJobID, SimulationRecipe: request.SimulationRecipe,
			IdempotencyKey: request.IdempotencyKey, SubmittedBy: "fleet-board-client", Decision: ArtifactForgeRunLaunchRetryable,
			Trace: ArtifactForgeTrace{SimulationJobID: request.SimulationJobID}, CreatedAt: now, UpdatedAt: now,
		}
		if err := forge.store.Save(record); err != nil {
			t.Fatal(err)
		}
		recovered, created, err := forge.Request(context.Background(), request, "fleet-board-client")
		if err != nil || created || recovered.RunID == "" || len(simops.runs) != 1 {
			t.Fatalf("missing pre-launch association did not recover: record=%#v created=%v runs=%d err=%v", recovered, created, len(simops.runs), err)
		}
	})

	t.Run("Run created before association save", func(t *testing.T) {
		cfg := DefaultConfig().Simops
		simopsStore := NewInMemorySimopsStore()
		controller := NewSimopsController(cfg, simopsStore, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: simopsStore}, IcebergArtifactPlanner{}, nil, nil)
		controller.runID = func() string { return "forge-run-save-window" }
		base := NewInMemoryArtifactForgeStore()
		store := &failArtifactForgeAssociationSaveOnce{ArtifactForgeStore: base}
		forge := NewArtifactForgeManager(store, controller, NewInMemoryWorkbenchStore())
		request := artifactForgeRequestFixture()
		if _, _, err := forge.Request(context.Background(), request, "fleet-board-client"); err == nil {
			t.Fatal("injected association save failure was not observed")
		}
		if len(simopsStore.runs) != 1 {
			t.Fatalf("Run was not created before injected save failure: %#v", simopsStore.runs)
		}
		recovered, created, err := forge.Request(context.Background(), request, "fleet-board-client")
		if err != nil || created || recovered.RunID != "forge-run-save-window" || len(simopsStore.runs) != 1 {
			t.Fatalf("association save window created parallel lifecycle: record=%#v created=%v runs=%d err=%v", recovered, created, len(simopsStore.runs), err)
		}
	})
}

func TestArtifactForgeExpiresSessionThenRetainsLedgerForSevenDays(t *testing.T) {
	forge, _, _ := newArtifactForgeTestRig(t)
	request := artifactForgeRequestFixture()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	forge.now = func() time.Time { return now }
	record, _, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || !record.SessionExpiresAt.Equal(now.Add(24*time.Hour)) || !record.RetainUntil.Equal(now.Add(8*24*time.Hour)) {
		t.Fatalf("retention metadata=%#v err=%v", record, err)
	}
	now = now.Add(7 * 24 * time.Hour)
	if removed, err := forge.ReconcileExpired(); err != nil || removed != 0 {
		t.Fatalf("ledger pruned before seven-day post-expiry retention: removed=%d err=%v", removed, err)
	}
	now = now.Add(24 * time.Hour)
	if removed, err := forge.ReconcileExpired(); err != nil || removed != 1 {
		t.Fatalf("expired retained ledger was not bounded: removed=%d err=%v", removed, err)
	}
	if _, err := forge.store.Find(request.GameSessionID, request.IdempotencyKey); !errors.Is(err, ErrArtifactForgeNotFound) {
		t.Fatalf("expired ledger still present: %v", err)
	}
}

type failArtifactForgeAssociationSaveOnce struct {
	ArtifactForgeStore
	failed bool
}

type failingArtifactForgeEligibilityStore struct {
	err error
}

func (s failingArtifactForgeEligibilityStore) ReadArtifactForgeEligibility(SimopsRunRecord) (ArtifactForgeEligibilityEvidence, error) {
	return ArtifactForgeEligibilityEvidence{}, s.err
}

func (s *failArtifactForgeAssociationSaveOnce) Save(record ArtifactForgeRecord) error {
	if record.RunID != "" && !s.failed {
		s.failed = true
		return errors.New("injected association persistence failure")
	}
	return s.ArtifactForgeStore.Save(record)
}

func newArtifactForgeTestRig(t *testing.T) (*ArtifactForgeManager, *InMemorySimopsStore, *InMemoryWorkbenchStore) {
	t.Helper()
	cfg := DefaultConfig().Simops
	simopsStore := NewInMemorySimopsStore()
	controller := NewSimopsController(cfg, simopsStore, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: simopsStore}, IcebergArtifactPlanner{}, nil, nil)
	sequence := 0
	controller.runID = func() string {
		sequence++
		return "forge-run-" + string(rune('0'+sequence))
	}
	workbenchStore := NewInMemoryWorkbenchStore()
	return NewArtifactForgeManager(NewInMemoryArtifactForgeStore(), controller, workbenchStore), simopsStore, workbenchStore
}

func artifactForgeRequestFixture() ArtifactForgeRequest {
	return ArtifactForgeRequest{
		GameSessionID:      "game-session-01",
		ReactorID:          "reactor-01",
		SimulationJobID:    "local-job-01",
		SimulationJobState: "completed",
		SimulationRecipe:   ArtifactForgeSchedulerDriftRecipe,
		IdempotencyKey:     "forge-click-01",
	}
}

func seedEligibleArtifactForgeProjection(t *testing.T, workbench *InMemoryWorkbenchStore, record ArtifactForgeRecord, request ArtifactForgeRequest) {
	t.Helper()
	result := artifactForgeResultFrame(record.RunID, request.SimulationRecipe)
	if _, err := workbench.SaveResultProjection("artifact-forge-test", SimopsResultProjection{Frame: result, RedpandaTopic: "simops.results.v1", RedpandaOffset: 1}); err != nil {
		t.Fatalf("save simulated result: %v", err)
	}
	seedArtifactForgeEligibilityLineage(t, workbench, record, request, result, 2)
}

func seedArtifactForgeEligibilityLineage(t *testing.T, store artifactForgeEligibilityContractStore, record ArtifactForgeRecord, request ArtifactForgeRequest, result SimopsResultFrame, offset int64) {
	t.Helper()
	artifact, err := store.ArtifactForgeResultArtifact(record.RunID)
	if err != nil {
		t.Fatalf("read durable result artifact: %v", err)
	}
	lineage := DigitalTwinValueLineage{
		SchemaVersion: WorkbenchLineageSchemaVersion,
		LineageID:     "lineage-forge-01",
		ValueID:       result.Values[0].ValueID,
		ValueBasis:    WorkbenchValueSimulated,
		Inputs: []TwinLineageInput{
			{SourceKind: "game-session", SourceID: request.GameSessionID, ValueBasis: WorkbenchValueSimulated},
			{SourceKind: "fleet-reactor", SourceID: request.ReactorID, ValueBasis: WorkbenchValueSimulated},
			{SourceKind: "simulation-run", SourceID: record.RunID, ValueBasis: WorkbenchValueSimulated},
			{SourceKind: "simulation-recipe", SourceID: request.SimulationRecipe, ValueBasis: WorkbenchValueSimulated},
		},
		ProcessingSteps: []string{"Project public-safe Simulated Result State for Artifact Forge eligibility"},
		Artifacts:       []TwinLineageArtifact{{ArtifactID: artifact.ArtifactID, Path: "simops://results/run_id=" + record.RunID, MediaType: artifact.MediaType}},
	}
	state := DigitalTwinState{SchemaVersion: WorkbenchTwinStateSchemaVersion, TwinID: WorkbenchDefaultTwinID, AsOf: time.Now().UTC(), Entities: []DigitalTwinEntity{{EntityID: request.ReactorID, DisplayName: "Reactor", Values: []DigitalTwinValue{{ValueID: result.Values[0].ValueID, Label: result.Values[0].Label, ValueBasis: WorkbenchValueSimulated, Unit: result.Values[0].Unit, Value: map[string]any{"scalar": 16.1}, Confidence: 0.71, LineageID: lineage.LineageID, SourceIDs: []string{record.RunID}}}}}}
	if _, err := store.SaveTwinStateProjection("artifact-forge-test", TwinStateProjection{State: state, Lineage: []DigitalTwinValueLineage{lineage}, LineagePresent: true, RedpandaTopic: "digital-twin.state.v1", RedpandaOffset: offset}); err != nil {
		t.Fatalf("save Twin State and Lineage: %v", err)
	}
}

func seedArtifactForgeResultOnly(t *testing.T, workbench *InMemoryWorkbenchStore, runID, recipe string) {
	t.Helper()
	if _, err := workbench.SaveResultProjection("artifact-forge-test", SimopsResultProjection{Frame: artifactForgeResultFrame(runID, recipe), RedpandaTopic: "simops.results.v1", RedpandaOffset: 1}); err != nil {
		t.Fatalf("save simulated result: %v", err)
	}
}

func artifactForgeResultFrame(runID, recipe string) SimopsResultFrame {
	return SimopsResultFrame{SchemaVersion: WorkbenchResultSchemaVersion, RunID: runID, ScenarioID: recipe, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, Sequence: 1, ProducedAt: "2026-07-14T10:00:00Z", ResultType: "syntheticEngineeringState", ModelID: "MODEL-FORGE-V1", InputWindow: SimopsResultInputWindow{Start: "2026-07-14T09:59:00Z", End: "2026-07-14T10:00:00Z"}, ValueBasis: WorkbenchValueSimulated, SyntheticStatus: WorkbenchSyntheticPublicStandin, Values: []SimopsResultValue{{ResultID: "result-forge-01", EntityID: "reactor-01", ValueID: WorkbenchSimulatedMarginValue, Label: "Simulated forecast margin", Unit: "percent", Value: json.RawMessage(`{"scalar":16.1}`), Confidence: 0.71}}}
}

func setArtifactForgeTestArtifact(store *InMemoryWorkbenchStore, runID string, mutate func(*ArtifactForgeResultArtifact)) {
	store.mu.Lock()
	defer store.mu.Unlock()
	artifact := store.forgeArtifacts[runID]
	mutate(&artifact)
	store.forgeArtifacts[runID] = artifact
}

func mustArtifacts(t *testing.T, store *InMemorySimopsStore, runID string) []SimopsArtifactRecord {
	t.Helper()
	artifacts, err := store.ListArtifacts(runID)
	if err != nil {
		t.Fatal(err)
	}
	return artifacts
}

func artifactForgeHasEvent(record ArtifactForgeRecord, eventType ArtifactForgeEventType) bool {
	return artifactForgeEventCount(record, eventType) > 0
}

func artifactForgeEventCount(record ArtifactForgeRecord, eventType ArtifactForgeEventType) int {
	count := 0
	for _, event := range record.Events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}
