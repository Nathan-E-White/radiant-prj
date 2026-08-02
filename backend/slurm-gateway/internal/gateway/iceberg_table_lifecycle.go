//go:build iceberggo

package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	_ "github.com/apache/iceberg-go/catalog/sql"
	sqlcatalog "github.com/apache/iceberg-go/catalog/sql"
	iceio "github.com/apache/iceberg-go/io"
	_ "github.com/apache/iceberg-go/io/gocloud"
	icetable "github.com/apache/iceberg-go/table"
)

// IcebergCatalogConfiguration is the shared connection configuration for a
// trusted Iceberg writer. Stream adapters retain authority over their table
// definitions and append inputs.
type IcebergCatalogConfiguration struct {
	DSN           string
	Warehouse     string
	S3Endpoint    string
	S3Region      string
	S3AccessKeyID string
	S3SecretKey   string
}

// IcebergLifecycleFailure identifies a stable table-lifecycle outcome.
type IcebergLifecycleFailure string

const (
	IcebergLifecycleConfigurationFailure IcebergLifecycleFailure = "configuration"
	IcebergLifecycleCatalogFailure       IcebergLifecycleFailure = "catalog"
	IcebergLifecycleTableLoadFailure     IcebergLifecycleFailure = "table-load"
	IcebergLifecycleNamespaceFailure     IcebergLifecycleFailure = "namespace"
	IcebergLifecycleTableCreateFailure   IcebergLifecycleFailure = "table-create"
	IcebergLifecycleRaceRecoveryFailure  IcebergLifecycleFailure = "race-recovery"
)

// IcebergTableLifecycleError preserves the source failure while exposing one
// shared, observable lifecycle category to both writer families.
type IcebergTableLifecycleError struct {
	Failure    IcebergLifecycleFailure
	Identifier icetable.Identifier
	Err        error
}

func (e *IcebergTableLifecycleError) Error() string {
	if len(e.Identifier) == 0 {
		return fmt.Sprintf("iceberg %s failure: %v", e.Failure, e.Err)
	}
	return fmt.Sprintf("iceberg %s failure for %s: %v", e.Failure, strings.Join(e.Identifier, "."), e.Err)
}

func (e *IcebergTableLifecycleError) Unwrap() error { return e.Err }

// IcebergTableLifecycle owns catalog mechanics shared by trusted data-plane
// writers. It intentionally accepts writer-owned schemas and identifiers.
type IcebergTableLifecycle struct {
	Configuration IcebergCatalogConfiguration
	loadCatalog   func(context.Context, string, iceberg.Properties) (icecatalog.Catalog, error)
}

type icebergTableLifecycleCatalog interface {
	LoadTable(context.Context, icetable.Identifier) (*icetable.Table, error)
	CreateNamespace(context.Context, icetable.Identifier, iceberg.Properties) error
	CreateTable(context.Context, icetable.Identifier, *iceberg.Schema, ...icecatalog.CreateTableOpt) (*icetable.Table, error)
}

func (l IcebergTableLifecycle) Validate() error {
	if strings.TrimSpace(l.Configuration.DSN) == "" {
		return &IcebergTableLifecycleError{Failure: IcebergLifecycleConfigurationFailure, Err: errors.New("catalog DSN is required")}
	}
	if strings.TrimSpace(l.Configuration.Warehouse) == "" {
		return &IcebergTableLifecycleError{Failure: IcebergLifecycleConfigurationFailure, Err: errors.New("warehouse is required")}
	}
	return nil
}

func (l IcebergTableLifecycle) LoadCatalog(ctx context.Context, name string) (icecatalog.Catalog, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	loadCatalog := l.loadCatalog
	if loadCatalog == nil {
		loadCatalog = icecatalog.Load
	}
	cat, err := loadCatalog(ctx, strings.TrimSpace(name), l.catalogProperties())
	if err != nil {
		return nil, &IcebergTableLifecycleError{Failure: IcebergLifecycleCatalogFailure, Err: err}
	}
	return cat, nil
}

func (l IcebergTableLifecycle) LoadOrCreateTable(ctx context.Context, cat icebergTableLifecycleCatalog, identifier icetable.Identifier, schema *iceberg.Schema) (*icetable.Table, error) {
	table, err := cat.LoadTable(ctx, identifier)
	if err == nil {
		return table, nil
	}
	if !errors.Is(err, icecatalog.ErrNoSuchTable) {
		return nil, l.failure(IcebergLifecycleTableLoadFailure, identifier, err)
	}

	namespace := icecatalog.NamespaceFromIdent(identifier)
	if err := cat.CreateNamespace(ctx, namespace, iceberg.Properties{}); err != nil && !errors.Is(err, icecatalog.ErrNamespaceAlreadyExists) {
		return nil, l.failure(IcebergLifecycleNamespaceFailure, identifier, err)
	}

	table, err = cat.CreateTable(ctx, identifier, schema, icecatalog.WithProperties(l.tableProperties()))
	if err == nil {
		return table, nil
	}
	if !errors.Is(err, icecatalog.ErrTableAlreadyExists) {
		return nil, l.failure(IcebergLifecycleTableCreateFailure, identifier, err)
	}
	table, err = cat.LoadTable(ctx, identifier)
	if err != nil {
		return nil, l.failure(IcebergLifecycleRaceRecoveryFailure, identifier, err)
	}
	return table, nil
}

func (l IcebergTableLifecycle) failure(failure IcebergLifecycleFailure, identifier icetable.Identifier, err error) error {
	return &IcebergTableLifecycleError{Failure: failure, Identifier: identifier, Err: err}
}

func (l IcebergTableLifecycle) catalogProperties() iceberg.Properties {
	properties := iceberg.Properties{
		"type":                "sql",
		"uri":                 strings.TrimSpace(l.Configuration.DSN),
		sqlcatalog.DriverKey:  "pgx",
		sqlcatalog.DialectKey: string(sqlcatalog.Postgres),
		"init_catalog_tables": "true",
		"warehouse":           strings.TrimRight(strings.TrimSpace(l.Configuration.Warehouse), "/"),
	}
	for key, value := range l.s3Properties() {
		properties[key] = value
	}
	return properties
}

func (l IcebergTableLifecycle) tableProperties() iceberg.Properties {
	properties := iceberg.Properties{icetable.PropertyFormatVersion: "2"}
	for key, value := range l.s3Properties() {
		properties[key] = value
	}
	return properties
}

func (l IcebergTableLifecycle) s3Properties() iceberg.Properties {
	properties := iceberg.Properties{}
	if endpoint := strings.TrimSpace(l.Configuration.S3Endpoint); endpoint != "" {
		properties[iceio.S3EndpointURL] = endpoint
	}
	if region := strings.TrimSpace(l.Configuration.S3Region); region != "" {
		properties[iceio.S3Region] = region
		properties[iceio.S3ClientRegion] = region
	}
	if accessKeyID := strings.TrimSpace(l.Configuration.S3AccessKeyID); accessKeyID != "" {
		properties[iceio.S3AccessKeyID] = accessKeyID
	}
	if secretKey := strings.TrimSpace(l.Configuration.S3SecretKey); secretKey != "" {
		properties[iceio.S3SecretAccessKey] = secretKey
	}
	return properties
}
