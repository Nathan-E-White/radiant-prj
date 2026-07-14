package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const configuredDataFlushAdvisoryLock int64 = 0x43444601

type PostgresConfiguredDataFlushRepository struct {
	db *sql.DB
}

func NewPostgresConfiguredDataFlushRepository(dsn string) (*PostgresConfiguredDataFlushRepository, error) {
	if err := requirePostgresDriver(); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(10 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping configured data flush store: %w", err)
	}
	return &PostgresConfiguredDataFlushRepository{db: db}, nil
}

func (r *PostgresConfiguredDataFlushRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *PostgresConfiguredDataFlushRepository) Inspect(ctx context.Context) (ConfiguredDataFlushInventory, error) {
	if r == nil || r.db == nil {
		return ConfiguredDataFlushInventory{}, fmt.Errorf("configured data flush database is required")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return ConfiguredDataFlushInventory{}, err
	}
	defer func() { _ = tx.Rollback() }()
	inventory, err := inspectConfiguredDataFlush(ctx, tx, false)
	if err != nil {
		return ConfiguredDataFlushInventory{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConfiguredDataFlushInventory{}, err
	}
	return inventory, nil
}

func (r *PostgresConfiguredDataFlushRepository) Apply(ctx context.Context, reviewed ConfiguredDataFlushPlan) (ConfiguredDataFlushResult, error) {
	if r == nil || r.db == nil {
		return ConfiguredDataFlushResult{}, fmt.Errorf("configured data flush database is required")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, configuredDataFlushAdvisoryLock); err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	inventory, err := inspectConfiguredDataFlush(ctx, tx, true)
	if err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	current := buildConfiguredDataFlushPlan(inventory)
	if !current.Ready {
		return ConfiguredDataFlushResult{}, fmt.Errorf("%w: %v", ErrConfiguredDataFlushBlocked, current.Blockers)
	}
	if current.PlanID != reviewed.PlanID || current.CurrentGeneration != reviewed.CurrentGeneration {
		return ConfiguredDataFlushResult{}, ErrConfiguredDataFlushStalePlan
	}

	deleted := make(map[string]int64, len(configuredDataFlushTargetRegistry))
	for _, target := range configuredDataFlushTargetRegistry {
		result, err := tx.ExecContext(ctx, target.DeleteSQL)
		if err != nil {
			return ConfiguredDataFlushResult{}, fmt.Errorf("clear %s: %w", target.Name, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return ConfiguredDataFlushResult{}, fmt.Errorf("count cleared %s: %w", target.Name, err)
		}
		deleted[target.Name] = rows
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE workbench_snapshot_generation
		SET generation = generation + 1, updated_at = now()
		WHERE singleton = TRUE AND generation = $1
	`, reviewed.CurrentGeneration)
	if err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	if rows != 1 {
		return ConfiguredDataFlushResult{}, ErrConfiguredDataFlushStalePlan
	}
	if err := tx.Commit(); err != nil {
		return ConfiguredDataFlushResult{}, err
	}
	return ConfiguredDataFlushResult{
		Operation: "configured-data-flush", PlanID: reviewed.PlanID,
		PreviousGeneration: reviewed.CurrentGeneration, Generation: reviewed.NextGeneration,
		DeletedRows: deleted,
	}, nil
}

type configuredDataFlushQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func inspectConfiguredDataFlush(ctx context.Context, queryer configuredDataFlushQueryer, lockGeneration bool) (ConfiguredDataFlushInventory, error) {
	inventory := ConfiguredDataFlushInventory{Rows: make(map[string]int64, len(configuredDataFlushTargetRegistry))}
	generationQuery := `SELECT generation FROM workbench_snapshot_generation WHERE singleton = TRUE`
	if lockGeneration {
		generationQuery += ` FOR UPDATE`
	}
	if err := queryer.QueryRowContext(ctx, generationQuery).Scan(&inventory.Generation); err != nil {
		return ConfiguredDataFlushInventory{}, fmt.Errorf("read Workbench generation: %w", err)
	}
	if err := queryer.QueryRowContext(ctx, `SELECT txid_snapshot_xmax(txid_current_snapshot())::text`).Scan(&inventory.Revision); err != nil {
		return ConfiguredDataFlushInventory{}, fmt.Errorf("read database revision: %w", err)
	}
	for _, target := range configuredDataFlushTargetRegistry {
		var count int64
		if err := queryer.QueryRowContext(ctx, target.CountSQL).Scan(&count); err != nil {
			return ConfiguredDataFlushInventory{}, fmt.Errorf("count %s: %w", target.Name, err)
		}
		inventory.Rows[target.Name] = count
	}
	activeRuns, err := queryer.QueryContext(ctx, `
		SELECT run_id FROM simops_runs
		WHERE lifecycle NOT IN ('complete', 'failed', 'stopped')
		ORDER BY run_id
	`)
	if err != nil {
		return ConfiguredDataFlushInventory{}, fmt.Errorf("list active SimOps Runs: %w", err)
	}
	defer activeRuns.Close()
	for activeRuns.Next() {
		var runID string
		if err := activeRuns.Scan(&runID); err != nil {
			return ConfiguredDataFlushInventory{}, err
		}
		inventory.ActiveRunIDs = append(inventory.ActiveRunIDs, runID)
	}
	if err := activeRuns.Err(); err != nil {
		return ConfiguredDataFlushInventory{}, err
	}
	activeSets, err := queryer.QueryContext(ctx, `
		SELECT worker_set->>'setId' FROM reactor_telemetry_worker_sets
		WHERE COALESCE(worker_set->>'lifecycle', '') <> 'removed'
		ORDER BY worker_set->>'setId'
	`)
	if err != nil {
		return ConfiguredDataFlushInventory{}, fmt.Errorf("list active Reactor Telemetry Worker Sets: %w", err)
	}
	defer activeSets.Close()
	for activeSets.Next() {
		var setID string
		if err := activeSets.Scan(&setID); err != nil {
			return ConfiguredDataFlushInventory{}, err
		}
		inventory.ActiveWorkerSetIDs = append(inventory.ActiveWorkerSetIDs, setID)
	}
	if err := activeSets.Err(); err != nil {
		return ConfiguredDataFlushInventory{}, err
	}
	return inventory, nil
}
