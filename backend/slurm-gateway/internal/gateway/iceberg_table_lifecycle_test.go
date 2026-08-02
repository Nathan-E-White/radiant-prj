//go:build iceberggo

package gateway

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
)

func TestIcebergTableLifecycleReturnsExistingTableWithoutCreatingNamespace(t *testing.T) {
	existing := &icetable.Table{}
	catalog := &icebergTableLifecycleFake{loadTable: existing}
	lifecycle := IcebergTableLifecycle{}

	table, err := lifecycle.LoadOrCreateTable(context.Background(), catalog, icetable.Identifier{"simops", "telemetry_frames"}, iceberg.NewSchema(0))
	if err != nil {
		t.Fatalf("load existing table: %v", err)
	}
	if table != existing {
		t.Fatal("lifecycle did not return the existing table")
	}
	if catalog.createNamespaceCalls != 0 || catalog.createTableCalls != 0 {
		t.Fatalf("existing table must not create namespace or table; got namespace=%d table=%d", catalog.createNamespaceCalls, catalog.createTableCalls)
	}
}

func TestIcebergTableLifecycleReloadsAfterConcurrentCreate(t *testing.T) {
	existing := &icetable.Table{}
	catalog := &icebergTableLifecycleFake{
		loadErrors:     []error{icecatalog.ErrNoSuchTable, nil},
		loadResults:    []*icetable.Table{nil, existing},
		createTableErr: icecatalog.ErrTableAlreadyExists,
	}
	lifecycle := IcebergTableLifecycle{}

	table, err := lifecycle.LoadOrCreateTable(context.Background(), catalog, icetable.Identifier{"scada", "measured_frames"}, iceberg.NewSchema(0))
	if err != nil {
		t.Fatalf("recover concurrent create: %v", err)
	}
	if table != existing {
		t.Fatal("lifecycle did not reload the table created by another writer")
	}
	if catalog.createNamespaceCalls != 1 || catalog.createTableCalls != 1 {
		t.Fatalf("expected one namespace and table creation attempt; got namespace=%d table=%d", catalog.createNamespaceCalls, catalog.createTableCalls)
	}
}

func TestIcebergTableLifecycleCreatesMissingNamespaceAndTable(t *testing.T) {
	created := &icetable.Table{}
	schema := iceberg.NewSchema(0)
	catalog := &icebergTableLifecycleFake{
		loadErrors:  []error{icecatalog.ErrNoSuchTable},
		createTable: created,
	}

	table, err := (IcebergTableLifecycle{}).LoadOrCreateTable(context.Background(), catalog, icetable.Identifier{"digital_twin", "state_values"}, schema)
	if err != nil {
		t.Fatalf("create missing table: %v", err)
	}
	if table != created {
		t.Fatal("lifecycle did not return the created table")
	}
	if !reflect.DeepEqual(catalog.createdNamespace, icetable.Identifier{"digital_twin"}) {
		t.Fatalf("created namespace = %v, want [digital_twin]", catalog.createdNamespace)
	}
	if catalog.createdSchema != schema {
		t.Fatal("lifecycle did not hand the writer-owned schema through unchanged")
	}
}

func TestIcebergTableLifecycleMapsIncompatibleCreateFailure(t *testing.T) {
	incompatible := errors.New("incompatible table schema")
	catalog := &icebergTableLifecycleFake{
		loadErrors:     []error{icecatalog.ErrNoSuchTable},
		createTableErr: incompatible,
	}
	_, err := (IcebergTableLifecycle{}).LoadOrCreateTable(context.Background(), catalog, icetable.Identifier{"simops", "telemetry_frames"}, iceberg.NewSchema(0))
	if !errors.Is(err, incompatible) {
		t.Fatalf("expected incompatible schema failure, got %v", err)
	}
	var lifecycleErr *IcebergTableLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.Failure != IcebergLifecycleTableCreateFailure {
		t.Fatalf("expected table-create lifecycle failure, got %#v", err)
	}
}

func TestIcebergWriterFamiliesShareCatalogConfiguration(t *testing.T) {
	config := DefaultConfig()
	simops := config.Simops
	workbench := config.Workbench
	simops.IcebergCatalogDSN = "postgres://catalog"
	workbench.IcebergCatalogDSN = simops.IcebergCatalogDSN
	workbench.IcebergWarehouse = simops.IcebergWarehouse
	workbench.IcebergS3Endpoint = simops.IcebergS3Endpoint
	workbench.IcebergS3Region = simops.IcebergS3Region
	workbench.IcebergS3AccessKeyID = simops.IcebergS3AccessKeyID
	workbench.IcebergS3SecretKey = simops.IcebergS3SecretKey

	if !reflect.DeepEqual(simopsIcebergLifecycle(simops).catalogProperties(), workbenchIcebergLifecycle(workbench).catalogProperties()) {
		t.Fatal("SimOps and Workbench catalog properties drifted")
	}
}

func TestIcebergTableLifecycleMapsCatalogLoadFailure(t *testing.T) {
	catalogFailure := errors.New("catalog unavailable")
	lifecycle := IcebergTableLifecycle{
		Configuration: IcebergCatalogConfiguration{DSN: "postgres://catalog", Warehouse: "file:///warehouse"},
		loadCatalog: func(context.Context, string, iceberg.Properties) (icecatalog.Catalog, error) {
			return nil, catalogFailure
		},
	}

	_, err := lifecycle.LoadCatalog(context.Background(), "simops")
	if !errors.Is(err, catalogFailure) {
		t.Fatalf("expected catalog failure, got %v", err)
	}
	var lifecycleErr *IcebergTableLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.Failure != IcebergLifecycleCatalogFailure {
		t.Fatalf("expected catalog lifecycle failure, got %#v", err)
	}
}

