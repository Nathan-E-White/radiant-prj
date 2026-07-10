package gateway

import (
	"context"
	"encoding/json"
	"fmt"
)

type SimopsTelemetryProjectionStore interface {
	SaveProjection(ctx context.Context, consumerName string, projection SimopsTelemetryProjection) (bool, error)
}

func RunTimescaleTelemetryConsumer(ctx context.Context, cfg SimopsConfig, reader SimopsKafkaReader, store SimopsTelemetryProjectionStore, metrics *SimopsConsumerMetrics) error {
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if store == nil {
		return fmt.Errorf("timescale telemetry consumer requires store")
	}
	if reader == nil {
		created, err := NewSimopsKafkaReader(cfg, cfg.TimescaleConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
	}
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			metrics.MarkBrokerConnected(false)
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.MarkConsumed(msg.Offset)

		event, err := decodeSimopsKafkaEvent(msg)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
				return commitErr
			}
			continue
		}
		event.RedpandaTopic = msg.Topic
		event.RedpandaPartition = msg.Partition
		event.RedpandaOffset = msg.Offset

		projection, ok, err := ProjectTelemetryEvent(msg.Topic, msg.Partition, msg.Offset, event)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		if ok {
			written, err := store.SaveProjection(ctx, cfg.TimescaleConsumerGroup, projection)
			if err != nil {
				metrics.IncWriteFailures()
				metrics.SetLastError(err)
				return err
			}
			if written {
				metrics.IncFramesWritten(1)
			}
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.SetLastError(nil)
	}
}

func RunArtifactIntentConsumer(ctx context.Context, cfg SimopsConfig, reader SimopsKafkaReader, processor *SimopsArtifactIntentProcessor, metrics *SimopsConsumerMetrics) error {
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if processor == nil {
		return fmt.Errorf("artifact intent consumer requires processor")
	}
	if reader == nil {
		created, err := NewSimopsKafkaReader(cfg, cfg.IcebergConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
	}
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			metrics.MarkBrokerConnected(false)
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.MarkConsumed(msg.Offset)

		event, err := decodeSimopsKafkaEvent(msg)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
				return commitErr
			}
			continue
		}
		event.RedpandaTopic = msg.Topic
		event.RedpandaPartition = msg.Partition
		event.RedpandaOffset = msg.Offset
		ready, err := processor.ProcessEvent(ctx, event)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		if ready > 0 {
			metrics.IncFramesWritten(uint64(ready))
			metrics.IncBatchFlushes()
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.SetLastError(nil)
	}
}

func decodeSimopsKafkaEvent(msg SimopsBrokerMessage) (SimopsEvent, error) {
	var event SimopsEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return SimopsEvent{}, fmt.Errorf("decode simops kafka event topic=%s partition=%d offset=%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
	}
	return event, nil
}
