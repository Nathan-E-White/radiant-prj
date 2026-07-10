//go:build !dataplane

package gateway

import "fmt"

func NewRedpandaEventLog(_ SimopsConfig, _ SimopsStore) (*RedpandaEventLog, error) {
	return nil, fmt.Errorf("redpanda event log requires the dataplane build tag")
}

func NewSimopsKafkaReader(_ SimopsConfig, _ string) (SimopsKafkaReader, error) {
	return nil, fmt.Errorf("redpanda consumer requires the dataplane build tag")
}
