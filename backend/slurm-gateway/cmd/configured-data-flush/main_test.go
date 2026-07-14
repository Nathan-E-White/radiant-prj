package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"radiant/slurm-gateway/internal/gateway"
)

func TestExecuteDefaultsToReadableDryRun(t *testing.T) {
	service := &recordingFlushExecutor{plan: gateway.ConfiguredDataFlushPlan{Mode: "dry-run", PlanID: "cdf-reviewed", Ready: true}}
	var output bytes.Buffer
	if err := execute(context.Background(), service, "", &output); err != nil {
		t.Fatalf("execute dry-run: %v", err)
	}
	if service.applyPlan != "" || !strings.Contains(output.String(), `"mode": "dry-run"`) || !strings.Contains(output.String(), `"planId": "cdf-reviewed"`) {
		t.Fatalf("default command was not a readable non-mutating plan: %s", output.String())
	}
}

func TestExecuteAppliesOnlyExplicitPlanID(t *testing.T) {
	service := &recordingFlushExecutor{result: gateway.ConfiguredDataFlushResult{PlanID: "cdf-reviewed", PreviousGeneration: 3, Generation: 4}}
	var output bytes.Buffer
	if err := execute(context.Background(), service, "cdf-reviewed", &output); err != nil {
		t.Fatalf("execute apply: %v", err)
	}
	if service.applyPlan != "cdf-reviewed" || !strings.Contains(output.String(), `"generation": 4`) {
		t.Fatalf("explicit reviewed plan was not applied: service=%#v output=%s", service, output.String())
	}
}

type recordingFlushExecutor struct {
	plan      gateway.ConfiguredDataFlushPlan
	result    gateway.ConfiguredDataFlushResult
	applyPlan string
}

func (r *recordingFlushExecutor) Plan(context.Context) (gateway.ConfiguredDataFlushPlan, error) {
	return r.plan, nil
}

func (r *recordingFlushExecutor) Apply(_ context.Context, planID string) (gateway.ConfiguredDataFlushResult, error) {
	r.applyPlan = planID
	return r.result, nil
}
