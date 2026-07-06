package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	_ "github.com/apache/iceberg-go/catalog/sql"
	sqlcatalog "github.com/apache/iceberg-go/catalog/sql"
	iceio "github.com/apache/iceberg-go/io"
	_ "github.com/apache/iceberg-go/io/gocloud"
	icetable "github.com/apache/iceberg-go/table"
)

var (
	workbenchScadaIcebergIdentifier  = icetable.Identifier{"scada", "measured_frames"}
	workbenchResultIcebergIdentifier = icetable.Identifier{"simops", "simulated_results"}
	workbenchTwinIcebergIdentifier   = icetable.Identifier{"digital_twin", "state_values"}
)

type WorkbenchIcebergWriter struct {
	cfg WorkbenchConfig
}

func NewWorkbenchIcebergWriter(cfg WorkbenchConfig) (*WorkbenchIcebergWriter, error) {
	if strings.TrimSpace(cfg.IcebergCatalogDSN) == "" {
		return nil, fmt.Errorf("WORKBENCH_ICEBERG_CATALOG_DSN is required")
	}
	if strings.TrimSpace(cfg.IcebergWarehouse) == "" {
		return nil, fmt.Errorf("WORKBENCH_ICEBERG_WAREHOUSE is required")
	}
	return &WorkbenchIcebergWriter{cfg: cfg}, nil
}

