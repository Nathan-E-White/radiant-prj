package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type PostgresReactorTelemetryStore struct {
	db *sql.DB
}

func NewPostgresReactorTelemetryStore(dsn string) (*PostgresReactorTelemetryStore, error) {
	if err := requirePostgresDriver(); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := simopsSQLContext()
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping reactor telemetry postgres store: %w", err)
	}
	if err := ensureReactorTelemetrySchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate reactor telemetry schema: %w", err)
	}
	return &PostgresReactorTelemetryStore{db: db}, nil
}

func ensureReactorTelemetrySchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS reactor_telemetry_worker_sets (
			game_session_id TEXT NOT NULL,
			reactor_id TEXT NOT NULL,
			register_idempotency_key TEXT NOT NULL,
			worker_set JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (game_session_id, reactor_id),
			UNIQUE (game_session_id, register_idempotency_key)
		)
	`)
	return err
}

func (s *PostgresReactorTelemetryStore) GetWorkerSet(gameSessionID, reactorID string) (ReactorTelemetryWorkerSet, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return queryReactorTelemetrySet(ctx, s.db, `
		SELECT worker_set FROM reactor_telemetry_worker_sets
		WHERE game_session_id = $1 AND reactor_id = $2
	`, gameSessionID, reactorID)
}

func (s *PostgresReactorTelemetryStore) FindRegistration(gameSessionID, idempotencyKey string) (ReactorTelemetryWorkerSet, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return queryReactorTelemetrySet(ctx, s.db, `
		SELECT worker_set FROM reactor_telemetry_worker_sets
		WHERE game_session_id = $1 AND register_idempotency_key = $2
	`, gameSessionID, idempotencyKey)
}

func (s *PostgresReactorTelemetryStore) ListWorkerSets(gameSessionID string) ([]ReactorTelemetryWorkerSet, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	query := `SELECT worker_set FROM reactor_telemetry_worker_sets`
	args := []any{}
	if gameSessionID != "" {
		query += ` WHERE game_session_id = $1`
		args = append(args, gameSessionID)
	}
	query += ` ORDER BY game_session_id, reactor_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sets := []ReactorTelemetryWorkerSet{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var set ReactorTelemetryWorkerSet
		if err := json.Unmarshal(raw, &set); err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	return sets, rows.Err()
}

func (s *PostgresReactorTelemetryStore) SaveWorkerSet(set ReactorTelemetryWorkerSet) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	set = redactTelemetryCredentials(set)
	raw, err := json.Marshal(set)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO reactor_telemetry_worker_sets (
			game_session_id, reactor_id, register_idempotency_key, worker_set, updated_at
		) VALUES ($1,$2,$3,$4::jsonb,$5)
		ON CONFLICT (game_session_id, reactor_id) DO UPDATE
		SET register_idempotency_key = EXCLUDED.register_idempotency_key,
		    worker_set = EXCLUDED.worker_set,
		    updated_at = EXCLUDED.updated_at
	`, set.GameSessionID, set.ReactorID, set.RegisterIdempotency, raw, set.UpdatedAt)
	return err
}

func queryReactorTelemetrySet(ctx context.Context, db *sql.DB, query string, args ...any) (ReactorTelemetryWorkerSet, error) {
	var raw []byte
	if err := db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReactorTelemetryWorkerSet{}, ErrReactorTelemetryNotFound
		}
		return ReactorTelemetryWorkerSet{}, err
	}
	var set ReactorTelemetryWorkerSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return ReactorTelemetryWorkerSet{}, err
	}
	return set, nil
}
