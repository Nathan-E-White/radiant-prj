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
	record.State = ensureSimopsRunState(record)

	result, err := tx.ExecContext(ctx, `
		INSERT INTO simops_runs (
			run_id, scenario_id, lifecycle, source, work_script, launch_mode,
			runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
			created_at, updated_at, state_phase, condition_status, cleanup_disposition, launch_observation_deadline
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
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
		string(record.State.Phase), string(record.State.Condition.Status), string(record.State.Cleanup), nullableTime(timePtr(record.State.LaunchObservationDeadline)),
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
		worker.State = ensureSimopsWorkerState(worker)
		labels, err := marshalLabels(worker.Labels)
		if err != nil {
			return SimopsRunRecord{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO simops_workers (
					run_id, worker_id, worker_kind, lifecycle, launch_mode,
					endpoint, frames, labels, observed_lifecycle, observed_reason,
					observed_message, runtime, runtime_id, observed_exit_code,
					observed_at, updated_at, state_phase, launch_disposition, active_execution,
					condition_status, failure_stage, cleanup_disposition
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
			`,
			record.RunID,
			worker.WorkerID,
			string(worker.WorkerKind),
			string(worker.Lifecycle),
			worker.LaunchMode,
			nullableString(worker.Endpoint),
			worker.Frames,
			labels,
			nullableString(string(worker.ObservedLifecycle)),
			nullableString(worker.ObservedReason),
			nullableString(worker.ObservedMessage),
			nullableString(worker.Runtime),
			nullableString(worker.RuntimeID),
			nullableInt(worker.ObservedExitCode),
			nullableTime(worker.ObservedAt),
			worker.UpdatedAt,
			string(worker.State.Phase), string(worker.State.LaunchDisposition), string(worker.State.ActiveExecution), string(worker.State.Condition.Status), string(worker.State.FailureStage), string(worker.State.Cleanup),
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
		materializeWorkerLaunchState(&worker)
		labels, err := marshalLabels(worker.Labels)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO simops_workers (
					run_id, worker_id, worker_kind, lifecycle, launch_mode,
					endpoint, frames, labels, observed_lifecycle, observed_reason,
					observed_message, runtime, runtime_id, observed_exit_code,
					observed_at, updated_at, state_phase, launch_disposition, active_execution,
					condition_status, failure_stage, cleanup_disposition
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
				ON CONFLICT (run_id, worker_id) DO UPDATE
				SET worker_kind = EXCLUDED.worker_kind,
				    lifecycle = CASE
			      WHEN simops_workers.frames > EXCLUDED.frames THEN simops_workers.lifecycle
			      ELSE EXCLUDED.lifecycle
			    END,
			    launch_mode = EXCLUDED.launch_mode,
			    endpoint = EXCLUDED.endpoint,
			    frames = GREATEST(simops_workers.frames, EXCLUDED.frames),
			    labels = EXCLUDED.labels,
			    observed_lifecycle = COALESCE(EXCLUDED.observed_lifecycle, simops_workers.observed_lifecycle),
			    observed_reason = COALESCE(EXCLUDED.observed_reason, simops_workers.observed_reason),
			    observed_message = COALESCE(EXCLUDED.observed_message, simops_workers.observed_message),
			    runtime = COALESCE(EXCLUDED.runtime, simops_workers.runtime),
			    runtime_id = COALESCE(EXCLUDED.runtime_id, simops_workers.runtime_id),
			    observed_exit_code = COALESCE(EXCLUDED.observed_exit_code, simops_workers.observed_exit_code),
			    observed_at = COALESCE(EXCLUDED.observed_at, simops_workers.observed_at),
			    state_phase = EXCLUDED.state_phase,
			    launch_disposition = EXCLUDED.launch_disposition,
			    active_execution = EXCLUDED.active_execution,
			    condition_status = EXCLUDED.condition_status,
			    failure_stage = EXCLUDED.failure_stage,
			    cleanup_disposition = EXCLUDED.cleanup_disposition,
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
			nullableString(string(worker.ObservedLifecycle)),
			nullableString(worker.ObservedReason),
			nullableString(worker.ObservedMessage),
			nullableString(worker.Runtime),
			nullableString(worker.RuntimeID),
			nullableInt(worker.ObservedExitCode),
			nullableTime(worker.ObservedAt),
			worker.UpdatedAt,
			string(worker.State.Phase), string(worker.State.LaunchDisposition), string(worker.State.ActiveExecution), string(worker.State.Condition.Status), string(worker.State.FailureStage), string(worker.State.Cleanup),
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
		       created_at, updated_at, state_phase, condition_status, cleanup_disposition, launch_observation_deadline
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
		       created_at, updated_at, state_phase, condition_status, cleanup_disposition, launch_observation_deadline
		FROM simops_runs
		WHERE run_id = $1
	`, runID)
}

func (s *PostgresSimopsStore) ListWorkers(runID string) ([]SimopsWorkerRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, worker_id, worker_kind, lifecycle, launch_mode,
		       endpoint, frames, labels, observed_lifecycle, observed_reason,
		       observed_message, runtime, runtime_id, observed_exit_code,
		       observed_at, updated_at, state_phase, launch_disposition, active_execution,
		       condition_status, failure_stage, cleanup_disposition
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
		var observedLifecycle, observedReason, observedMessage, runtime, runtimeID sql.NullString
		var observedExitCode sql.NullInt64
		var observedAt sql.NullTime
		var statePhase, launchDisposition, activeExecution, conditionStatus, failureStage, cleanupDisposition sql.NullString
		var labelsRaw []byte
		if err := rows.Scan(
			&worker.RunID,
			&worker.WorkerID,
			&kind,
			&lifecycle,
			&worker.LaunchMode,
			&endpoint,
			&worker.Frames,
			&labelsRaw,
			&observedLifecycle,
			&observedReason,
			&observedMessage,
			&runtime,
			&runtimeID,
			&observedExitCode,
			&observedAt,
			&worker.UpdatedAt,
			&statePhase, &launchDisposition, &activeExecution, &conditionStatus, &failureStage, &cleanupDisposition,
		); err != nil {
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
		if observedLifecycle.Valid {
			worker.ObservedLifecycle = ObservedWorkerState(observedLifecycle.String)
		}
		if observedReason.Valid {
			worker.ObservedReason = observedReason.String
		}
		if observedMessage.Valid {
			worker.ObservedMessage = observedMessage.String
		}
		if runtime.Valid {
			worker.Runtime = runtime.String
		}
		if runtimeID.Valid {
			worker.RuntimeID = runtimeID.String
		}
		if observedExitCode.Valid {
			exitCode := int(observedExitCode.Int64)
			worker.ObservedExitCode = &exitCode
		}
		if observedAt.Valid {
			observedAtTime := observedAt.Time
			worker.ObservedAt = &observedAtTime
		}
		if statePhase.Valid {
			worker.State = SimopsWorkerState{Phase: SimopsWorkerPhase(statePhase.String), LaunchDisposition: SimopsLaunchDisposition(launchDisposition.String), ActiveExecution: SimopsActiveExecution(activeExecution.String), Condition: SimopsOperationalCondition{Status: SimopsConditionStatus(conditionStatus.String)}, FailureStage: SimopsFailureStage(failureStage.String), Cleanup: SimopsCleanupDisposition(cleanupDisposition.String)}
		}
		worker.State = ensureSimopsWorkerState(worker)
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
		SET lifecycle = $2, state_phase = CASE $2 WHEN 'starting' THEN 'launching' WHEN 'streaming' THEN 'running' WHEN 'complete' THEN 'completed' WHEN 'failed' THEN 'failed' WHEN 'stopped' THEN 'stopped' ELSE 'admitted' END, condition_status = CASE WHEN $2 = 'failed' THEN 'failed' ELSE condition_status END, updated_at = $3
		WHERE run_id = $1
		RETURNING run_id, scenario_id, lifecycle, source, work_script, launch_mode,
		          runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
		          created_at, updated_at, state_phase, condition_status, cleanup_disposition, launch_observation_deadline
	`, runID, string(lifecycle), time.Now().UTC())
	return scanSimopsRun(row)
}

