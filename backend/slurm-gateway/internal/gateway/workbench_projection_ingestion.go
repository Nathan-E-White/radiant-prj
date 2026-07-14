package gateway

import (
	"context"
	"fmt"
)

type WorkbenchProjectionStream string

const (
	WorkbenchProjectionMeasured  WorkbenchProjectionStream = "measured_state"
	WorkbenchProjectionSimulated WorkbenchProjectionStream = "simulated_result_state"
	WorkbenchProjectionTwin      WorkbenchProjectionStream = "twin_state"
)

type WorkbenchProjectionIngestionStage string

const (
	WorkbenchProjectionIngestionFetch   WorkbenchProjectionIngestionStage = "fetch"
	WorkbenchProjectionIngestionProject WorkbenchProjectionIngestionStage = "project"
	WorkbenchProjectionIngestionPersist WorkbenchProjectionIngestionStage = "persist"
	WorkbenchProjectionIngestionCommit  WorkbenchProjectionIngestionStage = "commit"
)

type WorkbenchProjectionIngestionError struct {
	Stream    WorkbenchProjectionStream
	Stage     WorkbenchProjectionIngestionStage
	Topic     string
	Partition int
	Offset    int64
	Cause     error
}

func (e *WorkbenchProjectionIngestionError) Error() string {
	return fmt.Sprintf("workbench %s ingestion failed during %s at %s[%d]@%d: %v", e.Stream, e.Stage, e.Topic, e.Partition, e.Offset, e.Cause)
}

func (e *WorkbenchProjectionIngestionError) Unwrap() error {
	return e.Cause
}

type WorkbenchProjectionIngestionAdapter[T any] struct {
	Stream  WorkbenchProjectionStream
	Project func(SimopsBrokerMessage) (T, error)
	Persist func(T) (bool, error)
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

	for {
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionFetch, message, err, true)
		}
		metrics.MarkBrokerConnected(true)
		if err := ctx.Err(); err != nil {
			return err
		}

		projection, err := adapter.Project(message)
		if err != nil {
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionProject, message, err, false)
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		written, err := adapter.Persist(projection)
		if err != nil {
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionPersist, message, err, false)
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := reader.CommitMessages(ctx, message); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failWorkbenchProjectionIngestion(metrics, adapter.Stream, WorkbenchProjectionIngestionCommit, message, err, false)
		}
		metrics.MarkConsumed(message.Offset)
		if written {
			metrics.IncFramesWritten(1)
		}
		metrics.SetLastError(nil)
	}
}

func failWorkbenchProjectionIngestion(metrics *SimopsConsumerMetrics, stream WorkbenchProjectionStream, stage WorkbenchProjectionIngestionStage, message SimopsBrokerMessage, cause error, disconnected bool) error {
	err := &WorkbenchProjectionIngestionError{
		Stream: stream, Stage: stage, Topic: message.Topic, Partition: message.Partition, Offset: message.Offset, Cause: cause,
	}
	if disconnected {
		metrics.MarkBrokerConnected(false)
	}
	metrics.IncWriteFailures()
	metrics.SetLastError(err)
	return err
}
