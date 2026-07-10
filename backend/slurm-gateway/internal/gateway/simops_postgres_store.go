package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const simopsPostgresTimeout = 5 * time.Second

type PostgresSimopsStore struct {
	db *sql.DB
}

func NewPostgresSimopsStore(dsn string) (*PostgresSimopsStore, error) {
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
		return nil, fmt.Errorf("ping simops postgres store: %w", err)
	}
	return &PostgresSimopsStore{db: db}, nil
}

func (s *PostgresSimopsStore) CreateRun(record SimopsRunRecord, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) (SimopsRunRecord, bool, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SimopsRunRecord{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO simops_runs (
			run_id, scenario_id, lifecycle, source, work_script, launch_mode,
			runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
			created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT DO NOTHING
	`,
		record.RunID,
		record.ScenarioID,
		string(record.Lifecycle),
		record.Source,
		record.WorkScript,
		record.LaunchMode,
		record.RuntimeLimitSec,
		nullableString(record.IdempotencyKey),
		record.SubmittedBy,
		record.IngestToken,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return SimopsRunRecord{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return SimopsRunRecord{}, false, err
	}
	if affected == 0 {
		if record.IdempotencyKey != "" {
			existing, err := s.GetRunByIdempotency(record.SubmittedBy, record.IdempotencyKey)
			if err == nil {
				return existing, false, nil
			}
		}
		return SimopsRunRecord{}, false, ErrSimopsConflict
	}

	for _, worker := range workers {
		labels, err := marshalLabels(worker.Labels)
		if err != nil {
			return SimopsRunRecord{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simops_workers (
				run_id, worker_id, worker_kind, lifecycle, launch_mode,
				endpoint, frames, labels, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9)
		`,
			record.RunID,
			worker.WorkerID,
			string(worker.WorkerKind),
			string(worker.Lifecycle),
			worker.LaunchMode,
			nullableString(worker.Endpoint),
			worker.Frames,
			labels,
			worker.UpdatedAt,
		); err != nil {
			return SimopsRunRecord{}, false, err
		}
	}

	for _, command := range commands {
		metadata, err := marshalLabels(command.Metadata)
		if err != nil {
			return SimopsRunRecord{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simops_spool_commands (
				command_id, run_id, worker_id, mode, state, message, metadata, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)
		`,
			command.CommandID,
			command.RunID,
			command.WorkerID,
			command.Mode,
			string(command.State),
			command.Message,
			metadata,
			command.CreatedAt,
			command.UpdatedAt,
		); err != nil {
			return SimopsRunRecord{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return SimopsRunRecord{}, false, err
	}
	return record, true, nil
}

func (s *PostgresSimopsStore) SaveLaunch(runID string, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var existingRunID string
	if err := tx.QueryRowContext(ctx, `SELECT run_id FROM simops_runs WHERE run_id = $1`, runID).Scan(&existingRunID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSimopsRunNotFound
		}
		return err
	}

	for _, worker := range workers {
		labels, err := marshalLabels(worker.Labels)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simops_workers (
				run_id, worker_id, worker_kind, lifecycle, launch_mode,
				endpoint, frames, labels, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9)
			ON CONFLICT (run_id, worker_id) DO UPDATE
			SET worker_kind = EXCLUDED.worker_kind,
			    lifecycle = CASE
			      WHEN simops_workers.frames > EXCLUDED.frames THEN simops_workers.lifecycle
			      ELSE EXCLUDED.lifecycle
			    END,
			    launch_mode = EXCLUDED.launch_mode,
			    endpoint = EXCLUDED.endpoint,
			    labels = EXCLUDED.labels,
			    updated_at = GREATEST(simops_workers.updated_at, EXCLUDED.updated_at)
		`,
			runID,
			worker.WorkerID,
			string(worker.WorkerKind),
			string(worker.Lifecycle),
			worker.LaunchMode,
			nullableString(worker.Endpoint),
			worker.Frames,
			labels,
			worker.UpdatedAt,
		); err != nil {
			return err
		}
	}

	for _, command := range commands {
		metadata, err := marshalLabels(command.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simops_spool_commands (
				command_id, run_id, worker_id, mode, state, message, metadata, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)
			ON CONFLICT (command_id) DO UPDATE
			SET state = EXCLUDED.state,
			    message = EXCLUDED.message,
			    metadata = EXCLUDED.metadata,
			    updated_at = EXCLUDED.updated_at
		`,
			command.CommandID,
			runID,
			command.WorkerID,
			command.Mode,
			string(command.State),
			command.Message,
			metadata,
			command.CreatedAt,
			command.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresSimopsStore) GetRunByIdempotency(identity string, key string) (SimopsRunRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return s.scanRun(ctx, `
		SELECT run_id, scenario_id, lifecycle, source, work_script, launch_mode,
		       runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
		       created_at, updated_at
		FROM simops_runs
		WHERE submitted_by = $1 AND idempotency_key = $2
	`, identity, key)
}

func (s *PostgresSimopsStore) GetRun(runID string) (SimopsRunRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return s.scanRun(ctx, `
		SELECT run_id, scenario_id, lifecycle, source, work_script, launch_mode,
		       runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
		       created_at, updated_at
		FROM simops_runs
		WHERE run_id = $1
	`, runID)
}

func (s *PostgresSimopsStore) ListWorkers(runID string) ([]SimopsWorkerRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, worker_id, worker_kind, lifecycle, launch_mode,
		       endpoint, frames, labels, updated_at
		FROM simops_workers
		WHERE run_id = $1
		ORDER BY worker_id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workers := []SimopsWorkerRecord{}
	for rows.Next() {
		var worker SimopsWorkerRecord
		var kind, lifecycle string
		var endpoint sql.NullString
		var labelsRaw []byte
		if err := rows.Scan(&worker.RunID, &worker.WorkerID, &kind, &lifecycle, &worker.LaunchMode, &endpoint, &worker.Frames, &labelsRaw, &worker.UpdatedAt); err != nil {
			return nil, err
		}
		worker.WorkerKind = SimopsWorkerKind(kind)
		worker.Lifecycle = SimopsLifecycle(lifecycle)
		worker.Endpoint = endpoint.String
		if endpoint.Valid {
			worker.Endpoint = endpoint.String
		}
		if len(labelsRaw) > 0 {
			if err := json.Unmarshal(labelsRaw, &worker.Labels); err != nil {
				return nil, err
			}
		}
		workers = append(workers, worker)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(workers) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return workers, nil
}

func (s *PostgresSimopsStore) ListCommands(runID string) ([]SimopsSpoolCommand, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT command_id, run_id, worker_id, mode, state, message, metadata, created_at, updated_at
		FROM simops_spool_commands
		WHERE run_id = $1
		ORDER BY created_at, command_id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commands := []SimopsSpoolCommand{}
	for rows.Next() {
		var command SimopsSpoolCommand
		var state string
		var metadataRaw []byte
		if err := rows.Scan(&command.CommandID, &command.RunID, &command.WorkerID, &command.Mode, &state, &command.Message, &metadataRaw, &command.CreatedAt, &command.UpdatedAt); err != nil {
			return nil, err
		}
		command.State = SimopsLifecycle(state)
		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &command.Metadata); err != nil {
				return nil, err
			}
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(commands) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return commands, nil
}

func (s *PostgresSimopsStore) ListArtifacts(runID string) ([]SimopsArtifactRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT artifact_id, run_id, kind, media_type, location, iceberg_table, status, created_at
		FROM simops_artifacts
		WHERE run_id = $1
		ORDER BY created_at, artifact_id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artifacts := []SimopsArtifactRecord{}
	for rows.Next() {
		var artifact SimopsArtifactRecord
		var table sql.NullString
		if err := rows.Scan(&artifact.ArtifactID, &artifact.RunID, &artifact.Kind, &artifact.MediaType, &artifact.Location, &table, &artifact.Status, &artifact.CreatedAt); err != nil {
			return nil, err
		}
		if table.Valid {
			artifact.IcebergTable = table.String
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(artifacts) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return artifacts, nil
}

func (s *PostgresSimopsStore) UpdateRunLifecycle(runID string, lifecycle SimopsLifecycle) (SimopsRunRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	row := s.db.QueryRowContext(ctx, `
		UPDATE simops_runs
		SET lifecycle = $2, updated_at = $3
		WHERE run_id = $1
		RETURNING run_id, scenario_id, lifecycle, source, work_script, launch_mode,
		          runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
		          created_at, updated_at
	`, runID, string(lifecycle), time.Now().UTC())
	return scanSimopsRun(row)
}

func (s *PostgresSimopsStore) UpdateWorkerFrames(runID string, workerID string, lifecycle SimopsLifecycle, framesDelta int) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `
		UPDATE simops_workers
		SET lifecycle = $3, frames = frames + $4, updated_at = $5
		WHERE run_id = $1 AND worker_id = $2
	`, runID, workerID, string(lifecycle), framesDelta, time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrSimopsRunNotFound)
}

func (s *PostgresSimopsStore) SaveArtifact(record SimopsArtifactRecord) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	status := record.Status
	if status == "" {
		status = SimopsArtifactStatusReceived
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO simops_artifacts (
			artifact_id, run_id, kind, media_type, status, location, iceberg_table, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (artifact_id) DO UPDATE
		SET kind = EXCLUDED.kind,
		    media_type = EXCLUDED.media_type,
		    status = EXCLUDED.status,
		    location = EXCLUDED.location,
		    iceberg_table = EXCLUDED.iceberg_table,
		    created_at = EXCLUDED.created_at
	`, record.ArtifactID, record.RunID, record.Kind, record.MediaType, status, record.Location, nullableString(record.IcebergTable), record.CreatedAt)
	return err
}

func (s *PostgresSimopsStore) SaveEvent(event SimopsEvent) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	var frame any
	if len(event.Frame) > 0 {
		frame = string(event.Frame)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO simops_events (run_id, worker_id, event_type, lifecycle, frame, occurred_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6)
	`, event.RunID, nullableString(event.WorkerID), event.EventType, nullableString(string(event.Lifecycle)), frame, occurredAt)
	return err
}

func (s *PostgresSimopsStore) ListEvents(runID string) ([]SimopsEvent, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, worker_id, event_type, lifecycle, frame, occurred_at
		FROM simops_events
		WHERE run_id = $1
		ORDER BY event_id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []SimopsEvent{}
	for rows.Next() {
		var event SimopsEvent
		var workerID, lifecycle sql.NullString
		var frame []byte
		if err := rows.Scan(&event.RunID, &workerID, &event.EventType, &lifecycle, &frame, &event.OccurredAt); err != nil {
			return nil, err
		}
		if workerID.Valid {
			event.WorkerID = workerID.String
		}
		if lifecycle.Valid {
			event.Lifecycle = SimopsLifecycle(lifecycle.String)
		}
		if len(frame) > 0 {
			event.Frame = json.RawMessage(frame)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (s *PostgresSimopsStore) UpdateArtifactStatus(runID string, artifactID string, status string) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `
		UPDATE simops_artifacts
		SET status = $3
		WHERE run_id = $1 AND artifact_id = $2
	`, runID, artifactID, status)
	if err != nil {
		return err
	}
	return requireAffected(result, ErrSimopsArtifactNotFound)
}

func (s *PostgresSimopsStore) ActiveRunCount() int {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM simops_runs
		WHERE lifecycle IN ('created','starting','streaming','degraded')
	`).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (s *PostgresSimopsStore) scanRun(ctx context.Context, query string, args ...any) (SimopsRunRecord, error) {
	return scanSimopsRun(s.db.QueryRowContext(ctx, query, args...))
}

type simopsRunScanner interface {
	Scan(dest ...any) error
}

func scanSimopsRun(row simopsRunScanner) (SimopsRunRecord, error) {
	var record SimopsRunRecord
	var lifecycle string
	var idempotency sql.NullString
	if err := row.Scan(
		&record.RunID,
		&record.ScenarioID,
		&lifecycle,
		&record.Source,
		&record.WorkScript,
		&record.LaunchMode,
		&record.RuntimeLimitSec,
		&idempotency,
		&record.SubmittedBy,
		&record.IngestToken,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SimopsRunRecord{}, ErrSimopsRunNotFound
		}
		return SimopsRunRecord{}, err
	}
	record.Lifecycle = SimopsLifecycle(lifecycle)
	if idempotency.Valid {
		record.IdempotencyKey = idempotency.String
	}
	return record, nil
}

func simopsSQLContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), simopsPostgresTimeout)
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func marshalLabels(labels map[string]string) (string, error) {
	if labels == nil {
		labels = map[string]string{}
	}
	data, err := json.Marshal(labels)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func requireAffected(result sql.Result, fallback error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fallback
	}
	return nil
}