func TestIcebergWriterAdaptersHandOffTheirOwnSchemas(t *testing.T) {
	config := DefaultConfig()
	config.Simops.IcebergCatalogDSN = "postgres://catalog"
	config.Workbench.IcebergCatalogDSN = config.Simops.IcebergCatalogDSN
	config.Workbench.IcebergWarehouse = config.Simops.IcebergWarehouse

	tests := []struct {
		name           string
		load           func(*icebergTableLifecycleFake) error
		wantIdentifier icetable.Identifier
		wantFirstField string
		wantFieldCount int
	}{
		{
			name: "SimOps telemetry",
			load: func(catalog *icebergTableLifecycleFake) error {
				_, err := (&IcebergGoSimopsArtifactWriter{cfg: config.Simops}).loadOrCreateTable(context.Background(), catalog)
				return err
			},
			wantIdentifier: simopsIcebergIdentifier,
			wantFirstField: "received_at",
			wantFieldCount: 16,
		},
		{
			name: "Workbench measured state",
			load: func(catalog *icebergTableLifecycleFake) error {
				_, err := (&WorkbenchIcebergWriter{cfg: config.Workbench}).loadOrCreateTable(context.Background(), catalog, workbenchScadaIcebergIdentifier, workbenchScadaIcebergSchema())
				return err
			},
			wantIdentifier: workbenchScadaIcebergIdentifier,
			wantFirstField: "observed_at",
			wantFieldCount: 14,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := &icebergTableLifecycleFake{loadErrors: []error{icecatalog.ErrNoSuchTable}, createTable: &icetable.Table{}}
			if err := test.load(catalog); err != nil {
				t.Fatalf("hand off writer schema: %v", err)
			}
			if !reflect.DeepEqual(catalog.createdIdentifier, test.wantIdentifier) {
				t.Fatalf("writer table identifier = %v, want %v", catalog.createdIdentifier, test.wantIdentifier)
			}
			if catalog.createdSchema == nil || len(catalog.createdSchema.Fields()) != test.wantFieldCount || catalog.createdSchema.Fields()[0].Name != test.wantFirstField {
				t.Fatalf("writer schema handoff = %#v, want %d fields beginning with %q", catalog.createdSchema, test.wantFieldCount, test.wantFirstField)
			}
		})
	}
}

func TestIcebergWriterAppendHandoffsPreserveStreamMetadata(t *testing.T) {
	simopsProperties := simopsIcebergAppendProperties("RUN-137", "simops.telemetry.v1")
	if simopsProperties["simops.run_id"] != "RUN-137" || simopsProperties["simops.batch_topic"] != "simops.telemetry.v1" {
		t.Fatalf("SimOps append handoff = %v", simopsProperties)
	}

	workbenchProperties := workbenchIcebergAppendProperties("scada.telemetry.v1")
	if workbenchProperties["workbench.topic"] != "scada.telemetry.v1" {
		t.Fatalf("Workbench append handoff = %v", workbenchProperties)
	}
}

type icebergTableLifecycleFake struct {
	loadTable            *icetable.Table
	loadResults          []*icetable.Table
	loadErrors           []error
	createNamespaceErr   error
	createTableErr       error
	createTable          *icetable.Table
	createdNamespace     icetable.Identifier
	createdIdentifier    icetable.Identifier
	createdSchema        *iceberg.Schema
	createNamespaceCalls int
	createTableCalls     int
}

func (f *icebergTableLifecycleFake) LoadTable(context.Context, icetable.Identifier) (*icetable.Table, error) {
	if len(f.loadErrors) == 0 {
		return f.loadTable, nil
	}
	err := f.loadErrors[0]
	f.loadErrors = f.loadErrors[1:]
	var table *icetable.Table
	if len(f.loadResults) > 0 {
		table = f.loadResults[0]
		f.loadResults = f.loadResults[1:]
	}
	return table, err
}

func (f *icebergTableLifecycleFake) CreateNamespace(_ context.Context, namespace icetable.Identifier, _ iceberg.Properties) error {
	f.createNamespaceCalls++
	f.createdNamespace = namespace
	return f.createNamespaceErr
}

func (f *icebergTableLifecycleFake) CreateTable(_ context.Context, identifier icetable.Identifier, schema *iceberg.Schema, _ ...icecatalog.CreateTableOpt) (*icetable.Table, error) {
	f.createTableCalls++
	f.createdIdentifier = identifier
	f.createdSchema = schema
	return f.createTable, f.createTableErr
}

func TestIcebergTableLifecyclePreservesCatalogFailure(t *testing.T) {
	catalogFailure := errors.New("catalog unavailable")
	catalog := &icebergTableLifecycleFake{loadErrors: []error{catalogFailure}}
	_, err := (IcebergTableLifecycle{}).LoadOrCreateTable(context.Background(), catalog, icetable.Identifier{"simops", "telemetry_frames"}, iceberg.NewSchema(0))
	if !errors.Is(err, catalogFailure) {
		t.Fatalf("expected stable catalog failure, got %v", err)
	}
}
