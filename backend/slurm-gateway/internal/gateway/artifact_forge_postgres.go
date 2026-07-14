package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PostgresArtifactForgeStore struct {
	db *sql.DB
}

func NewPostgresArtifactForgeStore(dsn string) (*PostgresArtifactForgeStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("Artifact Forge Postgres DSN is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	store := &PostgresArtifactForgeStore{db: db}
	ctx, cancel := simopsSQLContext()
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS artifact_forge_requests (
			request_id TEXT PRIMARY KEY,
			game_session_id TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			reactor_id TEXT NOT NULL,
			run_id TEXT,
			decision TEXT NOT NULL,
			record JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			last_activity_at TIMESTAMPTZ NOT NULL,
			session_expires_at TIMESTAMPTZ NOT NULL,
			retain_until TIMESTAMPTZ NOT NULL,
			UNIQUE (game_session_id, idempotency_key)
		);
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ;
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS session_expires_at TIMESTAMPTZ;
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS retain_until TIMESTAMPTZ;
	`); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresArtifactForgeStore) Find(gameSessionID, idempotencyKey string) (ArtifactForgeRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT record FROM artifact_forge_requests
		WHERE game_session_id = $1 AND idempotency_key = $2
	`, gameSessionID, idempotencyKey).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return ArtifactForgeRecord{}, ErrArtifactForgeNotFound
		}
		return ArtifactForgeRecord{}, err
	}
	var record ArtifactForgeRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return ArtifactForgeRecord{}, err
	}
	return record, nil
}

func (s *PostgresArtifactForgeStore) List(gameSessionID string) ([]ArtifactForgeRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT record FROM artifact_forge_requests
		WHERE game_session_id = $1 ORDER BY request_id
	`, gameSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []ArtifactForgeRecord{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var record ArtifactForgeRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *PostgresArtifactForgeStore) FindRun(runID string) (ArtifactForgeRecord, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT record FROM artifact_forge_requests WHERE run_id = $1`, runID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return ArtifactForgeRecord{}, ErrArtifactForgeNotFound
		}
		return ArtifactForgeRecord{}, err
	}
	var record ArtifactForgeRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return ArtifactForgeRecord{}, err
	}
	return record, nil
}

func (s *PostgresArtifactForgeStore) Save(record ArtifactForgeRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO artifact_forge_requests (
			request_id, game_session_id, idempotency_key, reactor_id, run_id,
			decision, record, created_at, updated_at, last_activity_at, session_expires_at, retain_until
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (game_session_id, idempotency_key) DO UPDATE SET
			reactor_id = EXCLUDED.reactor_id,
			run_id = EXCLUDED.run_id,
			decision = EXCLUDED.decision,
			record = EXCLUDED.record,
			updated_at = EXCLUDED.updated_at,
			last_activity_at = EXCLUDED.last_activity_at,
			session_expires_at = EXCLUDED.session_expires_at,
			retain_until = EXCLUDED.retain_until
		WHERE artifact_forge_requests.record->'outcome' IS NULL
		   OR artifact_forge_requests.record->'outcome' = EXCLUDED.record->'outcome'
	`, record.RequestID, record.GameSessionID, record.IdempotencyKey, record.ReactorID, nullableString(record.RunID), string(record.Decision), raw, record.CreatedAt, record.UpdatedAt, record.LastActivityAt, record.SessionExpiresAt, record.RetainUntil)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("Artifact Forge applied outcome cannot be removed or replaced")
	}
	return nil
}

func (s *PostgresArtifactForgeStore) TouchSession(gameSessionID string, activityAt time.Time) error {
	records, err := s.List(gameSessionID)
	if err != nil {
		return err
	}
	for _, record := range records {
		setArtifactForgeRetention(&record, activityAt)
		if err := s.Save(record); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresArtifactForgeStore) PruneExpired(now time.Time) (int64, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `DELETE FROM artifact_forge_requests WHERE retain_until <= $1`, now.UTC())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
