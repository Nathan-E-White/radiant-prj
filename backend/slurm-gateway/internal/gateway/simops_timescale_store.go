package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TimescaleTelemetryStore struct {
	db *sql.DB
}

func NewTimescaleTelemetryStore(dsn string) (*TimescaleTelemetryStore, error) {
	if err := requirePostgresDriver(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("timescale telemetry store requires SIMOPS_POSTGRES_DSN")
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
		return nil, fmt.Errorf("ping timescale telemetry store: %w", err)
	}
	return &TimescaleTelemetryStore{db: db}, nil
}

func (s *TimescaleTelemetryStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *TimescaleTelemetryStore) SaveProjection(ctx context.Context, consumerName string, projection SimopsTelemetryProjection) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("timescale telemetry store is not initialized")
	}
	consumerName = strings.TrimSpace(consumerName)
	if consumerName == "" {
		return false, fmt.Errorf("consumer name is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO simops_processed_messages (
			consumer_name, redpanda_topic, redpanda_partition, redpanda_offset, processed_at
		)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT DO NOTHING
	`, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	frame := string(projection.Frame)
	if !json.Valid(projection.Frame) {
		return false, fmt.Errorf("telemetry projection frame must be valid JSON")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simops_telemetry_frames (
			received_at, emitted_at, run_id, scenario_id, worker_id, worker_kind,
			sequence, payload_type, quality, source_lag_ms, collector_lag_ms,
			dropped_frame_count, frame, redpanda_topic, redpanda_partition, redpanda_offset
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15,$16)
	`, projection.ReceivedAt, projection.EmittedAt, projection.RunID, projection.ScenarioID, projection.WorkerID,
		string(projection.WorkerKind), int64(projection.Sequence), projection.PayloadType, nullableString(projection.Quality),
		projection.SourceLagMs, projection.CollectorLagMs, projection.DroppedFrameCount, frame, projection.RedpandaTopic,
		projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
		return false, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simops_consumer_offsets (
			consumer_name, redpanda_topic, redpanda_partition, redpanda_offset, updated_at
		)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (consumer_name, redpanda_topic, redpanda_partition)
		DO UPDATE SET
			redpanda_offset = GREATEST(simops_consumer_offsets.redpanda_offset, EXCLUDED.redpanda_offset),
			updated_at = now()
	`, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
