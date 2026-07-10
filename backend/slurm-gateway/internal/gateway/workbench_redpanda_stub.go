//go:build !dataplane

package gateway

import "fmt"

func NewRedpandaWorkbenchEventLog(_ WorkbenchConfig) (*RedpandaWorkbenchEventLog, error) {
	return nil, fmt.Errorf("workbench redpanda event log requires the dataplane build tag")
}

func NewWorkbenchKafkaReader(_ WorkbenchConfig, _ string, _ string) (SimopsKafkaReader, error) {
	return nil, fmt.Errorf("workbench redpanda consumer requires the dataplane build tag")
}
