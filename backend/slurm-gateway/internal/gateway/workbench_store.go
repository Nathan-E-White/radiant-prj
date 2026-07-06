package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var ErrWorkbenchNotFound = errors.New("workbench record not found")

type WorkbenchStore interface {
	SaveResidentSource(source ScadaResidentSourceDeclaration) error
	GetResidentTag(tagID string) (ScadaSourceTag, error)
	SaveScadaProjection(consumerName string, projection ScadaProjection) (bool, error)
	SaveResultProjection(consumerName string, projection SimopsResultProjection) (bool, error)
	SaveTwinStateProjection(consumerName string, projection TwinStateProjection) (bool, error)
	SaveLineage(lineage DigitalTwinValueLineage) error
	LatestMeasuredFrames(limit int) ([]ScadaTelemetryFrame, error)
	LatestResultFrames(limit int) ([]SimopsResultFrame, error)
	CurrentTwinState() (DigitalTwinState, error)
	LineageForValue(valueID string) (DigitalTwinValueLineage, error)
	Snapshot() (WorkbenchSnapshot, error)
}

type InMemoryWorkbenchStore struct {
	mu             sync.RWMutex
	sources        map[string]ScadaResidentSourceDeclaration
	tags           map[string]ScadaSourceTag
	measuredByTag  map[string]ScadaTelemetryFrame
	resultsByValue map[string]SimopsResultFrame
	twin           DigitalTwinState
	lineageByValue map[string]DigitalTwinValueLineage
}

func NewInMemoryWorkbenchStore() *InMemoryWorkbenchStore {
	return &InMemoryWorkbenchStore{
		sources:        make(map[string]ScadaResidentSourceDeclaration),
		tags:           make(map[string]ScadaSourceTag),
		measuredByTag:  make(map[string]ScadaTelemetryFrame),
		resultsByValue: make(map[string]SimopsResultFrame),
		lineageByValue: make(map[string]DigitalTwinValueLineage),
	}
}

func (s *InMemoryWorkbenchStore) SaveResidentSource(source ScadaResidentSourceDeclaration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[source.SourceID] = source
	for _, tag := range source.Tags {
		s.tags[tag.TagID] = tag
	}
	return nil
}

func (s *InMemoryWorkbenchStore) GetResidentTag(tagID string) (ScadaSourceTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tag, ok := s.tags[tagID]
	if !ok {
		return ScadaSourceTag{}, ErrWorkbenchNotFound
	}
	return tag, nil
}

func (s *InMemoryWorkbenchStore) SaveScadaProjection(_ string, projection ScadaProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.measuredByTag[projection.Frame.TagID] = projection.Frame
	return true, nil
}

func (s *InMemoryWorkbenchStore) SaveResultProjection(_ string, projection SimopsResultProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, value := range projection.Frame.Values {
		s.resultsByValue[value.ValueID] = projection.Frame
	}
	return true, nil
}

func (s *InMemoryWorkbenchStore) SaveTwinStateProjection(_ string, projection TwinStateProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.twin = projection.State
	return true, nil
}

func (s *InMemoryWorkbenchStore) SaveLineage(lineage DigitalTwinValueLineage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lineageByValue[lineage.ValueID] = lineage
	return nil
}

func (s *InMemoryWorkbenchStore) LatestMeasuredFrames(limit int) ([]ScadaTelemetryFrame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	frames := make([]ScadaTelemetryFrame, 0, len(s.measuredByTag))
	for _, frame := range s.measuredByTag {
		frames = append(frames, frame)
	}
	sort.Slice(frames, func(i, j int) bool {
		if frames[i].ObservedAt.Equal(frames[j].ObservedAt) {
			return frames[i].TagID < frames[j].TagID
		}
		return frames[i].ObservedAt.After(frames[j].ObservedAt)
	})
	return trimMeasured(frames, limit), nil
}

