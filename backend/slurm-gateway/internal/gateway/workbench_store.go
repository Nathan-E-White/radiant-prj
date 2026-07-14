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
	GetTwinStatePublication(publicationID string) (TwinStatePublication, error)
	AcknowledgeTwinStatePublication(publicationID string) error
	LatestMeasuredFrames(limit int) ([]ScadaTelemetryFrame, error)
	LatestResultFrames(limit int) ([]SimopsResultFrame, error)
	CurrentTwinState() (DigitalTwinState, error)
	LineageForValue(valueID string) (DigitalTwinValueLineage, error)
	Snapshot() (WorkbenchSnapshot, error)
}

type workbenchDynamicMeasuredRetentionStore interface {
	PruneDynamicMeasured(before time.Time) error
}

type InMemoryWorkbenchStore struct {
	mu              sync.RWMutex
	sources         map[string]ScadaResidentSourceDeclaration
	tags            map[string]ScadaSourceTag
	measuredByTag   map[string]ScadaTelemetryFrame
	ingestedAtByTag map[string]time.Time
	resultsByValue  map[string]SimopsResultFrame
	twin            DigitalTwinState
	lineageByValue  map[string]DigitalTwinValueLineage
	processed       map[string]struct{}
	publications    map[string]TwinStatePublication
	generation      uint64
	now             func() time.Time
}

func NewInMemoryWorkbenchStore() *InMemoryWorkbenchStore {
	return &InMemoryWorkbenchStore{
		sources:         make(map[string]ScadaResidentSourceDeclaration),
		tags:            make(map[string]ScadaSourceTag),
		measuredByTag:   make(map[string]ScadaTelemetryFrame),
		ingestedAtByTag: make(map[string]time.Time),
		resultsByValue:  make(map[string]SimopsResultFrame),
		lineageByValue:  make(map[string]DigitalTwinValueLineage),
		processed:       make(map[string]struct{}),
		publications:    make(map[string]TwinStatePublication),
		now:             time.Now,
	}
}

