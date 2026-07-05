package gateway

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Metrics struct {
	mu      sync.Mutex
	submits map[string]int
}

func NewMetrics() *Metrics {
	return &Metrics{submits: make(map[string]int)}
}

func (m *Metrics) IncSubmit(status string, mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submits[metricKey(status, mode)]++
}

func (m *Metrics) Render(ready bool) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var builder strings.Builder
	builder.WriteString("# HELP slurm_gateway_ready Gateway readiness state.\n")
	builder.WriteString("# TYPE slurm_gateway_ready gauge\n")
	if ready {
		builder.WriteString("slurm_gateway_ready 1\n")
	} else {
		builder.WriteString("slurm_gateway_ready 0\n")
	}
	builder.WriteString("# HELP slurm_gateway_job_submit_total Job submission attempts by status and mode.\n")
	builder.WriteString("# TYPE slurm_gateway_job_submit_total counter\n")

	keys := make([]string, 0, len(m.submits))
	for key := range m.submits {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		status, mode, _ := strings.Cut(key, "|")
		builder.WriteString(fmt.Sprintf("slurm_gateway_job_submit_total{status=%q,mode=%q} %d\n", status, mode, m.submits[key]))
	}

	return builder.String()
}

func metricKey(status string, mode string) string {
	return status + "|" + mode
}