func (s *InMemoryWorkbenchStore) LatestResultFrames(limit int) ([]SimopsResultFrame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]SimopsResultFrame, 0, len(s.resultsByValue))
	seen := make(map[string]struct{})
	for _, frame := range s.resultsByValue {
		key := fmt.Sprintf("%s/%s/%d", frame.RunID, frame.WorkerID, frame.Sequence)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, frame)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ProducedAt > results[j].ProducedAt
	})
	return trimResults(results, limit), nil
}

func (s *InMemoryWorkbenchStore) CurrentTwinState() (DigitalTwinState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.twin.SchemaVersion == "" {
		return DigitalTwinState{}, ErrWorkbenchNotFound
	}
	return s.twin, nil
}

func (s *InMemoryWorkbenchStore) LineageForValue(valueID string) (DigitalTwinValueLineage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lineage, ok := s.lineageByValue[valueID]
	if !ok {
		return DigitalTwinValueLineage{}, ErrWorkbenchNotFound
	}
	return lineage, nil
}

func (s *InMemoryWorkbenchStore) Snapshot() (WorkbenchSnapshot, error) {
	measured, _ := s.LatestMeasuredFrames(100)
	results, _ := s.LatestResultFrames(20)
	twin, _ := s.CurrentTwinState()
	lineage := []DigitalTwinValueLineage{}
	s.mu.RLock()
	for _, record := range s.lineageByValue {
		lineage = append(lineage, record)
	}
	s.mu.RUnlock()
	sort.Slice(lineage, func(i, j int) bool { return lineage[i].LineageID < lineage[j].LineageID })
	return WorkbenchSnapshot{
		State:    buildWorkbenchState(measured, results, twin),
		Measured: measured,
		Twin:     twin,
		Lineage:  lineage,
		Results:  results,
	}, nil
}

type PostgresWorkbenchStore struct {
	db *sql.DB
}

func NewPostgresWorkbenchStore(dsn string) (*PostgresWorkbenchStore, error) {
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
		return nil, fmt.Errorf("ping workbench postgres store: %w", err)
	}
	return &PostgresWorkbenchStore{db: db}, nil
}