func (s *InMemoryWorkbenchStore) SaveResidentSource(source ScadaResidentSourceDeclaration) error {
	source, err := cloneWorkbenchValue(source)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[source.SourceID] = source
	for _, tag := range source.Tags {
		tag.SourceID = source.SourceID
		tag.ReactorID = source.ReactorID
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
	return cloneWorkbenchValue(tag)
}

func (s *InMemoryWorkbenchStore) SaveScadaProjection(consumerName string, projection ScadaProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workbenchProjectionKey(consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
	if s.projectionProcessed(key) {
		return false, nil
	}
	frame, err := cloneWorkbenchValue(projection.Frame)
	if err != nil {
		return false, err
	}
	s.processed[key] = struct{}{}
	s.measuredByTag[frame.TagID] = frame
	if s.ingestedAtByTag == nil {
		s.ingestedAtByTag = make(map[string]time.Time)
	}
	if s.now == nil {
		s.now = time.Now
	}
	s.ingestedAtByTag[frame.TagID] = s.now().UTC()
	s.generation++
	return true, nil
}

func (s *InMemoryWorkbenchStore) SaveResultProjection(consumerName string, projection SimopsResultProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := workbenchProjectionKey(consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
	if s.projectionProcessed(key) {
		return false, nil
	}
	frame, err := cloneWorkbenchValue(projection.Frame)
	if err != nil {
		return false, err
	}
	s.processed[key] = struct{}{}
	for _, value := range frame.Values {
		s.resultsByValue[value.ValueID] = frame
	}
	s.generation++
	return true, nil
}

func (s *InMemoryWorkbenchStore) SaveTwinStateProjection(consumerName string, projection TwinStateProjection) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ""
	if projection.PublicationID != "" {
		key = "twin-publication\x00" + projection.PublicationID
		if s.projectionProcessed(key) {
			return false, nil
		}
	} else if consumerName != "" {
		key = workbenchProjectionKey(consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
		if s.projectionProcessed(key) {
			return false, nil
		}
	}
	transition, err := cloneWorkbenchValue(TwinStateTransition{State: projection.State, Lineage: projection.Lineage})
	if err != nil {
		return false, err
	}
	var publication TwinStatePublication
	if projection.PublicationID != "" && consumerName == "" {
		publication, err = cloneWorkbenchValue(TwinStatePublication{
			PublicationID: projection.PublicationID, Source: projection.PublicationSource,
			TwinStateTopic: projection.RedpandaTopic, State: transition.State, Lineage: transition.Lineage,
		})
		if err != nil {
			return false, err
		}
	}
	if key != "" {
		s.processed[key] = struct{}{}
	}
	s.twin = transition.State
	if projection.LineagePresent {
		s.lineageByValue = make(map[string]DigitalTwinValueLineage, len(transition.Lineage))
		for _, lineage := range transition.Lineage {
			s.lineageByValue[lineage.ValueID] = lineage
		}
	}
	if projection.PublicationID != "" && consumerName == "" {
		if s.publications == nil {
			s.publications = make(map[string]TwinStatePublication)
		}
		s.publications[projection.PublicationID] = publication
	}
	s.generation++
	return true, nil
}

func (s *InMemoryWorkbenchStore) GetTwinStatePublication(publicationID string) (TwinStatePublication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	publication, ok := s.publications[publicationID]
	if !ok {
		return TwinStatePublication{}, ErrWorkbenchNotFound
	}
	return cloneWorkbenchValue(publication)
}

func (s *InMemoryWorkbenchStore) AcknowledgeTwinStatePublication(publicationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	publication, ok := s.publications[publicationID]
	if ok {
		s.publications[publicationID] = TwinStatePublication{
			PublicationID: publication.PublicationID, Source: publication.Source,
			TwinStateTopic: publication.TwinStateTopic, Acknowledged: true,
		}
	}
	return nil
}

func (s *InMemoryWorkbenchStore) projectionProcessed(key string) bool {
	if s.processed == nil {
		s.processed = make(map[string]struct{})
	}
	_, ok := s.processed[key]
	return ok
}

func workbenchProjectionKey(consumerName, topic string, partition int, offset int64) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%d", consumerName, topic, partition, offset)
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
	return cloneWorkbenchValue(trimMeasured(frames, limit))
}

func (s *InMemoryWorkbenchStore) PruneDynamicMeasured(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for tagID, frame := range s.measuredByTag {
		if frame.ReactorID != "" && s.ingestedAtByTag[tagID].Before(before) {
			delete(s.measuredByTag, tagID)
			delete(s.ingestedAtByTag, tagID)
			changed = true
		}
	}
	if changed {
		s.generation++
	}
	return nil
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
	return cloneWorkbenchValue(trimResults(results, limit))
}

func (s *InMemoryWorkbenchStore) CurrentTwinState() (DigitalTwinState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.twin.SchemaVersion == "" {
		return DigitalTwinState{}, ErrWorkbenchNotFound
	}
	return cloneWorkbenchValue(s.twin)
}

func (s *InMemoryWorkbenchStore) LineageForValue(valueID string) (DigitalTwinValueLineage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lineage, ok := s.lineageByValue[valueID]
	if !ok {
		return DigitalTwinValueLineage{}, ErrWorkbenchNotFound
	}
	return cloneWorkbenchValue(lineage)
}

func (s *InMemoryWorkbenchStore) Snapshot() (WorkbenchSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	measured := make([]ScadaTelemetryFrame, 0, len(s.measuredByTag))
	for _, frame := range s.measuredByTag {
		measured = append(measured, frame)
	}
	sort.Slice(measured, func(i, j int) bool {
		if measured[i].ObservedAt.Equal(measured[j].ObservedAt) {
			return measured[i].TagID < measured[j].TagID
		}
		return measured[i].ObservedAt.After(measured[j].ObservedAt)
	})
	measured = trimMeasured(measured, 100)

	results := make([]SimopsResultFrame, 0, len(s.resultsByValue))
	seenResults := make(map[string]struct{})
	for _, frame := range s.resultsByValue {
		key := fmt.Sprintf("%s/%s/%d", frame.RunID, frame.WorkerID, frame.Sequence)
		if _, ok := seenResults[key]; ok {
			continue
		}
		seenResults[key] = struct{}{}
		results = append(results, frame)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ProducedAt > results[j].ProducedAt })
	results = trimResults(results, 20)

	twin := s.twin
	lineage := []DigitalTwinValueLineage{}
	for _, record := range s.lineageByValue {
		lineage = append(lineage, record)
	}
	sort.Slice(lineage, func(i, j int) bool { return lineage[i].LineageID < lineage[j].LineageID })
	state := buildWorkbenchState(measured, results, twin)
	state.SnapshotGeneration = s.generation
	snapshot := WorkbenchSnapshot{
		Generation: s.generation,
		State:      state,
		Measured:   measured,
		Twin:       twin,
		Lineage:    lineage,
		Results:    results,
	}
	return cloneWorkbenchValue(snapshot)
}

type PostgresWorkbenchStore struct {
	db                      *sql.DB
	afterSnapshotGeneration func()
}

type workbenchSQLQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewPostgresWorkbenchStore(dsn string) (*PostgresWorkbenchStore, error) {
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
		return nil, fmt.Errorf("ping workbench postgres store: %w", err)
	}
	if err := ensureWorkbenchSnapshotSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate workbench Snapshot schema: %w", err)
	}
	return &PostgresWorkbenchStore{db: db}, nil
}

func ensureWorkbenchSnapshotSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS workbench_snapshot_generation (
			singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			generation BIGINT NOT NULL CHECK (generation >= 0),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO workbench_snapshot_generation (singleton, generation)
		VALUES (TRUE, 0)
		ON CONFLICT (singleton) DO NOTHING;
		CREATE TABLE IF NOT EXISTS workbench_twin_publications (
			publication_id TEXT PRIMARY KEY,
			publication JSONB NOT NULL,
			persisted_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func (s *PostgresWorkbenchStore) SaveResidentSource(source ScadaResidentSourceDeclaration) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	for index := range source.Tags {
		source.Tags[index].SourceID = source.SourceID
		source.Tags[index].ReactorID = source.ReactorID
	}
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
	if err := advanceWorkbenchSnapshotGeneration(ctx, tx); err != nil {
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
	if err := advanceWorkbenchSnapshotGeneration(ctx, tx); err != nil {
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
	if projection.PublicationID != "" {
		processed, err = insertWorkbenchProcessed(ctx, tx, "twin-publication", projection.PublicationID, 0, 0)
		if err != nil || !processed {
			return processed, err
		}
	} else if consumerName != "" {
		processed, err = insertWorkbenchProcessed(ctx, tx, consumerName, projection.RedpandaTopic, projection.RedpandaPartition, projection.RedpandaOffset)
		if err != nil || !processed {
			return processed, err
		}
	}
	if projection.LineagePresent {
		if _, err := tx.ExecContext(ctx, `DELETE FROM digital_twin_lineage`); err != nil {
			return false, err
		}
		for _, lineage := range projection.Lineage {
			lineageRaw, err := json.Marshal(lineage)
			if err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `
			INSERT INTO digital_twin_lineage (lineage_id, value_id, value_basis, lineage, updated_at)
			VALUES ($1,$2,$3,$4::jsonb,now())
			ON CONFLICT (lineage_id) DO UPDATE
			SET value_id = EXCLUDED.value_id,
			    value_basis = EXCLUDED.value_basis,
			    lineage = EXCLUDED.lineage,
			    updated_at = now()
			`, lineage.LineageID, lineage.ValueID, string(lineage.ValueBasis), lineageRaw); err != nil {
				return false, err
			}
		}
	}
	stateRaw, err := json.Marshal(projection.State)
	if err != nil {
		return false, err
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
	if projection.PublicationID != "" && consumerName == "" {
		publicationRaw, err := json.Marshal(TwinStatePublication{
			PublicationID: projection.PublicationID, Source: projection.PublicationSource,
			TwinStateTopic: projection.RedpandaTopic, State: projection.State, Lineage: projection.Lineage,
		})
		if err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workbench_twin_publications (publication_id, publication, persisted_at)
			VALUES ($1, $2::jsonb, now())
			ON CONFLICT (publication_id) DO NOTHING
		`, projection.PublicationID, publicationRaw); err != nil {
			return false, err
		}
	}
	if err := advanceWorkbenchSnapshotGeneration(ctx, tx); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *PostgresWorkbenchStore) GetTwinStatePublication(publicationID string) (TwinStatePublication, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT publication FROM workbench_twin_publications WHERE publication_id = $1`, publicationID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TwinStatePublication{}, ErrWorkbenchNotFound
		}
		return TwinStatePublication{}, err
	}
	var publication TwinStatePublication
	if err := json.Unmarshal(raw, &publication); err != nil {
		return TwinStatePublication{}, err
	}
	return publication, nil
}

func (s *PostgresWorkbenchStore) AcknowledgeTwinStatePublication(publicationID string) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	_, err := s.db.ExecContext(ctx, `
		UPDATE workbench_twin_publications
		SET publication = jsonb_build_object(
			'publicationId', publication->>'publicationId',
			'source', publication->'source',
			'twinStateTopic', publication->>'twinStateTopic',
			'acknowledged', true
		)
		WHERE publication_id = $1
	`, publicationID)
	return err
}

func (s *PostgresWorkbenchStore) LatestMeasuredFrames(limit int) ([]ScadaTelemetryFrame, error) {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	return latestMeasuredFrames(ctx, s.db, limit)
}

func (s *PostgresWorkbenchStore) PruneDynamicMeasured(before time.Time) error {
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		DELETE FROM scada_measured_frames
		WHERE COALESCE(frame->>'reactorId', '') <> '' AND created_at < $1
	`, before.UTC())
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE workbench_snapshot_generation
			SET generation = generation + 1, updated_at = now()
			WHERE singleton = TRUE
		`); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func latestMeasuredFrames(ctx context.Context, queryer workbenchSQLQueryer, limit int) ([]ScadaTelemetryFrame, error) {
	rows, err := queryer.QueryContext(ctx, `
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
	return latestResultFrames(ctx, s.db, limit)
}

func latestResultFrames(ctx context.Context, queryer workbenchSQLQueryer, limit int) ([]SimopsResultFrame, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT frame
		FROM (
			SELECT DISTINCT ON (run_id, worker_id, sequence)
				run_id, worker_id, sequence, produced_at, frame
			FROM simops_result_values
			ORDER BY run_id, worker_id, sequence, produced_at DESC
		) latest
		ORDER BY produced_at DESC, run_id, worker_id, sequence
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
	return currentTwinState(ctx, s.db)
}

func currentTwinState(ctx context.Context, queryer workbenchSQLQueryer) (DigitalTwinState, error) {
	var raw []byte
	if err := queryer.QueryRowContext(ctx, `
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
	return lineageForValue(ctx, s.db, valueID)
}

func lineageForValue(ctx context.Context, queryer workbenchSQLQueryer, valueID string) (DigitalTwinValueLineage, error) {
	var raw []byte
	if err := queryer.QueryRowContext(ctx, `SELECT lineage FROM digital_twin_lineage WHERE value_id = $1`, valueID).Scan(&raw); err != nil {
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
	ctx, cancel := simopsSQLContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, workbenchSnapshotTxOptions())
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	defer func() { _ = tx.Rollback() }()

	generation, err := workbenchSnapshotGeneration(ctx, tx)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	if s.afterSnapshotGeneration != nil {
		s.afterSnapshotGeneration()
	}
	measured, err := latestMeasuredFrames(ctx, tx, 100)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	results, err := latestResultFrames(ctx, tx, 20)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	twin, err := currentTwinState(ctx, tx)
	if err != nil && !errors.Is(err, ErrWorkbenchNotFound) {
		return WorkbenchSnapshot{}, err
	}
	lineage, err := allWorkbenchLineage(ctx, tx)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}
	state := buildWorkbenchState(measured, results, twin)
	state.SnapshotGeneration = generation
	if err := tx.Commit(); err != nil {
		return WorkbenchSnapshot{}, err
	}
	return WorkbenchSnapshot{
		Generation: generation,
		State:      state,
		Measured:   measured,
		Twin:       twin,
		Lineage:    lineage,
		Results:    results,
	}, nil
}

func workbenchSnapshotTxOptions() *sql.TxOptions {
	return &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
}

func workbenchSnapshotGeneration(ctx context.Context, queryer workbenchSQLQueryer) (uint64, error) {
	var generation uint64
	err := queryer.QueryRowContext(ctx, `SELECT generation FROM workbench_snapshot_generation WHERE singleton = TRUE`).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return generation, err
}

func allWorkbenchLineage(ctx context.Context, queryer workbenchSQLQueryer) ([]DigitalTwinValueLineage, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT lineage FROM digital_twin_lineage ORDER BY lineage_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lineage := []DigitalTwinValueLineage{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var record DigitalTwinValueLineage
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		lineage = append(lineage, record)
	}
	return lineage, rows.Err()
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

func advanceWorkbenchSnapshotGeneration(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workbench_snapshot_generation (singleton, generation, updated_at)
		VALUES (TRUE, 1, now())
		ON CONFLICT (singleton) DO UPDATE
		SET generation = workbench_snapshot_generation.generation + 1,
		    updated_at = now()
	`)
	return err
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

func cloneWorkbenchValue[T any](value T) (T, error) {
	var clone T
	raw, err := json.Marshal(value)
	if err != nil {
		return clone, err
	}
	if err := json.Unmarshal(raw, &clone); err != nil {
		return clone, err
	}
	return clone, nil
}
