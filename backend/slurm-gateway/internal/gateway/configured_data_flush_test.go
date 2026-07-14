package gateway

import (
	"context"
	"errors"
	"testing"
)

func TestConfiguredDataFlushPlanIsReadableCompleteAndNonMutating(t *testing.T) {
	repository := &recordingConfiguredDataFlushRepository{inventory: ConfiguredDataFlushInventory{
		Generation: 7,
		Rows: map[string]int64{
			"artifact_forge_requests":               2,
			"artifact_forge_result_artifacts":       2,
			"simops_events":                         3,
			"simops_telemetry_frames":               12,
			"scada_measured_frames":                 8,
			"simops_result_values":                  4,
			"digital_twin_state_values":             5,
			"digital_twin_lineage":                  5,
			"workbench_twin_publications":           2,
			"reactor_telemetry_worker_sets.removed": 2,
		},
	}}
	service := NewConfiguredDataFlushService(repository)

	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan configured data flush: %v", err)
	}
	if !plan.Ready || plan.CurrentGeneration != 7 || plan.NextGeneration != 8 || plan.PlanID == "" {
		t.Fatalf("plan lost generation or readiness: %#v", plan)
	}
	if len(plan.Targets) != 10 {
		t.Fatalf("plan omitted a runtime record class: %#v", plan.Targets)
	}
	if len(plan.ProtectedResources) != 9 {
		t.Fatalf("plan omitted a protected resource class: %#v", plan.ProtectedResources)
	}
	if repository.applyCalls != 0 {
		t.Fatal("dry-run plan mutated the repository")
	}

	repeated, err := service.Plan(context.Background())
	if err != nil || repeated.PlanID != plan.PlanID {
		t.Fatalf("unchanged inventory did not produce a stable plan: %#v err=%v", repeated, err)
	}
}

func TestConfiguredDataFlushPlanBlocksActiveRuntimeResources(t *testing.T) {
	repository := &recordingConfiguredDataFlushRepository{inventory: ConfiguredDataFlushInventory{
		Generation:         2,
		ActiveRunIDs:       []string{"run-active"},
		ActiveWorkerSetIDs: []string{"set-active"},
		Rows:               map[string]int64{},
	}}
	plan, err := NewConfiguredDataFlushService(repository).Plan(context.Background())
	if err != nil {
		t.Fatalf("plan blocked flush: %v", err)
	}
	if plan.Ready || len(plan.Blockers) != 2 {
		t.Fatalf("active resources did not block mutation: %#v", plan)
	}
	if _, err := NewConfiguredDataFlushService(repository).Apply(context.Background(), plan.PlanID); !errors.Is(err, ErrConfiguredDataFlushBlocked) {
		t.Fatalf("blocked plan was applied: %v", err)
	}
}

func TestConfiguredDataFlushApplyRejectsStalePlanAndReturnsNewGeneration(t *testing.T) {
	repository := &recordingConfiguredDataFlushRepository{inventory: ConfiguredDataFlushInventory{
		Generation: 4,
		Rows:       map[string]int64{"scada_measured_frames": 6},
	}}
	service := NewConfiguredDataFlushService(repository)
	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := service.Apply(context.Background(), "not-the-reviewed-plan"); !errors.Is(err, ErrConfiguredDataFlushStalePlan) {
		t.Fatalf("stale plan was not rejected: %v", err)
	}
	result, err := service.Apply(context.Background(), plan.PlanID)
	if err != nil {
		t.Fatalf("apply reviewed plan: %v", err)
	}
	if result.PreviousGeneration != 4 || result.Generation != 5 || repository.applyCalls != 1 {
		t.Fatalf("flush did not create one new generation: result=%#v calls=%d", result, repository.applyCalls)
	}
}

func TestConfiguredDataFlushPlanChangesWhenDatabaseRevisionChangesAtSameCounts(t *testing.T) {
	repository := &recordingConfiguredDataFlushRepository{inventory: ConfiguredDataFlushInventory{
		Generation: 9, Revision: "100", Rows: map[string]int64{"scada_measured_frames": 2},
	}}
	service := NewConfiguredDataFlushService(repository)
	before, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan before replacement: %v", err)
	}
	repository.inventory.Revision = "101"
	after, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan after replacement: %v", err)
	}
	if before.PlanID == after.PlanID {
		t.Fatal("same-count replacement did not invalidate the reviewed plan")
	}
}

type recordingConfiguredDataFlushRepository struct {
	inventory  ConfiguredDataFlushInventory
	applyCalls int
}

func (r *recordingConfiguredDataFlushRepository) Inspect(context.Context) (ConfiguredDataFlushInventory, error) {
	return r.inventory, nil
}

func (r *recordingConfiguredDataFlushRepository) Apply(_ context.Context, plan ConfiguredDataFlushPlan) (ConfiguredDataFlushResult, error) {
	r.applyCalls++
	previous := r.inventory.Generation
	r.inventory.Generation++
	r.inventory.Rows = map[string]int64{}
	return ConfiguredDataFlushResult{
		PlanID: plan.PlanID, PreviousGeneration: previous, Generation: r.inventory.Generation,
	}, nil
}