func (s *PostgresWorkbenchStore) SaveResidentSource(source ScadaResidentSourceDeclaration) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	raw, err := json.Marshal(source)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workbench_resident_sources (source_id, declaration, updated_at)
		VALUES ($1, $2::jsonb, now())
		ON CONFLICT (source_id) DO UPDATE
		SET declaration = EXCLUDED.declaration, updated_at = now()
	`, source.SourceID, raw); err != nil {
		return err
	}
	for _, tag := range source.Tags {
		tagRaw, err := json.Marshal(tag)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workbench_resident_tags (
				tag_id, source_id, asset_id, signal_kind, unit, value_basis, tag
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
			ON CONFLICT (tag_id) DO UPDATE
			SET source_id = EXCLUDED.source_id,
			    asset_id = EXCLUDED.asset_id,
			    signal_kind = EXCLUDED.signal_kind,
			    unit = EXCLUDED.unit,
			    value_basis = EXCLUDED.value_basis,
			    tag = EXCLUDED.tag
		`, tag.TagID, source.SourceID, tag.AssetID, string(tag.SignalKind), tag.Unit, string(tag.ValueBasis), tagRaw); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresWorkbenchStore) GetResidentTag(tagID string) (ScadaSourceTag, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT tag FROM workbench_resident_tags WHERE tag_id = $1`, tagID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ScadaSourceTag{}, ErrWorkbenchNotFound
		}
		return ScadaSourceTag{}, err
	}
	var tag ScadaSourceTag
	if err := json.Unmarshal(raw, &tag); err != nil {
		return ScadaSourceTag{}, err
	}
	return tag, nil
}

func (s *PostgresWorkbenchStore) SaveScadaProjection(consumerName string, projection ScadaProjection) (bool, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	processed, err := insertWorkbenchProcessed(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
	if err != nil || !processed {
		return processed, err
	}
	valueRaw, err := json.Marshal(projection.Frame.Value)
	if err != nil {
		return false, err
	}
	frameRaw := projection.Raw
	if len(frameRaw) == 0 {
		frameRaw, _ = json.Marshal(projection.Frame)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO scada_measured_frames (
			observed_at, sampled_at, source_id, tag_id, asset_id, signal_kind,
			sequence, unit, quality, value_basis, synthetic_status, value,
			frame, redpanda_topic, redpanda_partition, redpanda_offset
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,$15,$16)
	`, projection.ObservedAt, projection.SampledAt, projection.Frame.SourceID, projection.Frame.TagID,
		projection.Frame.AssetID, string(projection.Frame.SignalKind), projection.Frame.Sequence,
		projection.Frame.Unit, projection.Frame.Quality, string(projection.Frame.ValueBasis),
		projection.Frame.SyntheticStatus, valueRaw, frameRaw, projection.RedpandaTopic,
		projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
		return false, err
	}
	if err := upsertWorkbenchOffset(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *PostgresWorkbenchStore) SaveResultProjection(consumerName string, projection SimopsResultProjection) (bool, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	processed, err := insertWorkbenchProcessed(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
	if err != nil || !processed {
		return processed, err
	}
	frameRaw := projection.Raw
	if len(frameRaw) == 0 {
		frameRaw, _ = json.Marshal(projection.Frame)
	}
	for _, value := range projection.Frame.Values {
		valueRaw := value.Value
		if len(valueRaw) == 0 {
			valueRaw = []byte(`{}`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO simops_result_values (
				produced_at, received_at, run_id, scenario_id, worker_id, worker_kind,
				sequence, result_type, model_id, input_window_start, input_window_end,
				value_basis, synthetic_status, result_id, entity_id, value_id, label,
				unit, value, confidence, frame, redpanda_topic, redpanda_partition, redpanda_offset
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19::jsonb,$20,$21::jsonb,$22,$23,$24)
		`, projection.ProducedAt, projection.ReceivedAt, projection.Frame.RunID, projection.Frame.ScenarioID,
			projection.Frame.WorkerID, string(projection.Frame.WorkerKind), projection.Frame.Sequence,
			projection.Frame.ResultType, projection.Frame.ModelID, projection.Frame.InputWindow.Start,
			projection.Frame.InputWindow.End, string(projection.Frame.ValueBasis), projection.Frame.SyntheticStatus,
			value.ResultID, value.EntityID, value.ValueID, value.Label, value.Unit, valueRaw,
			value.Confidence, frameRaw, projection.RedpandaTopic, projection.RedpandaPartition,
			projection.RedpandaOffset); err != nil {
			return false, err
		}
	}
	if err := upsertWorkbenchOffset(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *PostgresWorkbenchStore) SaveTwinStateProjection(consumerName string, projection TwinStateProjection) (bool, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	processed := true
	if consumerName != "" {
		processed, err = insertWorkbenchProcessed(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
		if err != nil || !processed {
			return processed, err
		}
	}
	stateRaw := projection.Raw
	if len(stateRaw) == 0 {
		stateRaw, _ = json.Marshal(projection.State)
	}
	for _, entity := range projection.State.Entities {
		for _, value := range entity.Values {
			valueRaw, err := json.Marshal(value.Value)
			if err != nil {
				return false, err
			}
			freshnessRaw, err := json.Marshal(value.Freshness)
			if err != nil {
				return false, err
			}
			sourceIDsRaw, err := json.Marshal(value.SourceIDs)
			if err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO digital_twin_state_values (
					as_of, twin_id, entity_id, display_name, value_id, label, value_basis,
					unit, value, confidence, freshness, lineage_id, source_ids, state,
					redpanda_topic, redpanda_partition, redpanda_offset
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11::jsonb,$12,$13::jsonb,$14::jsonb,$15,$16,$17)
				ON CONFLICT (twin_id, entity_id, value_id) DO UPDATE
				SET as_of = EXCLUDED.as_of,
				    display_name = EXCLUDED.display_name,
				    label = EXCLUDED.label,
				    value_basis = EXCLUDED.value_basis,
				    unit = EXCLUDED.unit,
				    value = EXCLUDED.value,
				    confidence = EXCLUDED.confidence,
				    freshness = EXCLUDED.freshness,
				    lineage_id = EXCLUDED.lineage_id,
				    source_ids = EXCLUDED.source_ids,
				    state = EXCLUDED.state,
				    redpanda_topic = EXCLUDED.redpanda_topic,
				    redpanda_partition = EXCLUDED.redpanda_partition,
				    redpanda_offset = EXCLUDED.redpanda_offset
			`, projection.AsOf, projection.State.TwinID, entity.EntityID, entity.DisplayName,
				value.ValueID, value.Label, string(value.ValueBasis), value.Unit, valueRaw,
				value.Confidence, freshnessRaw, value.LineageID, sourceIDsRaw, stateRaw,
				projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
				return false, err
			}
		}
	}
	if consumerName != "" {
		if err := upsertWorkbenchOffset(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *PostgresWorkbenchStore) SaveLineage(lineage DigitalTwinValueLineage) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	raw, err := json.Marshal(lineage)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO digital_twin_lineage (lineage_id, value_id, value_basis, lineage, updated_at)
		VALUES ($1,$2,$3,$4::jsonb,now())
		ON CONFLICT (lineage_id) DO UPDATE
		SET value_id = EXCLUDED.value_id,
		    value_basis = EXCLUDED.value_basis,
		    lineage = EXCLUDED.lineage,
		    updated_at = now()
	`, lineage.LineageID, lineage.ValueID, string(lineage.ValueBasis), raw)
	return err
}

func (s *PostgresWorkbenchStore) LatestMeasuredFrames(limit int) ([]ScadaTelemetryFrame, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT frame
		FROM (
			SELECT DISTINCT ON (tag_id) tag_id, observed_at, frame
			FROM scada_measured_frames
			ORDER BY tag_id, observed_at DESC, redpanda_offset DESC
		) latest
		ORDER BY observed_at DESC, tag_id
		LIMIT $1
	`, normalizeLimit(limit, 100))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	frames := []ScadaTelemetryFrame{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var frame ScadaTelemetryFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	return frames, rows.Err()
}

func (s *PostgresWorkbenchStore) LatestResultFrames(limit int) ([]SimopsResultFrame, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT ON (run_id, worker_id, sequence) frame
		FROM simops_result_values
		ORDER BY run_id, worker_id, sequence, produced_at DESC
		LIMIT $1
	`, normalizeLimit(limit, 20))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []SimopsResultFrame{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var frame SimopsResultFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			return nil, err
		}
		results = append(results, frame)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ProducedAt > results[j].ProducedAt })
	return results, rows.Err()
}

func (s *PostgresWorkbenchStore) CurrentTwinState() (DigitalTwinState, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `
		SELECT state
		FROM digital_twin_state_values
		ORDER BY as_of DESC
		LIMIT 1
	`).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DigitalTwinState{}, ErrWorkbenchNotFound
		}
		return DigitalTwinState{}, err
	}
	var state DigitalTwinState
	if err := json.Unmarshal(raw, &state); err != nil {
		return DigitalTwinState{}, err
	}
	return state, nil
}

func (s *PostgresWorkbenchStore) LineageForValue(valueID string) (DigitalTwinValueLineage, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT lineage FROM digital_twin_lineage WHERE value_id = $1`, valueID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DigitalTwinValueLineage{}, ErrWorkbenchNotFound
		}
		return DigitalTwinValueLineage{}, err
	}
	var lineage DigitalTwinValueLineage
	if err := json.Unmarshal(raw, &lineage); err != nil {
		return DigitalTwinValueLineage{}, err
	}
	return lineage, nil
}

