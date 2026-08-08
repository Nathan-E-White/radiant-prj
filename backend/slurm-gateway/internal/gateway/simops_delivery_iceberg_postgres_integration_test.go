//go:build iceberggo && postgresintegration

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
)

func TestPostgresIcebergDeliveryRecoversAmbiguousAppendAcrossRestart(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	store := &PostgresSimopsStore{db: workbench.db}
	now := time.Now().UTC()
	run := SimopsRunRecord{RunID: "delivery-iceberg-restart", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: now, UpdatedAt: now}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	cfg := deliveryIcebergPostgresConfig(t, workbench)
	writer := newPostgresIcebergDeliveryWriter(t, cfg)
	first := deliveryIcebergEvent(t, run.RunID, 41)
	writer.appendTable = func(ctx context.Context, table *icetable.Table, rows arrow.Table, batchSize int64, properties iceberg.Properties) (*icetable.Table, error) {
		if _, err := table.AppendTable(ctx, rows, batchSize, properties); err != nil {
			return nil, err
		}
		return nil, context.DeadlineExceeded
	}
	processor := NewSimopsArtifactIntentProcessorWithDeliveryStore(writer, nil, cfg.RedpandaTopic, 1, time.Now, store)
	if _, err := processor.ProcessEvent(context.Background(), first); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ambiguous post-append timeout: %v", err)
	}

	restartedDB := reopenArtifactForgePostgresTestDatabase(t, workbench.db)
	restartedStore := &PostgresSimopsStore{db: restartedDB}
	restartedWriter := newPostgresIcebergDeliveryWriter(t, cfg)
	restartedProcessor := NewSimopsArtifactIntentProcessorWithDeliveryStore(restartedWriter, nil, cfg.RedpandaTopic, 1, time.Now, restartedStore)
	if written, err := restartedProcessor.ProcessEvent(context.Background(), first); err != nil || written != 1 {
		t.Fatalf("reconcile restarted coordinate: written=%d err=%v", written, err)
	}
	next := deliveryIcebergEvent(t, run.RunID, 42)
	if written, err := restartedProcessor.ProcessEvent(context.Background(), next); err != nil || written != 1 {
		t.Fatalf("append new coordinate after recovery: written=%d err=%v", written, err)
	}

	attempts, err := restartedStore.ListDeliveryAttempts(run.RunID)
	if err != nil || len(attempts) != 2 {
		t.Fatalf("delivery attempts=%#v err=%v", attempts, err)
	}
	if attempts[0].State != DeliveryAttemptResolved || attempts[0].Evidence == nil || attempts[0].Evidence.Assurance != DeliveryAssuranceIcebergReadbackVerified {
		t.Fatalf("recovered attempt lacks one truthful evidence record: %#v", attempts[0])
	}
	if attempts[0].Coordinates[0].Offset != first.RedpandaOffset || attempts[1].Coordinates[0].Offset != next.RedpandaOffset {
		t.Fatalf("recovery changed delivery coordinates: %#v", attempts)
	}

	controller := NewSimopsController(cfg, restartedStore, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: restartedStore}, IcebergArtifactPlanner{}, nil, nil)
	response, status, err := controller.GetRun(run.RunID)
	if err != nil || status != 200 || len(response.DeliveryAttempts) != 2 || response.DeliveryAttempts[0].Assurance != DeliveryAssuranceIcebergReadbackVerified {
		t.Fatalf("Run read does not expose recovered assurance: status=%d response=%#v err=%v", status, response, err)
	}

	cat, err := restartedWriter.loadCatalog(context.Background())
	if err != nil {
		t.Fatalf("load Iceberg catalog for readback: %v", err)
	}
	table, err := cat.LoadTable(context.Background(), simopsIcebergIdentifier)
	if err != nil {
		t.Fatalf("load Iceberg telemetry table: %v", err)
	}
	readback, err := table.Scan().ToArrowTable(context.Background())
	if err != nil {
		t.Fatalf("read Iceberg telemetry: %v", err)
	}
	defer readback.Release()
	if readback.NumRows() != 2 {
		t.Fatalf("Iceberg rows=%d want one logical copy of each coordinate", readback.NumRows())
	}
	offsets, err := observedIcebergOffsets(readback)
	if err != nil || len(offsets) != 2 {
		t.Fatalf("Iceberg offsets=%#v err=%v", offsets, err)
	}
	for _, event := range []SimopsEvent{first, next} {
		if _, ok := offsets[icebergOffsetKey(event.RedpandaTopic, event.RedpandaPartition, event.RedpandaOffset)]; !ok {
			t.Fatalf("missing delivered coordinate for offset %d", event.RedpandaOffset)
		}
	}
}