func (w *WorkbenchIcebergWriter) AppendScada(ctx context.Context, projection ScadaProjection) error {
	cat, err := w.loadCatalog(ctx)
	if err != nil {
		return err
	}
	tbl, err := w.loadOrCreateTable(ctx, cat, workbenchScadaIcebergIdentifier, workbenchScadaIcebergSchema())
	if err != nil {
		return err
	}
	arrowTable, err := scadaProjectionArrowTable(projection)
	if err != nil {
		return err
	}
	defer arrowTable.Release()
	if _, err := tbl.AppendTable(ctx, arrowTable, arrowTable.NumRows(), iceberg.Properties{
		"workbench.topic": projection.RedpandaTopic,
	}); err != nil {
		return err
	}
	return verifyWorkbenchIcebergOffset(ctx, cat, workbenchScadaIcebergIdentifier, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
}

func (w *WorkbenchIcebergWriter) AppendResult(ctx context.Context, projection SimopsResultProjection) error {
	cat, err := w.loadCatalog(ctx)
	if err != nil {
		return err
	}
	tbl, err := w.loadOrCreateTable(ctx, cat, workbenchResultIcebergIdentifier, workbenchResultIcebergSchema())
	if err != nil {
		return err
	}
	arrowTable, err := resultProjectionArrowTable(projection)
	if err != nil {
		return err
	}
	defer arrowTable.Release()
	if _, err := tbl.AppendTable(ctx, arrowTable, arrowTable.NumRows(), iceberg.Properties{
		"workbench.topic": projection.RedpandaTopic,
	}); err != nil {
		return err
	}
	return verifyWorkbenchIcebergOffset(ctx, cat, workbenchResultIcebergIdentifier, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
}

func (w *WorkbenchIcebergWriter) AppendTwin(ctx context.Context, projection TwinStateProjection) error {
	cat, err := w.loadCatalog(ctx)
	if err != nil {
		return err
	}
	tbl, err := w.loadOrCreateTable(ctx, cat, workbenchTwinIcebergIdentifier, workbenchTwinIcebergSchema())
	if err != nil {
		return err
	}
	arrowTable, err := twinProjectionArrowTable(projection)
	if err != nil {
		return err
	}
	defer arrowTable.Release()
	if _, err := tbl.AppendTable(ctx, arrowTable, arrowTable.NumRows(), iceberg.Properties{
		"workbench.topic": projection.RedpandaTopic,
	}); err != nil {
		return err
	}
	return verifyWorkbenchIcebergOffset(ctx, cat, workbenchTwinIcebergIdentifier, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
}

func (w *WorkbenchIcebergWriter) loadCatalog(ctx context.Context) (icecatalog.Catalog, error) {
	props := iceberg.Properties{
		"type":                "sql",
		"uri":                 w.cfg.IcebergCatalogDSN,
		sqlcatalog.DriverKey:  "pgx",
		sqlcatalog.DialectKey: string(sqlcatalog.Postgres),
		"init_catalog_tables": "true",
		"warehouse":           strings.TrimRight(strings.TrimSpace(w.cfg.IcebergWarehouse), "/"),
	}
	for key, value := range w.s3Properties() {
		props[key] = value
	}
	return icecatalog.Load(ctx, "workbench", props)
}

func (w *WorkbenchIcebergWriter) s3Properties() iceberg.Properties {
	props := iceberg.Properties{}
	if endpoint := strings.TrimSpace(w.cfg.IcebergS3Endpoint); endpoint != "" {
		props[iceio.S3EndpointURL] = endpoint
	}
	if region := strings.TrimSpace(w.cfg.IcebergS3Region); region != "" {
		props[iceio.S3Region] = region
		props[iceio.S3ClientRegion] = region
	}
	if accessKey := strings.TrimSpace(w.cfg.IcebergS3AccessKeyID); accessKey != "" {
		props[iceio.S3AccessKeyID] = accessKey
	}
	if secretKey := strings.TrimSpace(w.cfg.IcebergS3SecretKey); secretKey != "" {
		props[iceio.S3SecretAccessKey] = secretKey
	}
	return props
}

func (w *WorkbenchIcebergWriter) loadOrCreateTable(ctx context.Context, cat icecatalog.Catalog, identifier icetable.Identifier, schema *iceberg.Schema) (*icetable.Table, error) {
	tbl, err := cat.LoadTable(ctx, identifier)
	if err == nil {
		return tbl, nil
	}
	if !errors.Is(err, icecatalog.ErrNoSuchTable) {
		return nil, err
	}
	namespace := icetable.Identifier{identifier[0]}
	if err := cat.CreateNamespace(ctx, namespace, iceberg.Properties{}); err != nil && !errors.Is(err, icecatalog.ErrNamespaceAlreadyExists) {
		return nil, err
	}
	tableProps := iceberg.Properties{icetable.PropertyFormatVersion: "2"}
	for key, value := range w.s3Properties() {
		tableProps[key] = value
	}
	tbl, err = cat.CreateTable(ctx, identifier, schema, icecatalog.WithProperties(tableProps))
	if err != nil {
		if errors.Is(err, icecatalog.ErrTableAlreadyExists) {
			return cat.LoadTable(ctx, identifier)
		}
		return nil, err
	}
	return tbl, nil
}

func verifyWorkbenchIcebergOffset(ctx context.Context, cat icecatalog.Catalog, identifier icetable.Identifier, topic string, partition int, offset int64) error {
	fresh, err := cat.LoadTable(ctx, identifier)
	if err != nil {
		return fmt.Errorf("load fresh iceberg table: %w", err)
	}
	scan := fresh.Scan(icetable.WithSelectedFields("redpanda_topic", "redpanda_partition", "redpanda_offset"))
	tasks, err := scan.PlanFiles(ctx)
	if err != nil {
		return fmt.Errorf("plan readback scan: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("iceberg table %v has no readable data files", identifier)
	}
	readback, err := scan.ToArrowTable(ctx)
	if err != nil {
		return fmt.Errorf("read iceberg table %v: %w", identifier, err)
	}
	defer readback.Release()
	observed, err := observedIcebergOffsets(readback)
	if err != nil {
		return err
	}
	key := icebergOffsetKey(topic, partition, offset)
	if _, ok := observed[key]; !ok {
		return fmt.Errorf("iceberg table %v missing Redpanda coordinate %s", identifier, key)
	}
	return nil
}

func workbenchScadaIcebergSchema() *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "observed_at", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 2, Name: "sampled_at", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 3, Name: "source_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 4, Name: "tag_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 5, Name: "asset_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 6, Name: "signal_kind", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 7, Name: "sequence", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 8, Name: "unit", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 9, Name: "quality", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 10, Name: "value_basis", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 11, Name: "frame", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 12, Name: "redpanda_topic", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 13, Name: "redpanda_partition", Type: iceberg.PrimitiveTypes.Int32, Required: true},
		iceberg.NestedField{ID: 14, Name: "redpanda_offset", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	)
}

func workbenchResultIcebergSchema() *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "produced_at", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 2, Name: "run_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 3, Name: "scenario_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 4, Name: "worker_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 5, Name: "sequence", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 6, Name: "result_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 7, Name: "entity_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 8, Name: "value_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 9, Name: "value_basis", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 10, Name: "unit", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 11, Name: "confidence", Type: iceberg.PrimitiveTypes.Float64, Required: true},
		iceberg.NestedField{ID: 12, Name: "frame", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 13, Name: "redpanda_topic", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 14, Name: "redpanda_partition", Type: iceberg.PrimitiveTypes.Int32, Required: true},
		iceberg.NestedField{ID: 15, Name: "redpanda_offset", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	)
}

func workbenchTwinIcebergSchema() *iceberg.Schema {
	return iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "as_of", Type: iceberg.PrimitiveTypes.TimestampTz, Required: true},
		iceberg.NestedField{ID: 2, Name: "twin_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 3, Name: "entity_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 4, Name: "value_id", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 5, Name: "value_basis", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 6, Name: "unit", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 7, Name: "confidence", Type: iceberg.PrimitiveTypes.Float64, Required: true},
		iceberg.NestedField{ID: 8, Name: "state", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 9, Name: "redpanda_topic", Type: iceberg.PrimitiveTypes.String, Required: true},
		iceberg.NestedField{ID: 10, Name: "redpanda_partition", Type: iceberg.PrimitiveTypes.Int32, Required: true},
		iceberg.NestedField{ID: 11, Name: "redpanda_offset", Type: iceberg.PrimitiveTypes.Int64, Required: true},
	)
}

