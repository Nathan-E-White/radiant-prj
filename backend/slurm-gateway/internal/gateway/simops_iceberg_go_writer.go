//go:build iceberggo

package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
)

var simopsIcebergIdentifier = icetable.Identifier{"simops", "telemetry_frames"}

type IcebergGoSimopsArtifactWriter struct {
	base        *simopsArtifactWriterBase
	cfg         SimopsConfig
	appendTable func(context.Context, *icetable.Table, arrow.Table, int64, iceberg.Properties) (*icetable.Table, error)
	reloadTable func(context.Context, icecatalog.Catalog) (*icetable.Table, error)
}

func NewIcebergGoSimopsArtifactWriter(cfg SimopsConfig, base *simopsArtifactWriterBase) (SimopsArtifactWriter, error) {
	if err := simopsIcebergLifecycle(cfg).Validate(); err != nil {
		return nil, err
	}
	return &IcebergGoSimopsArtifactWriter{base: base, cfg: cfg}, nil
}

func (w *IcebergGoSimopsArtifactWriter) Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error) {
	plan.Topic = normalizeTopic(plan.Topic)
	if strings.TrimSpace(plan.Artifact.RunID) == "" {
		return plan, fmt.Errorf("artifact run_id is required")
	}
	if strings.TrimSpace(plan.Artifact.ArtifactID) == "" {
		return plan, fmt.Errorf("artifact_id is required")
	}
	plan.Artifact.Status = SimopsArtifactStatusPrepared
	plan.Partition = strings.TrimSpace(plan.Partition)
	if plan.Partition == "" {
		plan.Partition = artifactPartition(plan.Artifact.RunID)
	}
	plan.Artifact.Location = strings.TrimRight(strings.TrimSpace(w.cfg.IcebergWarehouse), "/") + "/simops.db/telemetry_frames"
	plan.Artifact.IcebergTable = "simops.telemetry_frames"
	w.base.updateActiveArtifactID(plan.Artifact.RunID, plan.Artifact.ArtifactID)
	if err := w.base.ensureStoreStatus(plan.Artifact.RunID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return plan, err
	}
	return plan, nil
}

