package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var (
	ErrConfiguredDataFlushBlocked   = errors.New("configured data flush is blocked by active runtime resources")
	ErrConfiguredDataFlushStalePlan = errors.New("configured data flush plan no longer matches current data")
)

type ConfiguredDataFlushInventory struct {
	Generation         uint64
	Revision           string
	Rows               map[string]int64
	ActiveRunIDs       []string
	ActiveWorkerSetIDs []string
}

type ConfiguredDataFlushTarget struct {
	Name        string `json:"name"`
	RecordClass string `json:"recordClass"`
	Rows        int64  `json:"rows"`
	Selection   string `json:"selection"`
}

type ConfiguredDataFlushProtectedResource struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ConfiguredDataFlushPlan struct {
	Operation          string                                 `json:"operation"`
	Mode               string                                 `json:"mode"`
	PlanID             string                                 `json:"planId"`
	Ready              bool                                   `json:"ready"`
	CurrentGeneration  uint64                                 `json:"currentGeneration"`
	NextGeneration     uint64                                 `json:"nextGeneration"`
	InventoryRevision  string                                 `json:"inventoryRevision"`
	Targets            []ConfiguredDataFlushTarget            `json:"targets"`
	ProtectedResources []ConfiguredDataFlushProtectedResource `json:"protectedResources"`
	Blockers           []string                               `json:"blockers"`
}

type ConfiguredDataFlushResult struct {
	Operation          string           `json:"operation"`
	PlanID             string           `json:"planId"`
	PreviousGeneration uint64           `json:"previousGeneration"`
	Generation         uint64           `json:"generation"`
	DeletedRows        map[string]int64 `json:"deletedRows"`
}

type ConfiguredDataFlushRepository interface {
	Inspect(context.Context) (ConfiguredDataFlushInventory, error)
	Apply(context.Context, ConfiguredDataFlushPlan) (ConfiguredDataFlushResult, error)
}

type ConfiguredDataFlushService struct {
	repository ConfiguredDataFlushRepository
}

func NewConfiguredDataFlushService(repository ConfiguredDataFlushRepository) *ConfiguredDataFlushService {
	return &ConfiguredDataFlushService{repository: repository}
}

func (s *ConfiguredDataFlushService) Plan(ctx context.Context) (ConfiguredDataFlushPlan, error) {
	if s == nil || s.repository == nil {
		return ConfiguredDataFlushPlan{}, fmt.Errorf("configured data flush repository is required")
	}
	inventory, err := s.repository.Inspect(ctx)
	if err != nil {
		return ConfiguredDataFlushPlan{}, err
	}
	return buildConfiguredDataFlushPlan(inventory), nil
}

func (s *ConfiguredDataFlushService) Apply(ctx context.Context, reviewedPlanID string) (ConfiguredDataFlushResult, error) {
	plan, err := s.Plan(ctx)
	if err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	if !plan.Ready {
		return ConfiguredDataFlushResult{}, fmt.Errorf("%w: %v", ErrConfiguredDataFlushBlocked, plan.Blockers)
	}
	if reviewedPlanID == "" || reviewedPlanID != plan.PlanID {
		return ConfiguredDataFlushResult{}, ErrConfiguredDataFlushStalePlan
	}
	return s.repository.Apply(ctx, plan)
}

func buildConfiguredDataFlushPlan(inventory ConfiguredDataFlushInventory) ConfiguredDataFlushPlan {
	targets := make([]ConfiguredDataFlushTarget, 0, len(configuredDataFlushTargetRegistry))
	for _, definition := range configuredDataFlushTargetRegistry {
		targets = append(targets, ConfiguredDataFlushTarget{
			Name: definition.Name, RecordClass: definition.RecordClass,
			Rows: inventory.Rows[definition.Name], Selection: definition.Selection,
		})
	}
	blockers := make([]string, 0, len(inventory.ActiveRunIDs)+len(inventory.ActiveWorkerSetIDs))
	sort.Strings(inventory.ActiveRunIDs)
	sort.Strings(inventory.ActiveWorkerSetIDs)
	for _, runID := range inventory.ActiveRunIDs {
		blockers = append(blockers, "active SimOps Run: "+runID)
	}
	for _, setID := range inventory.ActiveWorkerSetIDs {
		blockers = append(blockers, "active Reactor Telemetry Worker Set: "+setID)
	}
	plan := ConfiguredDataFlushPlan{
		Operation: "configured-data-flush", Mode: "dry-run", Ready: len(blockers) == 0,
		CurrentGeneration: inventory.Generation, NextGeneration: inventory.Generation + 1,
		InventoryRevision: inventory.Revision,
		Targets:           targets, ProtectedResources: configuredDataFlushProtectedResources(), Blockers: blockers,
	}
	fingerprint, _ := json.Marshal(struct {
		Generation uint64                      `json:"generation"`
		Revision   string                      `json:"revision"`
		Targets    []ConfiguredDataFlushTarget `json:"targets"`
		Blockers   []string                    `json:"blockers"`
	}{Generation: plan.CurrentGeneration, Revision: plan.InventoryRevision, Targets: plan.Targets, Blockers: plan.Blockers})
	sum := sha256.Sum256(fingerprint)
	plan.PlanID = "cdf-" + hex.EncodeToString(sum[:12])
	return plan
}

