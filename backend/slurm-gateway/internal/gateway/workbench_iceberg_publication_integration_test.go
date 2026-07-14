//go:build iceberggo && postgresintegration

package gateway

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/apache/iceberg-go"
)

func TestTwinIcebergAppendDeduplicatesPublicationOnLegacySchema(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	var schema string
	if err := store.db.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("read isolated schema: %v", err)
	}
	parsed, err := url.Parse(strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN")))
	if err != nil {
		t.Fatalf("parse test DSN: %v", err)
	}
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = query.Encode()
	cfg := DefaultConfig().Workbench
	cfg.IcebergCatalogDSN = parsed.String()
	cfg.IcebergWarehouse = "file://" + t.TempDir()
	writer, err := NewWorkbenchIcebergWriter(cfg)
	if err != nil {
		t.Fatalf("new Iceberg writer: %v", err)
	}
	ctx := context.Background()
	cat, err := writer.loadCatalog(ctx)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	legacy := iceberg.NewSchema(0,
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
	if _, err := writer.loadOrCreateTable(ctx, cat, workbenchTwinIcebergIdentifier, legacy); err != nil {
		t.Fatalf("create legacy Twin table: %v", err)
	}
	publication := twinPublicationFixture("simops.results.v1", 2, 71)
	valueRows := 0
	for _, entity := range publication.State.Entities {
		valueRows += len(entity.Values)
	}
	for _, offset := range []int64{100, 101} {
		projection, err := ProjectTwinStatePublication(cfg.TwinStateTopic, 0, offset, publication)
		if err != nil {
			t.Fatalf("project offset %d: %v", offset, err)
		}
		if err := writer.AppendTwin(ctx, projection); err != nil {
			t.Fatalf("append offset %d: %v", offset, err)
		}
	}
	fresh, err := cat.LoadTable(ctx, workbenchTwinIcebergIdentifier)
	if err != nil {
		t.Fatalf("reload Twin table: %v", err)
	}
	if !icebergTwinPublicationSeen(fresh, publication.PublicationID) {
		t.Fatal("Twin Iceberg snapshot lost the semantic publication guard")
	}
	rows, err := fresh.Scan().ToArrowTable(ctx)
	if err != nil {
		t.Fatalf("read evolved Twin table: %v", err)
	}
	defer rows.Release()
	if got := int(rows.NumRows()); got != valueRows {
		t.Fatalf("duplicate semantic publication appended rows=%d want=%d", got, valueRows)
	}
}
