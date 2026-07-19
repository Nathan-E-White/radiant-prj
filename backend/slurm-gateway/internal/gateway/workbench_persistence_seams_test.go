package gateway

import (
	"testing"
	"time"
)

type focusedScadaProjectionStore struct{}

func (focusedScadaProjectionStore) SaveScadaProjection(string, ScadaProjection) (bool, error) {
	return true, nil
}

type focusedResultProjectionStore struct{}

func (focusedResultProjectionStore) SaveResultProjection(string, SimopsResultProjection) (bool, error) {
	return true, nil
}

type focusedTwinProjectionStore struct{}

func (focusedTwinProjectionStore) SaveTwinStateProjection(string, TwinStateProjection) (bool, error) {
	return true, nil
}

type focusedMeasuredRetentionStore struct{}

func (focusedMeasuredRetentionStore) PruneDynamicMeasured(time.Time) error { return nil }

func TestWorkbenchPersistenceSeamsRequireOnlyOwnedBehavior(t *testing.T) {
	var _ WorkbenchScadaProjectionPersistence = focusedScadaProjectionStore{}
	var _ WorkbenchResultProjectionPersistence = focusedResultProjectionStore{}
	var _ WorkbenchTwinProjectionPersistence = focusedTwinProjectionStore{}
	var _ WorkbenchDynamicMeasuredRetention = focusedMeasuredRetentionStore{}
	var _ WorkbenchStore = (*InMemoryWorkbenchStore)(nil)
	var _ WorkbenchStore = (*PostgresWorkbenchStore)(nil)
}