type configuredDataFlushTargetDefinition struct {
	Name        string
	RecordClass string
	Selection   string
	CountSQL    string
	DeleteSQL   string
}

var configuredDataFlushTargetRegistry = []configuredDataFlushTargetDefinition{
	{Name: "artifact_forge_requests", RecordClass: "Fleet Board game intent and outcome records", Selection: "all local-demo Artifact Forge ledger records", CountSQL: `SELECT count(*) FROM artifact_forge_requests`, DeleteSQL: `DELETE FROM artifact_forge_requests`},
	{Name: "artifact_forge_result_artifacts", RecordClass: "Artifact Forge simulated-result eligibility records", Selection: "all durably projected local-demo result artifacts", CountSQL: `SELECT count(*) FROM artifact_forge_result_artifacts`, DeleteSQL: `DELETE FROM artifact_forge_result_artifacts`},
	{Name: "simops_events", RecordClass: "SimOps event records", Selection: "all accepted local-demo records", CountSQL: `SELECT count(*) FROM simops_events`, DeleteSQL: `DELETE FROM simops_events`},
	{Name: "simops_telemetry_frames", RecordClass: "operational telemetry", Selection: "all accepted local-demo records", CountSQL: `SELECT count(*) FROM simops_telemetry_frames`, DeleteSQL: `DELETE FROM simops_telemetry_frames`},
	{Name: "scada_measured_frames", RecordClass: "Measured State projections", Selection: "all frames; Resident Source declarations remain protected", CountSQL: `SELECT count(*) FROM scada_measured_frames`, DeleteSQL: `DELETE FROM scada_measured_frames`},
	{Name: "simops_result_values", RecordClass: "Simulated Result State projections", Selection: "all accepted local-demo records", CountSQL: `SELECT count(*) FROM simops_result_values`, DeleteSQL: `DELETE FROM simops_result_values`},
	{Name: "digital_twin_state_values", RecordClass: "Twin State projections", Selection: "all accepted local-demo records", CountSQL: `SELECT count(*) FROM digital_twin_state_values`, DeleteSQL: `DELETE FROM digital_twin_state_values`},
	{Name: "digital_twin_lineage", RecordClass: "Lineage projections", Selection: "all accepted local-demo records", CountSQL: `SELECT count(*) FROM digital_twin_lineage`, DeleteSQL: `DELETE FROM digital_twin_lineage`},
	{Name: "workbench_twin_publications", RecordClass: "Twin publication recovery records", Selection: "all pre-flush publication recovery records", CountSQL: `SELECT count(*) FROM workbench_twin_publications`, DeleteSQL: `DELETE FROM workbench_twin_publications`},
	{Name: "reactor_telemetry_worker_sets.removed", RecordClass: "Reactor Telemetry control records", Selection: "removed worker sets only", CountSQL: `SELECT count(*) FROM reactor_telemetry_worker_sets WHERE worker_set->>'lifecycle' = 'removed'`, DeleteSQL: `DELETE FROM reactor_telemetry_worker_sets WHERE worker_set->>'lifecycle' = 'removed'`},
}

func configuredDataFlushProtectedResources() []ConfiguredDataFlushProtectedResource {
	return []ConfiguredDataFlushProtectedResource{
		{Name: "postgres-schemas", Reason: "table, hypertable, index, and constraint definitions are platform configuration"},
		{Name: "workbench-resident-sources", Reason: "Resident Source declarations are protected configuration"},
		{Name: "workbench-resident-tags", Reason: "Resident Source tag declarations are protected configuration"},
		{Name: "simops-run-control-records", Reason: "Runs, workers, spool commands, artifacts, and ingest credentials retain their lifecycle policy"},
		{Name: "consumer-recovery-cursors", Reason: "processed-message and consumer-offset records prevent retained topic data from replaying"},
		{Name: "ingest-credentials-and-configuration", Reason: "credentials and credential configuration are never flush targets"},
		{Name: "required-redpanda-topics", Reason: "topic existence and retained broker records are preserved"},
		{Name: "iceberg-catalog-and-protected-volumes", Reason: "catalog metadata, object storage, and protected volumes are not mutated"},
		{Name: "compose-wiring-and-platform-configuration", Reason: "the environment is reused without teardown or reprovisioning"},
	}
}
