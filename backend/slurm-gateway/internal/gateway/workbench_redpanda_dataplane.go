//go:build dataplane

package gateway

import (
	"fmt"
	"strings"
)

func NewRedpandaWorkbenchEventLog(cfg WorkbenchConfig) (*RedpandaWorkbenchEventLog, error) {
	brokers := csvValues(cfg.RedpandaBrokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("workbench redpanda event log requires brokers")
	}
	return &RedpandaWorkbenchEventLog{
		ScadaTopic:     cfg.ScadaTopic,
		ResultsTopic:   cfg.ResultsTopic,
		TwinStateTopic: cfg.TwinStateTopic,
		scadaWriter:    newKafkaBrokerWriter(brokers, cfg.ScadaTopic),
		resultWriter:   newKafkaBrokerWriter(brokers, cfg.ResultsTopic),
		twinWriter:     newKafkaBrokerWriter(brokers, cfg.TwinStateTopic),
	}, nil
}

func NewWorkbenchKafkaReader(cfg WorkbenchConfig, topic string, groupID string) (SimopsKafkaReader, error) {
	brokers := csvValues(cfg.RedpandaBrokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("workbench redpanda consumer requires brokers")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, fmt.Errorf("workbench redpanda consumer requires topic")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("workbench redpanda consumer requires group id")
	}
	return newKafkaBrokerReader(brokers, topic, groupID), nil
}