func TestPostgresIcebergDeliveryRetriesCatalogConflictWithFreshReadback(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	store := &PostgresSimopsStore{db: workbench.db}
	now := time.Now().UTC()
	run := SimopsRunRecord{RunID: "delivery-iceberg-conflict", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: now, UpdatedAt: now}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	cfg := deliveryIcebergPostgresConfig(t, workbench)
	writer := newPostgresIcebergDeliveryWriter(t, cfg)
	appendCalls := 0
	reloads := 0
	var attemptIDs []string
	writer.appendTable = func(ctx context.Context, table *icetable.Table, rows arrow.Table, batchSize int64, properties iceberg.Properties) (*icetable.Table, error) {
		appendCalls++
		attemptIDs = append(attemptIDs, properties["simops.delivery_attempt_id"])
		if appendCalls == 1 {
			return nil, fmt.Errorf("injected catalog conflict: %w", icetable.ErrCommitFailed)
		}
		return table.AppendTable(ctx, rows, batchSize, properties)
	}
	writer.reloadTable = func(ctx context.Context, cat icecatalog.Catalog) (*icetable.Table, error) {
		reloads++
		return cat.LoadTable(ctx, simopsIcebergIdentifier)
	}

	event := deliveryIcebergEvent(t, run.RunID, 61)
	processor := NewSimopsArtifactIntentProcessorWithDeliveryStore(writer, nil, cfg.RedpandaTopic, 1, time.Now, store)
	if written, err := processor.ProcessEvent(context.Background(), event); err != nil || written != 1 {
		t.Fatalf("append after catalog conflict: written=%d err=%v", written, err)
	}
	if appendCalls != 2 || reloads != 1 || len(attemptIDs) != 2 || attemptIDs[0] == "" || attemptIDs[0] != attemptIDs[1] {
		t.Fatalf("catalog retry did not preserve one stable attempt identity: calls=%d reloads=%d attempts=%#v", appendCalls, reloads, attemptIDs)
	}

	attempts, err := store.ListDeliveryAttempts(run.RunID)
	if err != nil || len(attempts) != 1 || attempts[0].State != DeliveryAttemptResolved || attempts[0].Evidence == nil {
		t.Fatalf("catalog retry did not persist verified delivery evidence: attempts=%#v err=%v", attempts, err)
	}
	cat, err := writer.loadCatalog(context.Background())
	if err != nil {
		t.Fatalf("load Iceberg catalog: %v", err)
	}
	table, err := cat.LoadTable(context.Background(), simopsIcebergIdentifier)
	if err != nil {
		t.Fatalf("load Iceberg telemetry table: %v", err)
	}
	readback, err := table.Scan().ToArrowTable(context.Background())
	if err != nil {
		t.Fatalf("read Iceberg telemetry: %v", err)
	}
	defer readback.Release()
	if readback.NumRows() != 1 {
		t.Fatalf("catalog retry rows=%d want one logical delivery", readback.NumRows())
	}
	offsets, err := observedIcebergOffsets(readback)
	if err != nil {
		t.Fatalf("read Iceberg delivery offsets: %v", err)
	}
	if _, ok := offsets[icebergOffsetKey(event.RedpandaTopic, event.RedpandaPartition, event.RedpandaOffset)]; !ok {
		t.Fatalf("catalog retry lost delivery coordinate %d", event.RedpandaOffset)
	}
}

func deliveryIcebergPostgresConfig(t *testing.T, workbench *PostgresWorkbenchStore) SimopsConfig {
	t.Helper()
	parsed, err := url.Parse(strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN")))
	if err != nil {
		t.Fatalf("parse Postgres test DSN: %v", err)
	}
	var schema string
	if err := workbench.db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("read isolated schema: %v", err)
	}
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = query.Encode()
	cfg := DefaultConfig().Simops
	cfg.IcebergWriterMode = "iceberg-go"
	cfg.IcebergCatalogDSN = parsed.String()
	cfg.IcebergWarehouse = "file://" + t.TempDir()
	return cfg
}

func newPostgresIcebergDeliveryWriter(t *testing.T, cfg SimopsConfig) *IcebergGoSimopsArtifactWriter {
	t.Helper()
	writer, err := NewIcebergGoSimopsArtifactWriter(cfg, &simopsArtifactWriterBase{now: time.Now})
	if err != nil {
		t.Fatalf("new Iceberg delivery writer: %v", err)
	}
	return writer.(*IcebergGoSimopsArtifactWriter)
}

func deliveryIcebergEvent(t *testing.T, runID string, offset int64) SimopsEvent {
	t.Helper()
	frame := testTelemetryFrame(t, runID, "scheduler-01", uint64(offset))
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("encode telemetry frame: %v", err)
	}
	return SimopsEvent{RunID: runID, WorkerID: frame.WorkerID, EventType: SimopsEventWorkerTelemetry, Frame: raw, OccurredAt: time.Now().UTC(), RedpandaTopic: "simops.telemetry.v1", RedpandaPartition: 0, RedpandaOffset: offset}
}