func scadaProjectionArrowTable(projection ScadaProjection) (arrow.Table, error) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "observed_at", Type: timestampType(), Nullable: false},
		{Name: "sampled_at", Type: timestampType(), Nullable: false},
		{Name: "source_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "tag_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "asset_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "signal_kind", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "sequence", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "unit", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "quality", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "value_basis", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "frame", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_topic", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_partition", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "redpanda_offset", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil)
	return buildSimpleTable(schema, 1, func(builders []array.Builder) {
		builders[0].(*array.TimestampBuilder).Append(arrow.Timestamp(projection.ObservedAt.UnixMicro()))
		builders[1].(*array.TimestampBuilder).Append(arrow.Timestamp(projection.SampledAt.UnixMicro()))
		builders[2].(*array.StringBuilder).Append(projection.Frame.SourceID)
		builders[3].(*array.StringBuilder).Append(projection.Frame.TagID)
		builders[4].(*array.StringBuilder).Append(projection.Frame.AssetID)
		builders[5].(*array.StringBuilder).Append(string(projection.Frame.SignalKind))
		builders[6].(*array.Int64Builder).Append(int64(projection.Frame.Sequence))
		builders[7].(*array.StringBuilder).Append(projection.Frame.Unit)
		builders[8].(*array.StringBuilder).Append(projection.Frame.Quality)
		builders[9].(*array.StringBuilder).Append(string(projection.Frame.ValueBasis))
		builders[10].(*array.StringBuilder).Append(string(projection.Raw))
		builders[11].(*array.StringBuilder).Append(projection.RedpandaTopic)
		builders[12].(*array.Int32Builder).Append(int32(projection.RedpandaPartition))
		builders[13].(*array.Int64Builder).Append(projection.RedpandaOffset)
	})
}