func (s *PostgresSimopsStore) UpdateRunLifecycleWithPublicationIntent(runID string, lifecycle SimopsLifecycle, event SimopsEvent) (SimopsRunRecord, SimopsPublicationIntent, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	if event.RunID == "" {
		event.RunID = runID
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	intent := newSimopsPublicationIntent(event, now)
	eventRaw, err := json.Marshal(event)
	if err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}

	record, err := scanSimopsRun(tx.QueryRowContext(ctx, `
		UPDATE simops_runs
		SET lifecycle = $2, state_phase = CASE $2 WHEN 'starting' THEN 'launching' WHEN 'streaming' THEN 'running' WHEN 'complete' THEN 'completed' WHEN 'failed' THEN 'failed' WHEN 'stopped' THEN 'stopped' ELSE 'admitted' END, condition_status = CASE WHEN $2 = 'failed' THEN 'failed' ELSE condition_status END, updated_at = $3
		WHERE run_id = $1
		RETURNING run_id, scenario_id, lifecycle, source, work_script, launch_mode,
		          runtime_limit_sec, idempotency_key, submitted_by, ingest_token,
		          created_at, updated_at, state_phase, condition_status, cleanup_disposition, launch_observation_deadline
	`, runID, string(lifecycle), now))
	if err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simops_publication_intents (
			intent_id, run_id, event, state, attempts, created_at, updated_at
		)
		VALUES ($1,$2,$3::jsonb,$4,0,$5,$6)
		ON CONFLICT (intent_id) DO NOTHING
	`, intent.IntentID, intent.RunID, string(eventRaw), intent.State, intent.CreatedAt, intent.UpdatedAt); err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}

	if err := tx.Commit(); err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}
	canonical, err := s.getPublicationIntent(intent.IntentID)
	if err != nil {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, err
	}
	return record, canonical, nil
}

func (s *PostgresSimopsStore) UpdateWorkerFrames(runID string, workerID string, lifecycle SimopsLifecycle, framesDelta int) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `
		UPDATE simops_workers
		SET lifecycle = $3,
		    frames = frames + $4,
		    updated_at = $5
		WHERE run_id = $1 AND worker_id = $2
	`, runID, workerID, string(lifecycle), framesDelta, time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrSimopsRunNotFound)
}

func (s *PostgresSimopsStore) ApplyStopTransition(transition SimopsStopTransition) (SimopsStopOutcome, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	now := transition.OccurredAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists string
	if err := tx.QueryRowContext(ctx, `SELECT run_id FROM simops_runs WHERE run_id=$1 FOR UPDATE`, transition.RunID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SimopsStopOutcome{}, ErrSimopsRunNotFound
		}
		return SimopsStopOutcome{}, err
	}
	condition := "nominal"
	phase, lifecycle := "stopping", string(SimopsDegraded)
	if transition.StopConfirmed || transition.CleanupConfirmed {
		condition = "failed"
	} else if transition.StopFailure != "" || transition.Cleanup == SimopsCleanupIncomplete || transition.Cleanup == SimopsCleanupFailed {
		condition = "degraded"
	}
	if transition.Cleanup == SimopsCleanupCompleted && transition.StopFailure == "" {
		phase, lifecycle = "stopped", string(SimopsStopped)
	}
	workerPhase := ""
	activeExecution := ""
	if transition.Cleanup == SimopsCleanupCompleted && transition.StopFailure == "" {
		workerPhase, activeExecution = "stopped", "terminal_observed"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE simops_workers SET cleanup_disposition=$2, condition_status=CASE WHEN $4 OR $5 THEN 'failed' WHEN $3 <> '' OR $2 IN ('incomplete','failed') THEN 'degraded' ELSE condition_status END, failure_stage=CASE WHEN $5 THEN 'cleanup' WHEN $4 THEN 'stop' ELSE failure_stage END, state_phase=CASE WHEN $6 <> '' AND condition_status <> 'failed' THEN $6 ELSE state_phase END, active_execution=CASE WHEN $7 <> '' AND condition_status <> 'failed' THEN $7 ELSE active_execution END, lifecycle=CASE WHEN $6 <> '' AND condition_status <> 'failed' THEN 'stopped' ELSE lifecycle END, updated_at=$8 WHERE run_id=$1`, transition.RunID, string(transition.Cleanup), transition.StopFailure, transition.StopConfirmed, transition.CleanupConfirmed, workerPhase, activeExecution, now); err != nil {
		return SimopsStopOutcome{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO simops_cleanup_attempts (run_id, disposition, source, message, observed_at) VALUES ($1,$2,'stop-orchestrator',$3,$4)`, transition.RunID, string(transition.Cleanup), nullableString(firstNonEmpty(transition.CleanupFailure, transition.StopFailure)), now); err != nil {
		return SimopsStopOutcome{}, err
	}
	if transition.StopFailure != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO simops_worker_observations (run_id, worker_id, code, source, message, observed_at) SELECT run_id, worker_id, 'stop-incomplete', 'stop-orchestrator', $2, $3 FROM simops_workers WHERE run_id=$1`, transition.RunID, transition.StopFailure, now); err != nil {
			return SimopsStopOutcome{}, err
		}
	}
	if transition.Cleanup == SimopsCleanupIncomplete || transition.Cleanup == SimopsCleanupFailed {
		code := "cleanup-incomplete"
		if transition.CleanupConfirmed {
			code = "cleanup-failed"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO simops_worker_observations (run_id, worker_id, code, source, message, observed_at) SELECT run_id, worker_id, $2, 'stop-orchestrator', $3, $4 FROM simops_workers WHERE run_id=$1`, transition.RunID, code, nullableString(transition.CleanupFailure), now); err != nil {
			return SimopsStopOutcome{}, err
		}
	}
	var anyFailed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM simops_workers WHERE run_id=$1 AND condition_status='failed')`, transition.RunID).Scan(&anyFailed); err != nil {
		return SimopsStopOutcome{}, err
	}
	if anyFailed {
		phase, lifecycle, condition = "failed", string(SimopsFailed), "failed"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE simops_runs SET state_phase=$2, condition_status=CASE WHEN $3 = 'failed' OR EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=simops_runs.run_id AND w.condition_status='failed') THEN 'failed' WHEN $3 = 'degraded' OR EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=simops_runs.run_id AND w.condition_status='degraded') THEN 'degraded' ELSE 'nominal' END, cleanup_disposition=$4, lifecycle=$5, updated_at=$6 WHERE run_id=$1`, transition.RunID, phase, condition, string(transition.Cleanup), lifecycle, now); err != nil {
		return SimopsStopOutcome{}, err
	}
	var intent *SimopsPublicationIntent
	if phase == "stopped" {
		event := SimopsEvent{RunID: transition.RunID, EventType: "run.lifecycle", Lifecycle: SimopsStopped, OccurredAt: now}
		raw, err := json.Marshal(event)
		if err != nil {
			return SimopsStopOutcome{}, err
		}
		created := newSimopsPublicationIntent(event, now)
		if _, err := tx.ExecContext(ctx, `INSERT INTO simops_publication_intents (intent_id, run_id, event, state, attempts, created_at, updated_at) VALUES ($1,$2,$3::jsonb,$4,0,$5,$6) ON CONFLICT (intent_id) DO NOTHING`, created.IntentID, transition.RunID, raw, created.State, now, now); err != nil {
			return SimopsStopOutcome{}, err
		}
		intent = &created
	}
	if err := tx.Commit(); err != nil {
		return SimopsStopOutcome{}, err
	}
	run, err := s.GetRun(transition.RunID)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	workers, err := s.ListWorkers(transition.RunID)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	return SimopsStopOutcome{Run: run, Workers: workers, Intent: intent}, nil
}

func (s *PostgresSimopsStore) UpdateWorkerObservedLifecycle(observation ObservedWorkerLifecycle) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	observedAt := observation.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	worker, err := scanSimopsWorker(tx.QueryRowContext(ctx, `
		SELECT run_id, worker_id, worker_kind, lifecycle, launch_mode, endpoint, frames, labels,
		       observed_lifecycle, observed_reason, observed_message, runtime, runtime_id,
		       observed_exit_code, observed_at, updated_at, state_phase, launch_disposition,
		       active_execution, condition_status, failure_stage, cleanup_disposition
		FROM simops_workers WHERE run_id = $1 AND worker_id = $2 FOR UPDATE`, observation.RunID, observation.WorkerID))
	if err != nil {
		return err
	}
	applyObservedWorkerLifecycle(&worker, observation, time.Now)
	worker.State = ensureSimopsWorkerState(worker)
	state, evidence := applyWorkerRuntimeObservation(worker.State, observation)
	worker.State = state
	if evidence != nil {
		applyDegradation(&worker.State, *evidence)
		if _, err := tx.ExecContext(ctx, `INSERT INTO simops_worker_observations (run_id, worker_id, code, source, message, observed_at) VALUES ($1,$2,$3,$4,$5,$6)`, observation.RunID, observation.WorkerID, evidence.Code, nullableString(evidence.Source), nullableString(evidence.Message), evidence.ObservedAt); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE simops_workers SET observed_lifecycle=$3, observed_reason=$4, observed_message=$5, runtime=$6, runtime_id=$7, observed_exit_code=$8, observed_at=$9, updated_at=$10, state_phase=$11, launch_disposition=$12, active_execution=$13, condition_status=$14, failure_stage=$15, cleanup_disposition=$16 WHERE run_id=$1 AND worker_id=$2`, observation.RunID, observation.WorkerID, nullableString(string(worker.ObservedLifecycle)), nullableString(worker.ObservedReason), nullableString(worker.ObservedMessage), nullableString(worker.Runtime), nullableString(worker.RuntimeID), nullableInt(worker.ObservedExitCode), nullableTime(worker.ObservedAt), worker.UpdatedAt, string(worker.State.Phase), string(worker.State.LaunchDisposition), string(worker.State.ActiveExecution), string(worker.State.Condition.Status), string(worker.State.FailureStage), string(worker.State.Cleanup))
	if err != nil {
		return err
	}
	if err := requireAffected(result, ErrSimopsRunNotFound); err != nil {
		return err
	}
	// Aggregate from the same transaction's worker facts. This is deliberately
	// a persisted transition, not a read-path reconciliation (#215 owns due
	// deadline evaluation separately).
	if _, err := tx.ExecContext(ctx, `
		UPDATE simops_runs r SET
		  condition_status = CASE WHEN EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=r.run_id AND (w.condition_status='failed' OR w.cleanup_disposition='failed')) THEN 'failed'
		                          WHEN EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=r.run_id AND (w.condition_status='degraded' OR w.launch_disposition='attempted-absent' OR w.cleanup_disposition='incomplete')) THEN 'degraded'
		                          ELSE 'nominal' END,
		  state_phase = CASE WHEN EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=r.run_id AND (w.condition_status='failed' OR w.cleanup_disposition='failed')) THEN 'failed'
		                     WHEN EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=r.run_id AND w.state_phase='executing') THEN 'running'
		                     WHEN NOT EXISTS (SELECT 1 FROM simops_workers w WHERE w.run_id=r.run_id AND w.state_phase NOT IN ('completed','failed','stopped')) THEN 'completed'
		                     ELSE 'launching' END,
		  updated_at = $2
		WHERE r.run_id = $1`, observation.RunID, worker.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresSimopsStore) ListWorkerDegradationEvidence(runID string, workerID string) ([]SimopsDegradationEvidence, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT code, source, message, observed_at FROM simops_worker_observations WHERE run_id=$1 AND worker_id=$2 ORDER BY observed_at, observation_id`, runID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	evidence := []SimopsDegradationEvidence{}
	for rows.Next() {
		var item SimopsDegradationEvidence
		var source, message sql.NullString
		if err := rows.Scan(&item.Code, &source, &message, &item.ObservedAt); err != nil {
			return nil, err
		}
		item.Source, item.Message = source.String, message.String
		evidence = append(evidence, item)
	}
	return evidence, rows.Err()
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

func (s *PostgresSimopsStore) ListPublicationIntents(runID string) ([]SimopsPublicationIntent, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT intent_id, run_id, event, state, attempts, COALESCE(last_error, ''), created_at, updated_at, published_at
		FROM simops_publication_intents
		WHERE run_id = $1
		ORDER BY created_at, intent_id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	intents := []SimopsPublicationIntent{}
	for rows.Next() {
		intent, err := scanPublicationIntent(rows)
		if err != nil {
			return nil, err
		}
		intents = append(intents, intent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(intents) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return intents, nil
}

func (s *PostgresSimopsStore) MarkPublicationIntentPublished(intentID string, event SimopsEvent) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	var frame any
	if len(event.Frame) > 0 {
		frame = string(event.Frame)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO simops_events (run_id, worker_id, event_type, lifecycle, frame, occurred_at)
		SELECT $1,$2,$3,$4,$5::jsonb,$6
		WHERE NOT EXISTS (
			SELECT 1
			FROM simops_events
			WHERE run_id = $1
			  AND COALESCE(worker_id, '') = COALESCE($2, '')
			  AND event_type = $3
			  AND COALESCE(lifecycle, '') = COALESCE($4, '')
		)
	`, event.RunID, nullableString(event.WorkerID), event.EventType, nullableString(string(event.Lifecycle)), frame, occurredAt); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE simops_publication_intents
		SET state = $2, last_error = NULL, updated_at = $3, published_at = $3
		WHERE intent_id = $1
	`, intentID, SimopsPublicationPublished, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := requireAffected(result, ErrSimopsRunNotFound); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresSimopsStore) MarkPublicationIntentFailed(intentID string, publishErr error) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	message := ""
	if publishErr != nil {
		message = publishErr.Error()
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE simops_publication_intents
		SET state = $2, attempts = attempts + 1, last_error = $3, updated_at = $4
		WHERE intent_id = $1
	`, intentID, SimopsPublicationFailed, nullableString(message), time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrSimopsRunNotFound)
}

