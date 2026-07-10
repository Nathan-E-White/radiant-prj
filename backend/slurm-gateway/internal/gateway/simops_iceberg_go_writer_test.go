//go:build iceberggo

package gateway

import (
	"encoding/json"
	"testing"
	"time"
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