func (w *IcebergGoSimopsArtifactWriter) WriteArtifact(runID string, plan SimopsArtifactWritePlan) error {
	if len(plan.Events) == 0 {
		return w.base.markRunFailed(runID, fmt.Errorf("iceberg-go writer received no telemetry events"), "append iceberg telemetry")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cat, err := w.loadCatalog(ctx)
	if err != nil {
		return w.base.markRunFailed(runID, err, "load iceberg catalog")
	}
	tbl, err := w.loadOrCreateTable(ctx, cat)
	if err != nil {
		return w.base.markRunFailed(runID, err, "load iceberg telemetry table")
	}
	arrowTable, err := simopsEventsArrowTable(plan.Topic, plan.Events)
	if err != nil {
		return w.base.markRunFailed(runID, err, "build iceberg arrow batch")
	}
	defer arrowTable.Release()

	if err := w.appendWithCatalogRetry(ctx, cat, tbl, arrowTable, int64(len(plan.Events)), simopsIcebergAppendProperties(runID, plan.Topic, plan.DeliveryAttemptID)); err != nil {
		return w.base.markRunFailed(runID, err, "append iceberg telemetry")
	}
	if err := w.verifyFreshReadback(ctx, cat, runID, plan); err != nil {
		return w.base.markRunFailed(runID, err, "verify iceberg telemetry readback")
	}
	if err := w.base.ensureStoreStatus(runID, plan.Artifact.ArtifactID, SimopsArtifactStatusPrepared); err != nil {
		return err
	}
	return nil
}

// appendWithCatalogRetry performs one writer-level refresh after a catalog
// conflict. iceberg-go retries within a table transaction; this outer retry
// covers an exhausted transaction whose metadata must be reloaded before the
// same identified delivery can be retried.
func (w *IcebergGoSimopsArtifactWriter) appendWithCatalogRetry(ctx context.Context, cat icecatalog.Catalog, table *icetable.Table, rows arrow.Table, batchSize int64, properties iceberg.Properties) error {
	appendTable := w.appendTable
	if appendTable == nil {
		appendTable = func(ctx context.Context, table *icetable.Table, rows arrow.Table, batchSize int64, properties iceberg.Properties) (*icetable.Table, error) {
			return table.AppendTable(ctx, rows, batchSize, properties)
		}
	}
	if _, err := appendTable(ctx, table, rows, batchSize, properties); err == nil {
		return nil
	} else if !errors.Is(err, icetable.ErrCommitFailed) {
		return err
	}

	reloadTable := w.reloadTable
	if reloadTable == nil {
		reloadTable = func(ctx context.Context, cat icecatalog.Catalog) (*icetable.Table, error) {
			return cat.LoadTable(ctx, simopsIcebergIdentifier)
		}
	}
	fresh, err := reloadTable(ctx, cat)
	if err != nil {
		return fmt.Errorf("reload iceberg table after catalog conflict: %w", err)
	}
	_, err = appendTable(ctx, fresh, rows, batchSize, properties)
	return err
}

func simopsIcebergAppendProperties(runID string, topic string, attemptIDs ...string) iceberg.Properties {
	properties := iceberg.Properties{
		"simops.run_id":      runID,
		"simops.batch_topic": topic,
	}
	if len(attemptIDs) > 0 {
		properties["simops.delivery_attempt_id"] = attemptIDs[0]
	}
	return properties
}

func (*IcebergGoSimopsArtifactWriter) DeliveryAssurance() DeliveryAssurance {
	return DeliveryAssuranceIcebergReadbackVerified
}

func (w *IcebergGoSimopsArtifactWriter) ReconcileDeliveryAttempt(attempt DeliveryAttempt) (VerifiedDeliveryEvidence, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cat, err := w.loadCatalog(ctx)
	if err != nil {
		return VerifiedDeliveryEvidence{}, false, err
	}
	table, err := cat.LoadTable(ctx, simopsIcebergIdentifier)
	if err != nil {
		return VerifiedDeliveryEvidence{}, false, err
	}
	found := false
	for _, snapshot := range table.Metadata().Snapshots() {
		if snapshot.Summary != nil && snapshot.Summary.Properties["simops.delivery_attempt_id"] == attempt.AttemptID {
			found = true
			break
		}
	}
	if !found {
		return VerifiedDeliveryEvidence{}, false, nil
	}
	if err := verifyIcebergAttemptCoordinates(ctx, table, attempt); err != nil {
		return VerifiedDeliveryEvidence{}, false, err
	}
	return VerifiedDeliveryEvidence{AttemptID: attempt.AttemptID, Assurance: DeliveryAssuranceIcebergReadbackVerified, Coordinates: cloneDeliveryCoordinates(attempt.Coordinates), Reconciliation: DeliveryReconciliationVerified, ObservedAt: w.base.resolvedNow()}, true, nil
}

func verifyIcebergAttemptCoordinates(ctx context.Context, table *icetable.Table, attempt DeliveryAttempt) error {
	scan := table.Scan(icetable.WithRowFilter(iceberg.EqualTo(iceberg.Reference("run_id"), attempt.RunID)), icetable.WithSelectedFields("redpanda_topic", "redpanda_partition", "redpanda_offset"))
	readback, err := scan.ToArrowTable(ctx)
	if err != nil {
		return fmt.Errorf("read iceberg delivery attempt: %w", err)
	}
	defer readback.Release()
	observed, err := observedIcebergOffsets(readback)
	if err != nil {
		return err
	}
	for _, coordinate := range attempt.Coordinates {
		if _, ok := observed[icebergOffsetKey(coordinate.Topic, coordinate.Partition, coordinate.Offset)]; !ok {
			return fmt.Errorf("iceberg readback missing delivery coordinate %s", icebergOffsetKey(coordinate.Topic, coordinate.Partition, coordinate.Offset))
		}
	}
	return nil
}

func (w *IcebergGoSimopsArtifactWriter) Commit(runID string) error {
	artifactID := w.base.activeArtifactID(runID)
	if strings.TrimSpace(artifactID) == "" {
		return fmt.Errorf("run %s has no prepared artifact", runID)
	}
	return w.base.ensureStoreStatus(runID, artifactID, SimopsArtifactStatusCommitted)
}

func (w *IcebergGoSimopsArtifactWriter) verifyFreshReadback(ctx context.Context, cat icecatalog.Catalog, runID string, plan SimopsArtifactWritePlan) error {
	fresh, err := cat.LoadTable(ctx, simopsIcebergIdentifier)
	if err != nil {
		return fmt.Errorf("load fresh iceberg table: %w", err)
	}
	scan := fresh.Scan(
		icetable.WithRowFilter(iceberg.EqualTo(iceberg.Reference("run_id"), runID)),
		icetable.WithSelectedFields("run_id", "redpanda_topic", "redpanda_partition", "redpanda_offset"),
	)
	tasks, err := scan.PlanFiles(ctx)
	if err != nil {
		return fmt.Errorf("plan readback scan: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("iceberg table has catalog metadata but no readable data files for run %s", runID)
	}
	for _, task := range tasks {
		if strings.TrimSpace(task.File.FilePath()) == "" {
			return fmt.Errorf("iceberg scan task has empty data-file path for run %s", runID)
		}
		if task.File.Count() <= 0 {
			return fmt.Errorf("iceberg data file %s has no records for run %s", task.File.FilePath(), runID)
		}
	}
	readback, err := scan.ToArrowTable(ctx)
	if err != nil {
		return fmt.Errorf("read iceberg data files: %w", err)
	}
	defer readback.Release()

	if readback.NumRows() == 0 {
		return fmt.Errorf("iceberg readback returned no rows for run %s", runID)
	}
	expected, err := expectedIcebergOffsets(plan.Topic, plan.Events)
	if err != nil {
		return err
	}
	observed, err := observedIcebergOffsets(readback)
	if err != nil {
		return err
	}
	for key := range expected {
		if _, ok := observed[key]; !ok {
			return fmt.Errorf("iceberg readback missing Redpanda coordinate %s for run %s", key, runID)
		}
	}
	return nil
}

func expectedIcebergOffsets(topic string, events []SimopsEvent) (map[string]struct{}, error) {
	expected := make(map[string]struct{})
	for _, event := range events {
		eventTopic := strings.TrimSpace(event.RedpandaTopic)
		if eventTopic == "" {
			eventTopic = topic
		}
		projection, ok, err := ProjectTelemetryEvent(eventTopic, event.RedpandaPartition, event.RedpandaOffset, event)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		expected[icebergOffsetKey(projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)] = struct{}{}
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("iceberg readback has no expected worker.telemetry offsets")
	}
	return expected, nil
}

func observedIcebergOffsets(tbl arrow.Table) (map[string]struct{}, error) {
	col := func(name string) (*arrow.Column, error) {
		idx := tbl.Schema().FieldIndices(name)
		if len(idx) != 1 {
			return nil, fmt.Errorf("iceberg readback missing column %s", name)
		}
		return tbl.Column(idx[0]), nil
	}
	topicCol, err := col("redpanda_topic")
	if err != nil {
		return nil, err
	}
	partitionCol, err := col("redpanda_partition")
	if err != nil {
		return nil, err
	}
	offsetCol, err := col("redpanda_offset")
	if err != nil {
		return nil, err
	}

	observed := make(map[string]struct{})
	row := 0
	for chunkIdx, chunk := range topicCol.Data().Chunks() {
		topics, ok := chunk.(*array.String)
		if !ok {
			return nil, fmt.Errorf("iceberg readback redpanda_topic chunk %d has %T", chunkIdx, chunk)
		}
		for i := 0; i < topics.Len(); i++ {
			partition, err := int32ValueAt(partitionCol, row)
			if err != nil {
				return nil, err
			}
			offset, err := int64ValueAt(offsetCol, row)
			if err != nil {
				return nil, err
			}
			observed[icebergOffsetKey(topics.Value(i), int(partition), offset)] = struct{}{}
			row++
		}
	}
	return observed, nil
}

func int32ValueAt(col *arrow.Column, row int) (int32, error) {
	remaining := row
	for chunkIdx := 0; chunkIdx < col.Data().Len(); chunkIdx++ {
		chunk, ok := col.Data().Chunk(chunkIdx).(*array.Int32)
		if !ok {
			return 0, fmt.Errorf("iceberg readback %s column has %T", col.Name(), col.Data().Chunk(chunkIdx))
		}
		if remaining < chunk.Len() {
			return chunk.Value(remaining), nil
		}
		remaining -= chunk.Len()
	}
	return 0, fmt.Errorf("iceberg readback %s missing row %d", col.Name(), row)
}

func int64ValueAt(col *arrow.Column, row int) (int64, error) {
	remaining := row
	for chunkIdx := 0; chunkIdx < col.Data().Len(); chunkIdx++ {
		chunk, ok := col.Data().Chunk(chunkIdx).(*array.Int64)
		if !ok {
			return 0, fmt.Errorf("iceberg readback %s column has %T", col.Name(), col.Data().Chunk(chunkIdx))
		}
		if remaining < chunk.Len() {
			return chunk.Value(remaining), nil
		}
		remaining -= chunk.Len()
	}
	return 0, fmt.Errorf("iceberg readback %s missing row %d", col.Name(), row)
}

func icebergOffsetKey(topic string, partition int, offset int64) string {
	return fmt.Sprintf("%s/%d/%d", normalizeTopic(topic), partition, offset)
}

func (w *IcebergGoSimopsArtifactWriter) loadCatalog(ctx context.Context) (icecatalog.Catalog, error) {
	return simopsIcebergLifecycle(w.cfg).LoadCatalog(ctx, "simops")
}

func simopsIcebergLifecycle(cfg SimopsConfig) IcebergTableLifecycle {
	return IcebergTableLifecycle{Configuration: IcebergCatalogConfiguration{
		DSN:           cfg.IcebergCatalogDSN,
		Warehouse:     cfg.IcebergWarehouse,
		S3Endpoint:    cfg.IcebergS3Endpoint,
		S3Region:      cfg.IcebergS3Region,
		S3AccessKeyID: cfg.IcebergS3AccessKeyID,
		S3SecretKey:   cfg.IcebergS3SecretKey,
	}}
}

func (w *IcebergGoSimopsArtifactWriter) loadOrCreateTable(ctx context.Context, cat icebergTableLifecycleCatalog) (*icetable.Table, error) {
	return simopsIcebergLifecycle(w.cfg).LoadOrCreateTable(ctx, cat, simopsIcebergIdentifier, simopsIcebergSchema())
}

func simopsIcebergSchema() *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "received_at", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 2, Name: "emitted_at", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 3, Name: "run_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 4, Name: "scenario_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 5, Name: "worker_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 6, Name: "worker_kind", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 7, Name: "sequence", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 8, Name: "payload_type", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 9, Name: "quality", Type: iceberg.PrimitiveTypes.String, Required: false},
		iceberg.NestedField{ID: 10, Name: "source_lag_ms", Type: iceberg.PrimitiveTypes.Float64, Required: false},
		iceberg.NestedField{ID: 11, Name: "collector_lag_ms", Type: iceberg.PrimitiveTypes.Float64, Required: false},
		iceberg.NestedField{ID: 12, Name: "dropped_frame_count", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 13, Name: "frame", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 14, Name: "redpanda_topic", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 15, Name: "redpanda_partition", Type: iceberg.PrimitiveTypes.Int32, Required: true},
		iceberg.NestedField{ID: 16, Name: "redpanda_offset", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	)
}

func simopsIcebergArrowSchema() *arrow.Schema {
	ts := &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
	return arrow.NewSchema([]arrow.Field{
		{Name: "received_at", Type: ts, Nullable: false},
		{Name: "emitted_at", Type: ts, Nullable: false},
		{Name: "run_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "scenario_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "worker_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "worker_kind", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "sequence", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "payload_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "quality", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "source_lag_ms", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "collector_lag_ms", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "dropped_frame_count", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "frame", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_topic", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_partition", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "redpanda_offset", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil)
}

func simopsEventsArrowTable(topic string, events []SimopsEvent) (arrow.Table, error) {
	schema := simopsIcebergArrowSchema()
	mem := memory.DefaultAllocator
	receivedAt := array.NewTimestampBuilder(mem, schema.Field(0).Type.(*arrow.TimestampType))
	emittedAt := array.NewTimestampBuilder(mem, schema.Field(1).Type.(*arrow.TimestampType))
	runID := array.NewStringBuilder(mem)
	scenarioID := array.NewStringBuilder(mem)
	workerID := array.NewStringBuilder(mem)
	workerKind := array.NewStringBuilder(mem)
	sequence := array.NewInt64Builder(mem)
	payloadType := array.NewStringBuilder(mem)
	quality := array.NewStringBuilder(mem)
	sourceLagMs := array.NewFloat64Builder(mem)
	collectorLagMs := array.NewFloat64Builder(mem)
	droppedFrameCount := array.NewInt64Builder(mem)
	frameRaw := array.NewStringBuilder(mem)
	redpandaTopic := array.NewStringBuilder(mem)
	redpandaPartition := array.NewInt32Builder(mem)
	redpandaOffset := array.NewInt64Builder(mem)
	defer receivedAt.Release()
	defer emittedAt.Release()
	defer runID.Release()
	defer scenarioID.Release()
	defer workerID.Release()
	defer workerKind.Release()
	defer sequence.Release()
	defer payloadType.Release()
	defer quality.Release()
	defer sourceLagMs.Release()
	defer collectorLagMs.Release()
	defer droppedFrameCount.Release()
	defer frameRaw.Release()
	defer redpandaTopic.Release()
	defer redpandaPartition.Release()
	defer redpandaOffset.Release()

	for _, event := range events {
		eventTopic := strings.TrimSpace(event.RedpandaTopic)
		if eventTopic == "" {
			eventTopic = topic
		}
		projection, ok, err := ProjectTelemetryEvent(eventTopic, event.RedpandaPartition, event.RedpandaOffset, event)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		receivedAt.Append(arrow.Timestamp(projection.ReceivedAt.UnixMicro()))
		emittedAt.Append(arrow.Timestamp(projection.EmittedAt.UnixMicro()))
		runID.Append(projection.RunID)
		scenarioID.Append(projection.ScenarioID)
		workerID.Append(projection.WorkerID)
		workerKind.Append(string(projection.WorkerKind))
		sequence.Append(int64(projection.Sequence))
		payloadType.Append(projection.PayloadType)
		if projection.Quality == "" {
			quality.AppendNull()
		} else {
			quality.Append(projection.Quality)
		}
		if projection.SourceLagMs.Valid {
			sourceLagMs.Append(projection.SourceLagMs.Float64)
		} else {
			sourceLagMs.AppendNull()
		}
		if projection.CollectorLagMs.Valid {
			collectorLagMs.Append(projection.CollectorLagMs.Float64)
		} else {
			collectorLagMs.AppendNull()
		}
		droppedFrameCount.Append(projection.DroppedFrameCount)
		frameRaw.Append(string(projection.Frame))
		redpandaTopic.Append(projection.RedpandaTopic)
		redpandaPartition.Append(int32(projection.RedpandaPartition))
		redpandaOffset.Append(projection.RedpandaOffset)
	}
	if runID.Len() == 0 {
		return nil, fmt.Errorf("iceberg-go append batch contains no worker.telemetry events")
	}

	arrays := []arrow.Array{
		receivedAt.NewArray(),
		emittedAt.NewArray(),
		runID.NewArray(),
		scenarioID.NewArray(),
		workerID.NewArray(),
		workerKind.NewArray(),
		sequence.NewArray(),
		payloadType.NewArray(),
		quality.NewArray(),
		sourceLagMs.NewArray(),
		collectorLagMs.NewArray(),
		droppedFrameCount.NewArray(),
		frameRaw.NewArray(),
		redpandaTopic.NewArray(),
		redpandaPartition.NewArray(),
		redpandaOffset.NewArray(),
	}
	defer func() {
		for _, arr := range arrays {
			arr.Release()
		}
	}()
	record := array.NewRecordBatch(schema, arrays, int64(runID.Len()))
	defer record.Release()
	return array.NewTableFromRecords(schema, []arrow.RecordBatch{record}), nil
}