func (s *PostgresWorkbenchStore) Snapshot() (WorkbenchSnapshot, error) {
	measured, err := s.LatestMeasuredFrames(100)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	results, err := s.LatestResultFrames(20)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	twin, _ := s.CurrentTwinState()
	ctx, cancel := simopsSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `SELECT lineage FROM digital_twin_lineage ORDER BY lineage_id`)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	defer rows.Close()
	lineage := []DigitalTwinValueLineage{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return WorkbenchSnapshot{}, err
		}
		var record DigitalTwinValueLineage
		if err := json.Unmarshal(raw, &record); err != nil {
			return WorkbenchSnapshot{}, err
		}
		lineage = append(lineage, record)
	}
	return WorkbenchSnapshot{
		State:    buildWorkbenchState(measured, results, twin),
		Measured: measured,
		Twin:     twin,
		Lineage:  lineage,
		Results:  results,
	}, rows.Err()
}

func insertWorkbenchProcessed(ctx context.Context, tx *sql.Tx, consumerName, topic string, partition int, offset int64) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO workbench_processed_messages (
			consumer_name, redpanda_topic, redpanda_partition, redpanda_offset
		)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING
	`, consumerName, topic, partition, offset)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func upsertWorkbenchOffset(ctx context.Context, tx *sql.Tx, consumerName, topic string, partition int, offset int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workbench_consumer_offsets (
			consumer_name, redpanda_topic, redpanda_partition, redpanda_offset, updated_at
		)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (consumer_name, redpanda_topic, redpanda_partition)
		DO UPDATE SET redpanda_offset = GREATEST(workbench_consumer_offsets.redpanda_offset, EXCLUDED.redpanda_offset),
		              updated_at = now()
	`, consumerName, topic, partition, offset)
	return err
}