func resultProjectionArrowTable(projection SimopsResultProjection) (arrow.Table, error) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "produced_at", Type: timestampType(), Nullable: false},
		{Name: "run_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "scenario_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "worker_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "sequence", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "result_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "entity_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "value_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "value_basis", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "unit", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "confidence", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "frame", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_topic", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_partition", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "redpanda_offset", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil)
	rows := len(projection.Frame.Values)
	if rows == 0 {
		return nil, fmt.Errorf("simops result projection has no values")
	}
	return buildSimpleTable(schema, rows, func(builders []array.Builder) {
		for _, value := range projection.Frame.Values {
			builders[0].(*array.TimestampBuilder).Append(arrow.Timestamp(projection.ProducedAt.UnixMicro()))
			builders[1].(*array.StringBuilder).Append(projection.Frame.RunID)
			builders[2].(*array.StringBuilder).Append(projection.Frame.ScenarioID)
			builders[3].(*array.StringBuilder).Append(projection.Frame.WorkerID)
			builders[4].(*array.Int64Builder).Append(int64(projection.Frame.Sequence))
			builders[5].(*array.StringBuilder).Append(value.ResultID)
			builders[6].(*array.StringBuilder).Append(value.EntityID)
			builders[7].(*array.StringBuilder).Append(value.ValueID)
			builders[8].(*array.StringBuilder).Append(string(projection.Frame.ValueBasis))
			builders[9].(*array.StringBuilder).Append(value.Unit)
			builders[10].(*array.Float64Builder).Append(value.Confidence)
			builders[11].(*array.StringBuilder).Append(string(projection.Raw))
			builders[12].(*array.StringBuilder).Append(projection.RedpandaTopic)
			builders[13].(*array.Int32Builder).Append(int32(projection.RedpandaPartition))
			builders[14].(*array.Int64Builder).Append(projection.RedpandaOffset)
		}
	})
}

func twinProjectionArrowTable(projection TwinStateProjection) (arrow.Table, error) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "as_of", Type: timestampType(), Nullable: false},
		{Name: "twin_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "entity_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "value_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "value_basis", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "unit", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "confidence", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "state", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_topic", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "redpanda_partition", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "redpanda_offset", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil)
	rows := 0
	for _, entity := range projection.State.Entities {
		rows += len(entity.Values)
	}
	if rows == 0 {
		return nil, fmt.Errorf("twin projection has no values")
	}
	return buildSimpleTable(schema, rows, func(builders []array.Builder) {
		for _, entity := range projection.State.Entities {
			for _, value := range entity.Values {
				builders[0].(*array.TimestampBuilder).Append(arrow.Timestamp(projection.AsOf.UnixMicro()))
				builders[1].(*array.StringBuilder).Append(projection.State.TwinID)
				builders[2].(*array.StringBuilder).Append(entity.EntityID)
				builders[3].(*array.StringBuilder).Append(value.ValueID)
				builders[4].(*array.StringBuilder).Append(string(value.ValueBasis))
				builders[5].(*array.StringBuilder).Append(value.Unit)
				builders[6].(*array.Float64Builder).Append(value.Confidence)
				builders[7].(*array.StringBuilder).Append(string(projection.Raw))
				builders[8].(*array.StringBuilder).Append(projection.RedpandaTopic)
				builders[9].(*array.Int32Builder).Append(int32(projection.RedpandaPartition))
				builders[10].(*array.Int64Builder).Append(projection.RedpandaOffset)
			}
		}
	})
}

func timestampType() *arrow.TimestampType {
	return &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
}

func buildSimpleTable(schema *arrow.Schema, rows int, appendRows func([]array.Builder)) (arrow.Table, error) {
	mem := memory.DefaultAllocator
	builders := make([]array.Builder, len(schema.Fields()))
	for i, field := range schema.Fields() {
		switch field.Type.ID() {
		case arrow.TIMESTAMP:
			builders[i] = array.NewTimestampBuilder(mem, field.Type.(*arrow.TimestampType))
		case arrow.STRING:
			builders[i] = array.NewStringBuilder(mem)
		case arrow.INT64:
			builders[i] = array.NewInt64Builder(mem)
		case arrow.INT32:
			builders[i] = array.NewInt32Builder(mem)
		case arrow.FLOAT64:
			builders[i] = array.NewFloat64Builder(mem)
		default:
			return nil, fmt.Errorf("unsupported arrow type %s", field.Type)
		}
		defer builders[i].Release()
	}
	appendRows(builders)
	arrays := make([]arrow.Array, len(builders))
	for i, builder := range builders {
		arrays[i] = builder.NewArray()
		defer arrays[i].Release()
	}
	record := array.NewRecordBatch(schema, arrays, int64(rows))
	defer record.Release()
	return array.NewTableFromRecords(schema, []arrow.RecordBatch{record}), nil
}
