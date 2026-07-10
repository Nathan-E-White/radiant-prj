//go:build !iceberggo

package gateway

import "fmt"

func NewIcebergGoSimopsArtifactWriter(cfg SimopsConfig, base *simopsArtifactWriterBase) (SimopsArtifactWriter, error) {
	return nil, fmt.Errorf("SIMOPS_ICEBERG_WRITER_MODE=iceberg-go requires building with -tags iceberggo")
}