func (s *PostgresSimopsStore) getPublicationIntent(intentID string) (SimopsPublicationIntent, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return scanPublicationIntent(s.db.QueryRowContext(ctx, `
		SELECT intent_id, run_id, event, state, attempts, COALESCE(last_error, ''), created_at, updated_at, published_at
		FROM simops_publication_intents
		WHERE intent_id = $1
	`, intentID))
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

func (s *PostgresSimopsStore) CreateDeliveryAttempt(request DeliveryAttemptRequest) (DeliveryAttempt, bool, error) {
	id, err := deliveryAttemptID(request)
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	coordinates, err := json.Marshal(request.Coordinates)
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	now := time.Now().UTC()
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO simops_delivery_attempts (attempt_id, run_id, target, coordinates, state, created_at, updated_at)
		VALUES ($1,$2,$3,$4::jsonb,$5,$6,$6) ON CONFLICT (attempt_id) DO NOTHING`,
		id, request.RunID, request.Target, coordinates, string(DeliveryAttemptPending), now)
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	created, err := result.RowsAffected()
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	attempt, err := s.GetDeliveryAttempt(id)
	return attempt, created == 1, err
}

func (s *PostgresSimopsStore) GetDeliveryAttempt(id string) (DeliveryAttempt, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return scanDeliveryAttempt(s.db.QueryRowContext(ctx, `SELECT attempt_id, run_id, target, location, coordinates, state, reason, evidence, created_at, updated_at FROM simops_delivery_attempts WHERE attempt_id = $1`, id))
}

func (s *PostgresSimopsStore) PrepareDeliveryAttempt(id string, location string) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `UPDATE simops_delivery_attempts SET location=$2, updated_at=$3 WHERE attempt_id=$1`, id, nullableString(location), time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrDeliveryAttemptNotFound)
}

func (s *PostgresSimopsStore) ResolveDeliveryAttempt(id string, evidence VerifiedDeliveryEvidence) error {
	attempt, err := s.GetDeliveryAttempt(id)
	if err != nil {
		return err
	}
	evidence.AttemptID = id
	evidence.Coordinates = cloneDeliveryCoordinates(attempt.Coordinates)
	payload, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `UPDATE simops_delivery_attempts SET state=$2, reason=NULL, evidence=$3::jsonb, updated_at=$4 WHERE attempt_id=$1`, id, string(DeliveryAttemptResolved), payload, time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrDeliveryAttemptNotFound)
}

func (s *PostgresSimopsStore) MarkDeliveryAttemptUnknown(id string, reason string) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `UPDATE simops_delivery_attempts SET state=$2, reason=$3, updated_at=$4 WHERE attempt_id=$1`, id, string(DeliveryAttemptUnknown), nullableString(reason), time.Now().UTC())
	if err != nil {
		return err
	}
	return requireAffected(result, ErrDeliveryAttemptNotFound)
}

func (s *PostgresSimopsStore) ListDeliveryAttempts(runID string) ([]DeliveryAttempt, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT attempt_id, run_id, target, location, coordinates, state, reason, evidence, created_at, updated_at FROM simops_delivery_attempts WHERE run_id=$1 ORDER BY created_at, attempt_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []DeliveryAttempt{}
	for rows.Next() {
		attempt, err := scanDeliveryAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(attempts) == 0 {
		if _, err := s.GetRun(runID); err != nil {
			return nil, err
		}
	}
	return attempts, nil
}

type deliveryAttemptScanner interface{ Scan(...any) error }

func scanDeliveryAttempt(row deliveryAttemptScanner) (DeliveryAttempt, error) {
	var attempt DeliveryAttempt
	var state string
	var location, reason sql.NullString
	var coordinates, evidence []byte
	if err := row.Scan(&attempt.AttemptID, &attempt.RunID, &attempt.Target, &location, &coordinates, &state, &reason, &evidence, &attempt.CreatedAt, &attempt.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeliveryAttempt{}, ErrDeliveryAttemptNotFound
		}
		return DeliveryAttempt{}, err
	}
	if err := json.Unmarshal(coordinates, &attempt.Coordinates); err != nil {
		return DeliveryAttempt{}, err
	}
	attempt.State = DeliveryAttemptState(state)
	if location.Valid {
		attempt.Location = location.String
	}
	if reason.Valid {
		attempt.Reason = reason.String
	}
	if len(evidence) > 0 {
		var decoded VerifiedDeliveryEvidence
		if err := json.Unmarshal(evidence, &decoded); err != nil {
			return DeliveryAttempt{}, err
		}
		attempt.Evidence = &decoded
	}
	return attempt, nil
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

type simopsPublicationIntentScanner interface {
	Scan(dest ...any) error
}

type simopsWorkerScanner interface {
	Scan(dest ...any) error
}

func scanSimopsWorker(row simopsWorkerScanner) (SimopsWorkerRecord, error) {
	var worker SimopsWorkerRecord
	var kind, lifecycle string
	var endpoint, observedLifecycle, observedReason, observedMessage, runtime, runtimeID sql.NullString
	var observedExitCode sql.NullInt64
	var observedAt sql.NullTime
	var labelsRaw []byte
	var statePhase, launchDisposition, activeExecution, conditionStatus, failureStage, cleanupDisposition sql.NullString
	if err := row.Scan(&worker.RunID, &worker.WorkerID, &kind, &lifecycle, &worker.LaunchMode, &endpoint, &worker.Frames, &labelsRaw, &observedLifecycle, &observedReason, &observedMessage, &runtime, &runtimeID, &observedExitCode, &observedAt, &worker.UpdatedAt, &statePhase, &launchDisposition, &activeExecution, &conditionStatus, &failureStage, &cleanupDisposition); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SimopsWorkerRecord{}, ErrSimopsRunNotFound
		}
		return SimopsWorkerRecord{}, err
	}
	worker.WorkerKind, worker.Lifecycle, worker.Endpoint = SimopsWorkerKind(kind), SimopsLifecycle(lifecycle), endpoint.String
	if len(labelsRaw) > 0 {
		if err := json.Unmarshal(labelsRaw, &worker.Labels); err != nil {
			return SimopsWorkerRecord{}, err
		}
	}
	if observedLifecycle.Valid {
		worker.ObservedLifecycle = ObservedWorkerState(observedLifecycle.String)
	}
	if observedReason.Valid {
		worker.ObservedReason = observedReason.String
	}
	if observedMessage.Valid {
		worker.ObservedMessage = observedMessage.String
	}
	if runtime.Valid {
		worker.Runtime = runtime.String
	}
	if runtimeID.Valid {
		worker.RuntimeID = runtimeID.String
	}
	if observedExitCode.Valid {
		code := int(observedExitCode.Int64)
		worker.ObservedExitCode = &code
	}
	if observedAt.Valid {
		value := observedAt.Time
		worker.ObservedAt = &value
	}
	if statePhase.Valid {
		worker.State = SimopsWorkerState{Phase: SimopsWorkerPhase(statePhase.String), LaunchDisposition: SimopsLaunchDisposition(launchDisposition.String), ActiveExecution: SimopsActiveExecution(activeExecution.String), Condition: SimopsOperationalCondition{Status: SimopsConditionStatus(conditionStatus.String)}, FailureStage: SimopsFailureStage(failureStage.String), Cleanup: SimopsCleanupDisposition(cleanupDisposition.String)}
	}
	worker.State = ensureSimopsWorkerState(worker)
	return worker, nil
}

func scanSimopsRun(row simopsRunScanner) (SimopsRunRecord, error) {
	var record SimopsRunRecord
	var lifecycle string
	var idempotency sql.NullString
	var statePhase, conditionStatus, cleanupDisposition sql.NullString
	var launchObservationDeadline sql.NullTime
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
		&statePhase,
		&conditionStatus,
		&cleanupDisposition,
		&launchObservationDeadline,
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
	if statePhase.Valid {
		record.State = SimopsRunState{Phase: SimopsRunPhase(statePhase.String), Condition: SimopsOperationalCondition{Status: SimopsConditionStatus(conditionStatus.String)}, Cleanup: SimopsCleanupDisposition(cleanupDisposition.String)}
		if launchObservationDeadline.Valid {
			record.State.LaunchObservationDeadline = launchObservationDeadline.Time
		}
	}
	record.State = ensureSimopsRunState(record)
	return record, nil
}

func scanPublicationIntent(row simopsPublicationIntentScanner) (SimopsPublicationIntent, error) {
	var intent SimopsPublicationIntent
	var eventRaw []byte
	var publishedAt sql.NullTime
	if err := row.Scan(
		&intent.IntentID,
		&intent.RunID,
		&eventRaw,
		&intent.State,
		&intent.Attempts,
		&intent.LastError,
		&intent.CreatedAt,
		&intent.UpdatedAt,
		&publishedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SimopsPublicationIntent{}, ErrSimopsRunNotFound
		}
		return SimopsPublicationIntent{}, err
	}
	if err := json.Unmarshal(eventRaw, &intent.Event); err != nil {
		return SimopsPublicationIntent{}, err
	}
	if publishedAt.Valid {
		intent.PublishedAt = &publishedAt.Time
	}
	return intent, nil
}

func simopsSQLContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), simopsPostgresTimeout)
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func nullableInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
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
