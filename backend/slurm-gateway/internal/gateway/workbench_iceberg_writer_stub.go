//go:build !iceberggo

package gateway

import (
	"context"
	"fmt"
)

type WorkbenchIcebergWriter struct{}

func NewWorkbenchIcebergWriter(cfg WorkbenchConfig) (*WorkbenchIcebergWriter, error) {
	return nil, fmt.Errorf("workbench Iceberg writer requires building with -tags iceberggo")
}

func (w *WorkbenchIcebergWriter) AppendScada(ctx context.Context, projection ScadaProjection) error {
	return fmt.Errorf("workbench Iceberg writer requires building with -tags iceberggo")
}

func (w *WorkbenchIcebergWriter) AppendResult(ctx context.Context, projection SimopsResultProjection) error {
	return fmt.Errorf("workbench Iceberg writer requires building with -tags iceberggo")
}

func (w *WorkbenchIcebergWriter) AppendTwin(ctx context.Context, projection TwinStateProjection) error {
	return fmt.Errorf("workbench Iceberg writer requires building with -tags iceberggo")
}
