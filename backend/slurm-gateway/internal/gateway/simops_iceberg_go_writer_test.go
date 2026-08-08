//go:build iceberggo

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
)

func TestIcebergGoBatchBuildsArrowTableFromTelemetryEvents(t *testing.T) {
	frame := testTelemetryFrame(t, "RUN-ICEBERG-BATCH", "scheduler-01", 5)
	raw, _ := json.Marshal(frame)
	table, err := simopsEventsArrowTable("simops.telemetry.v1", []SimopsEvent{{
		RunID:             frame.RunID,
		WorkerID:          frame.WorkerID,
		EventType:         SimopsEventWorkerTelemetry,
		Frame:             raw,
		OccurredAt:        time.Now().UTC(),
		RedpandaTopic:     "simops.telemetry.v1",
		RedpandaPartition: 1,
		RedpandaOffset:    12,
	}})
	if err != nil {
		t.Fatalf("build arrow table: %v", err)
	}
	defer table.Release()
	if table.NumRows() != 1 {
		t.Fatalf("expected one row, got %d", table.NumRows())
	}
	if got := table.Schema().Field(0).Name; got != "received_at" {
		t.Fatalf("unexpected first column %q", got)
	}
	if got := table.Schema().Field(15).Name; got != "redpanda_offset" {
		t.Fatalf("unexpected final column %q", got)
	}
}

func TestIcebergReadbackOffsetHelpersMatchRedpandaCoordinates(t *testing.T) {
	frame := testTelemetryFrame(t, "RUN-ICEBERG-READBACK", "scheduler-01", 8)
	raw, _ := json.Marshal(frame)
	events := []SimopsEvent{
		{
			RunID:             frame.RunID,
			WorkerID:          frame.WorkerID,
			EventType:         SimopsEventWorkerTelemetry,
			Frame:             raw,
			OccurredAt:        time.Now().UTC(),
			RedpandaTopic:     "simops.telemetry.v1",
			RedpandaPartition: 3,
			RedpandaOffset:    99,
		},
		{
			RunID:      frame.RunID,
			EventType:  SimopsEventRunLifecycle,
			Lifecycle:  SimopsStreaming,
			OccurredAt: time.Now().UTC(),
		},
	}
	expected, err := expectedIcebergOffsets("simops.telemetry.v1", events)
	if err != nil {
		t.Fatalf("expected offsets: %v", err)
	}
	if _, ok := expected["simops.telemetry.v1/3/99"]; !ok {
		t.Fatalf("expected Redpanda coordinate missing from %#v", expected)
	}

	table, err := simopsEventsArrowTable("simops.telemetry.v1", events)
	if err != nil {
		t.Fatalf("build arrow table: %v", err)
	}
	defer table.Release()
	observed, err := observedIcebergOffsets(table)
	if err != nil {
		t.Fatalf("observed offsets: %v", err)
	}
	if _, ok := observed["simops.telemetry.v1/3/99"]; !ok {
		t.Fatalf("observed Redpanda coordinate missing from %#v", observed)
	}
}

func TestIcebergReadbackOffsetHelpersRejectNoTelemetry(t *testing.T) {
	_, err := expectedIcebergOffsets("simops.telemetry.v1", []SimopsEvent{{
		RunID:      "RUN-NO-TELEMETRY",
		EventType:  SimopsEventRunLifecycle,
		Lifecycle:  SimopsStreaming,
		OccurredAt: time.Now().UTC(),
	}})
	if err == nil {
		t.Fatalf("expected no-telemetry readback plan to fail")
	}
}

func TestIcebergAppendPropertiesCarryStableDeliveryAttemptIdentity(t *testing.T) {
	properties := simopsIcebergAppendProperties("RUN-DELIVERY-PROPERTIES", "simops.telemetry.v1", "delivery-stable")
	if properties["simops.delivery_attempt_id"] != "delivery-stable" {
		t.Fatalf("missing stable delivery attempt property: %#v", properties)
	}
}

func TestIcebergAppendRetriesCatalogConflictAfterFreshReloadWithStableAttemptID(t *testing.T) {
	writer := &IcebergGoSimopsArtifactWriter{}
	attempts := 0
	reloads := 0
	var delivered []string
	writer.appendTable = func(_ context.Context, _ *icetable.Table, _ arrow.Table, _ int64, properties iceberg.Properties) (*icetable.Table, error) {
		attempts++
		delivered = append(delivered, properties["simops.delivery_attempt_id"])
		if attempts == 1 {
			return nil, fmt.Errorf("catalog conflict: %w", icetable.ErrCommitFailed)
		}
		return nil, nil
	}
	writer.reloadTable = func(_ context.Context, _ icecatalog.Catalog) (*icetable.Table, error) {
		reloads++
		return nil, nil
	}

	properties := simopsIcebergAppendProperties("RUN-CONFLICT", "simops.telemetry.v1", "delivery-stable")
	if err := writer.appendWithCatalogRetry(context.Background(), nil, nil, nil, 1, properties); err != nil {
		t.Fatalf("append after conflict: %v", err)
	}
	if attempts != 2 || reloads != 1 {
		t.Fatalf("attempts=%d reloads=%d, want one conflict, reload, and retry", attempts, reloads)
	}
	if len(delivered) != 2 || delivered[0] != "delivery-stable" || delivered[1] != "delivery-stable" {
		t.Fatalf("retry changed delivery attempt identity: %#v", delivered)
	}

	writer.appendTable = func(context.Context, *icetable.Table, arrow.Table, int64, iceberg.Properties) (*icetable.Table, error) {
		return nil, errors.New("unknown append failure")
	}
	if err := writer.appendWithCatalogRetry(context.Background(), nil, nil, nil, 1, properties); err == nil {
		t.Fatal("non-conflict append failure must not be retried")
	}
}
