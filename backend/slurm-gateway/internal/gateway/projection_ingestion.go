package gateway

import (
	"context"
	"fmt"
)

type ProjectionIngestionStream string

const (
	ProjectionStreamOperationalTelemetry ProjectionIngestionStream = "simops_telemetry"
	ProjectionStreamMeasuredState        ProjectionIngestionStream = "measured_state"
	ProjectionStreamSimulatedResultState ProjectionIngestionStream = "simulated_result_state"
	ProjectionStreamTwinState            ProjectionIngestionStream = "twin_state"
)

type ProjectionIngestionStage string

const (
	ProjectionIngestionFetch   ProjectionIngestionStage = "fetch"
	ProjectionIngestionProject ProjectionIngestionStage = "project"
	ProjectionIngestionAppend  ProjectionIngestionStage = "append"
	ProjectionIngestionPersist ProjectionIngestionStage = "persist"
	ProjectionIngestionCommit  ProjectionIngestionStage = "commit"
)

type ProjectionIngestionError struct {
	Stream   ProjectionIngestionStream
	Stage    ProjectionIngestionStage
	Position *WorkbenchProjectionPosition
	Cause    error
}

type WorkbenchProjectionPosition struct {
	Topic     string
	Partition int
	Offset    int64
}

func (e *ProjectionIngestionError) Error() string {
	if e.Position == nil {
		return fmt.Sprintf("projection %s ingestion failed during %s: %v", e.Stream, e.Stage, e.Cause)
	}
	return fmt.Sprintf("projection %s ingestion failed during %s at %s[%d]@%d: %v", e.Stream, e.Stage, e.Position.Topic, e.Position.Partition, e.Position.Offset, e.Cause)
}

func (e *ProjectionIngestionError) Unwrap() error {
	return e.Cause
}

type ProjectionIngestionAdapter[T any] struct {
	Stream               ProjectionIngestionStream
	WriteStage           ProjectionIngestionStage
	CommitProjectFailure bool
	Project              func(SimopsBrokerMessage) (T, error)
	Persist              func(T) (uint64, error)
}

func RunProjectionIngestion[T any](ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics, adapter ProjectionIngestionAdapter[T]) error {
	if reader == nil {
		return fmt.Errorf("projection %s ingestion requires a reader", adapter.Stream)
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if adapter.Stream == "" || adapter.Project == nil || adapter.Persist == nil {
		return fmt.Errorf("projection ingestion requires a stream, projector, and persistence adapter")
	}
	metrics.RequireBrokerConnections(string(adapter.Stream))
	writeStage := adapter.WriteStage
	if writeStage == "" {
		writeStage = ProjectionIngestionPersist
	}

	for {
		message, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failProjectionIngestion(metrics, adapter.Stream, ProjectionIngestionFetch, nil, err, true)
		}
		metrics.MarkBrokerConnection(string(adapter.Stream), true)
		if err := ctx.Err(); err != nil {
			return err
		}

		projection, err := adapter.Project(message)
		if err != nil {
			projectionErr := failProjectionIngestion(metrics, adapter.Stream, ProjectionIngestionProject, &message, err, false)
			if !adapter.CommitProjectFailure {
				return projectionErr
			}
			if err := reader.CommitMessages(ctx, message); err != nil {
				return failProjectionIngestion(metrics, adapter.Stream, ProjectionIngestionCommit, &message, err, false)
			}
			metrics.MarkConsumed(message.Offset)
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		written, err := adapter.Persist(projection)
		if err != nil {
			return failProjectionIngestion(metrics, adapter.Stream, writeStage, &message, err, false)
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
			return failProjectionIngestion(metrics, adapter.Stream, ProjectionIngestionCommit, &message, err, false)
		}
		metrics.MarkConsumed(message.Offset)
	}
}

func failProjectionIngestion(metrics *SimopsConsumerMetrics, stream ProjectionIngestionStream, stage ProjectionIngestionStage, message *SimopsBrokerMessage, cause error, disconnected bool) error {
	err := &ProjectionIngestionError{
		Stream: stream, Stage: stage, Cause: cause,
	}
	if message != nil {
		err.Position = &WorkbenchProjectionPosition{Topic: message.Topic, Partition: message.Partition, Offset: message.Offset}
	}
	if disconnected {
		metrics.MarkBrokerConnection(string(stream), false)
	}
	metrics.IncWriteFailures()
	metrics.SetLastError(err)
	return err
}
