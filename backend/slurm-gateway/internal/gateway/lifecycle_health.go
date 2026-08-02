package gateway

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var lifecycleTaskNames = []string{"reactor_expiry", "artifact_forge_expiry", "measured_retention"}

type LifecycleTaskPolicy struct {
	Required      bool
	MaxSuccessAge time.Duration
}

var lifecycleTaskPolicy = map[string]LifecycleTaskPolicy{
	"reactor_expiry":        {Required: true, MaxSuccessAge: 2 * time.Minute},
	"artifact_forge_expiry": {Required: true, MaxSuccessAge: 2 * time.Minute},
	"measured_retention":    {Required: true, MaxSuccessAge: 2 * time.Minute},
}

type LifecycleHealthState string

const (
	LifecycleDisabled LifecycleHealthState = "disabled"
	LifecycleStarting LifecycleHealthState = "starting"
	LifecycleReady    LifecycleHealthState = "ready"
	LifecycleDegraded LifecycleHealthState = "degraded"
	LifecycleNotReady LifecycleHealthState = "not_ready"
)

// LifecycleHealth is the gateway's readiness-facing record of scheduled work.
// It deliberately does not control process liveness.
type LifecycleHealth struct {
	mu          sync.RWMutex
	enabled     bool
	state       LifecycleHealthState
	lastRun     time.Time
	lastError   string
	lastSuccess time.Time
	tasks       map[string]error
	taskSuccess map[string]time.Time
}

func NewLifecycleHealth(enabled bool) *LifecycleHealth {
	state := LifecycleDisabled
	if enabled {
		state = LifecycleStarting
	}
	return &LifecycleHealth{enabled: enabled, state: state, tasks: map[string]error{}, taskSuccess: map[string]time.Time{}}
}

func (h *LifecycleHealth) RecordOutcomes(now time.Time, outcomes []LifecycleTaskOutcome) {
	if h == nil || !h.enabled {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastRun, h.tasks = now.UTC(), map[string]error{}
	failed := false
	for _, outcome := range outcomes {
		h.tasks[outcome.Name] = outcome.Err
		if outcome.Err == nil {
			h.taskSuccess[outcome.Name] = now.UTC()
		}
		failed = failed || outcome.Err != nil
	}
	if failed {
		if h.lastSuccess.IsZero() {
			h.state = LifecycleNotReady
		} else {
			h.state = LifecycleDegraded
		}
		return
	}
	h.state, h.lastError, h.lastSuccess = LifecycleReady, "", now.UTC()
}

func (h *LifecycleHealth) Prometheus() string {
	if h == nil {
		return ""
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	var b strings.Builder
	b.WriteString("# HELP slurm_gateway_lifecycle_task_success Lifecycle reconciliation task success.\n# TYPE slurm_gateway_lifecycle_task_success gauge\n")
	b.WriteString("# HELP slurm_gateway_lifecycle_task_failure Lifecycle reconciliation task failure.\n# TYPE slurm_gateway_lifecycle_task_failure gauge\n")
	b.WriteString("# HELP slurm_gateway_lifecycle_task_success_age_seconds Age of the last successful lifecycle reconciliation task.\n# TYPE slurm_gateway_lifecycle_task_success_age_seconds gauge\n")
	b.WriteString("# HELP slurm_gateway_lifecycle_task_affected_count Affected records from the most recent lifecycle task.\n# TYPE slurm_gateway_lifecycle_task_affected_count gauge\n")
	for _, name := range lifecycleTaskNames {
		outcome, observed := h.tasks[name]
		success := 0
		failure := 0
		if observed && outcome == nil {
			success = 1
		}
		if observed && outcome != nil {
			failure = 1
		}
		age := 0.0
		if succeeded := h.taskSuccess[name]; !succeeded.IsZero() {
			age = time.Since(succeeded).Seconds()
		}
		b.WriteString(fmt.Sprintf("slurm_gateway_lifecycle_task_success{task=%q} %d\n", name, success))
		b.WriteString(fmt.Sprintf("slurm_gateway_lifecycle_task_failure{task=%q} %d\n", name, failure))
		b.WriteString(fmt.Sprintf("slurm_gateway_lifecycle_task_success_age_seconds{task=%q} %.3f\n", name, age))
		b.WriteString(fmt.Sprintf("slurm_gateway_lifecycle_task_affected_count{task=%q} 0\n", name))
	}
	if !h.lastRun.IsZero() {
		b.WriteString(fmt.Sprintf("slurm_gateway_lifecycle_last_run_unix %d\n", h.lastRun.Unix()))
	}
	return b.String()
}

func (h *LifecycleHealth) Record(now time.Time, err error) {
	if h == nil || !h.enabled {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastRun = now.UTC()
	if err == nil {
		h.state, h.lastError = LifecycleReady, ""
		return
	}
	h.state, h.lastError = LifecycleNotReady, err.Error()
}

func (h *LifecycleHealth) State() LifecycleHealthState {
	return h.StateAt(time.Now())
}

func (h *LifecycleHealth) StateAt(now time.Time) LifecycleHealthState {
	if h == nil {
		return LifecycleDisabled
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.enabled || h.state == LifecycleDisabled || h.state == LifecycleStarting {
		return h.state
	}
	for name, outcome := range h.tasks {
		policy, known := lifecycleTaskPolicy[name]
		if !known || !policy.Required || outcome == nil {
			continue
		}
		if succeeded := h.taskSuccess[name]; succeeded.IsZero() || now.UTC().Sub(succeeded) > policy.MaxSuccessAge {
			return LifecycleNotReady
		}
	}
	return h.state
}
