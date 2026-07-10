//go:build dataplane

package gateway

import (
	"fmt"
	"strings"
)

func NewRedpandaEventLog(cfg SimopsConfig, store SimopsStore) (*RedpandaEventLog, error) {
	brokers := csvValues(cfg.RedpandaBrokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("redpanda event log requires brokers")
	}
	if strings.TrimSpace(cfg.RedpandaTopic) == "" {
		return nil, fmt.Errorf("redpanda event log requires topic")
	}
	return &RedpandaEventLog{
		Topic:  cfg.RedpandaTopic,
		Store:  store,
		Writer: newKafkaBrokerWriter(brokers, cfg.RedpandaTopic),
	}, nil
}

func NewSimopsKafkaReader(cfg SimopsConfig, groupID string) (SimopsKafkaReader, error) {
	brokers := csvValues(cfg.RedpandaBrokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("redpanda consumer requires brokers")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("redpanda consumer requires group id")
	}
	topic := strings.TrimSpace(cfg.RedpandaTopic)
	if topic == "" {
		return nil, fmt.Errorf("redpanda consumer requires topic")
	}
	return newKafkaBrokerReader(brokers, topic, groupID), nil
}
