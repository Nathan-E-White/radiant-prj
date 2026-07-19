package gateway

import (
	"context"
	"fmt"
)

type WorkbenchProjectionStream string

const (
	WorkbenchProjectionOperational WorkbenchProjectionStream = "simops_telemetry"
	WorkbenchProjectionMeasured    WorkbenchProjectionStream = "measured_state"
	WorkbenchProjectionSimulated   WorkbenchProjectionStream = "simulated_result_state"
	WorkbenchProjectionTwin        WorkbenchProjectionStream = "twin_state"
)

type WorkbenchProjectionIngestionStage string

const (
	WorkbenchProjectionIngestionFetch   WorkbenchProjectionIngestionStage = "fetch"
	WorkbenchProjectionIngestionProject WorkbenchProjectionIngestionStage = "project"
	WorkbenchProjectionIngestionAppend  WorkbenchProjectionIngestionStage = "append"
	WorkbenchProjectionIngestionPersist WorkbenchProjectionIngestionStage = "persist"
	WorkbenchProjectionIngestionCommit  WorkbenchProjectionIngestionStage = "commit"
)

type WorkbenchProjectionIngestionError struct {
	Stream   WorkbenchProjectionStream
	Stage    WorkbenchProjectionIngestionStage
	Position *WorkbenchProjectionPosition
	Cause    error
}

type WorkbenchProjectionPosition struct {
	Topic     string
	Partition int
	Offset    int64
}

func (e *WorkbenchProjectionIngestionError) Error() string {
	if e.Position == nil {
		return fmt.Sprintf("workbench %s ingestion failed during %s: %v", e.Stream, e.Stage, e.Cause)
	}
	return fmt.Sprintf("workbench %s ingestion failed during %s at %s[%d]@%d: %v", e.Stream, e.Stage, e.Position.Topic, e.Position.Partition, e.Position.Offset, e.Cause)
}

func (e *WorkbenchProjectionIngestionError) Unwrap() error {
	return e.Cause
}

type WorkbenchProjectionIngestionAdapter[T any] struct {
	Stream     WorkbenchProjectionStream
	WriteStage WorkbenchProjectionIngestionStage
	Project    func(SimopsBrokerMessage) (T, error)
	Persist    func(T) (uint64, error)
}

func RunWorkbenchProjectionIngestion[T any](ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics, adapter WorkbenchProjectionIngestionAdapter[T]) error {
	if reader == nil {
		return fmt.Errorf("workbench %s ingestion requires a reader", adapter.Stream)
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if adapter.Stream == "" || adapter.Project == nil || adapter.Persist == nil {
		return fmt.Errorf("workbench projection ingestion requires a stream, projector, and persistence adapter")
	}
	writeStage := adapter.WriteStage
	if writeStage == "" {
		writeStage = WorkbenchProjectionIngestionPersist
	}

	for {
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionFetch, nil, err, true)
		}
		metrics.MarkBrokerConnected(true)
		if err := ctx.Err(); err != nil {
			return err
		}

		projection, err := adapter.Project(message)
		if err != nil {
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionProject, &message, err, false)
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		written, err := adapter.Persist(projection)
		if err != nil {
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, writeStage, &message, err, false)
		}
		if written > 0 {
			metrics.IncFramesWritten(written)
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := reader.CommitMessages(ctx, message); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionCommit, &message, err, false)
		}
		metrics.MarkConsumed(message.Offset)
	}
}

func failWorkbenchProjectionIngestion(metrics *SimopsConsumerMetrics, stream WorkbenchProjectionStream, stage WorkbenchProjectionIngestionStage, message *SimopsBrokerMessage, cause error, disconnected bool) error {
	err := &WorkbenchProjectionIngestionError{
		Stream: stream, Stage: stage, Cause: cause,
	}
	if message != nil {
		err.Position = &WorkbenchProjectionPosition{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}
	}
	if disconnected {
		metrics.MarkBrokerConnected(false)
	}
	metrics.IncWriteFailures()
	metrics.SetLastError(err)
	return err
}