func buildWorkbenchState(measured []ScadaTelemetryFrame, results []SimopsResultFrame, twin DigitalTwinState) SimulatorWorkbenchState {
	counts := map[WorkbenchValueBasis]int{
		WorkbenchValueMeasured:  len(measured),
		WorkbenchValueImputed:   0,
		WorkbenchValueSimulated: 0,
	}
	for _, entity := range twin.Entities {
		for _, value := range entity.Values {
			counts[value.ValueBasis]++
		}
	}
	if counts[WorkbenchValueSimulated] == 0 {
		for _, result := range results {
			counts[WorkbenchValueSimulated] += len(result.Values)
		}
	}
	return SimulatorWorkbenchState{
		SchemaVersion:     "simulator-workbench.state.v1",
		GeneratedAt:       time.Now().UTC(),
		ScenarioID:        latestScenario(results),
		ValueBasisSummary: counts,
		MeasuredStateRefs: []string{"scada_measured_frames"},
		TwinStateRef:      "digital_twin_state_values",
		LineageRefs:       []string{"digital_twin_lineage"},
		ActiveSimulationRuns: []WorkbenchSimulationRunSummary{
			{
				RunID:          latestRun(results),
				ScenarioID:     latestScenario(results),
				Lifecycle:      "streaming",
				ValueBasis:     WorkbenchValueSimulated,
				Health:         "summary-only",
				ArtifactStatus: "dataflow-projected",
			},
		},
		Panels: []WorkbenchPanelSummary{
			{PanelID: "measured-state", Title: "Measured State", ValueBasis: WorkbenchValueMeasured},
			{PanelID: "twin-state", Title: "Imputed Twin State", ValueBasis: WorkbenchValueImputed},
			{PanelID: "simulation-results", Title: "Simulated Results", ValueBasis: WorkbenchValueSimulated},
		},
	}
}

func latestRun(results []SimopsResultFrame) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].RunID
}

func latestScenario(results []SimopsResultFrame) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].ScenarioID
}

func trimMeasured(frames []ScadaTelemetryFrame, limit int) []ScadaTelemetryFrame {
	limit = normalizeLimit(limit, len(frames))
	if len(frames) > limit {
		return frames[:limit]
	}
	return frames
}

func trimResults(results []SimopsResultFrame, limit int) []SimopsResultFrame {
	limit = normalizeLimit(limit, len(results))
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

func normalizeLimit(limit int, fallback int) int {
	if limit <= 0 {
		if fallback <= 0 {
			return 1
		}
		return fallback
	}
	return limit
}
