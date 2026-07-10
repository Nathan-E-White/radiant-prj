package gateway

import (
	"context"
	"time"
)

type SimopsBrokerMessage struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	Time      time.Time
}

type SimopsKafkaReader interface {
	FetchMessage(context.Context) (SimopsBrokerMessage, error)
	CommitMessages(context.Context, ...SimopsBrokerMessage) error
	Close() error
}

type simopsBrokerWriter interface {
	WriteMessages(context.Context, ...SimopsBrokerMessage) error
}
