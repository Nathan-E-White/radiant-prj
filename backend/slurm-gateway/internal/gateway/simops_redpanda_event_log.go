package gateway

import (
	"context"
	"encoding/json"
	"fmt"
)

type RedpandaEventLog struct {
	Topic  string
	Store  SimopsStore
	Writer simopsBrokerWriter
}

func (l *RedpandaEventLog) Publish(ctx context.Context, event SimopsEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Writer == nil {
		return fmt.Errorf("redpanda event log requires writer")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := event.RunID
	if event.WorkerID != "" {
		key += "|" + event.WorkerID
	}
	if err := l.Writer.WriteMessages(ctx, SimopsBrokerMessage{
		Key:   []byte(key),
		Value: payload,
		Time:  event.OccurredAt,
	}); err != nil {
		return err
	}
	if l.Store != nil {
		return l.Store.SaveEvent(event)
	}
	return nil
}
