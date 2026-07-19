package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

func RunWorkbenchScadaProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchScadaProjectionPersistence, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ScadaTopic, cfg.ScadaProjectionConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[ScadaProjection]{
		Stream: ProjectionStreamMeasuredState,
		Project: func(message SimopsBrokerMessage) (ScadaProjection, error) {
			return ProjectScadaFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection ScadaProjection) (uint64, error) {
			written, err := store.SaveScadaProjection(cfg.ScadaProjectionConsumerGroup, projection)
			return boolCount(written), err
		},
	})
}

func RunWorkbenchResultProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchResultProjectionPersistence, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ResultsTopic, cfg.ResultProjectionConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[SimopsResultProjection]{
		Stream: ProjectionStreamSimulatedResultState,
		Project: func(message SimopsBrokerMessage) (SimopsResultProjection, error) {
			return ProjectSimopsResultFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection SimopsResultProjection) (uint64, error) {
			written, err := store.SaveResultProjection(cfg.ResultProjectionConsumerGroup, projection)
			return boolCount(written), err
		},
	})
}

func RunWorkbenchTwinProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchTwinProjectionPersistence, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.TwinStateTopic, cfg.TwinProjectionConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[TwinStateProjection]{
		Stream: ProjectionStreamTwinState,
		Project: func(message SimopsBrokerMessage) (TwinStateProjection, error) {
			return ProjectTwinState(message.Topic, message.Partition, message.Offset, message.Value, message.Headers...)
		},
		Persist: func(projection TwinStateProjection) (uint64, error) {
			written, err := store.SaveTwinStateProjection(cfg.TwinProjectionConsumerGroup, projection)
			return boolCount(written), err
		},
	})
}

type WorkbenchIcebergProjectionAppender interface {
	AppendScada(context.Context, ScadaProjection) error
	AppendResult(context.Context, SimopsResultProjection) error
	AppendTwin(context.Context, TwinStateProjection) error
}

func RunWorkbenchScadaIcebergConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, writer WorkbenchIcebergProjectionAppender, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ScadaTopic, cfg.IcebergConsumerGroup+"-scada")
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if writer == nil {
		return fmt.Errorf("workbench measured-state iceberg ingestion requires a writer")
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[ScadaProjection]{
		Stream: ProjectionStreamMeasuredState, WriteStage: ProjectionIngestionAppend,
		Project: func(message SimopsBrokerMessage) (ScadaProjection, error) {
			return ProjectScadaFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection ScadaProjection) (uint64, error) {
			return 1, writer.AppendScada(ctx, projection)
		},
	})
}

func RunWorkbenchResultIcebergConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, writer WorkbenchIcebergProjectionAppender, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ResultsTopic, cfg.IcebergConsumerGroup+"-results")
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if writer == nil {
		return fmt.Errorf("workbench simulated-result-state iceberg ingestion requires a writer")
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[SimopsResultProjection]{
		Stream: ProjectionStreamSimulatedResultState, WriteStage: ProjectionIngestionAppend,
		Project: func(message SimopsBrokerMessage) (SimopsResultProjection, error) {
			return ProjectSimopsResultFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection SimopsResultProjection) (uint64, error) {
			return 1, writer.AppendResult(ctx, projection)
		},
	})
}

func RunWorkbenchTwinIcebergConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, writer WorkbenchIcebergProjectionAppender, metrics *SimopsConsumerMetrics) error {
	if reader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.TwinStateTopic, cfg.IcebergConsumerGroup+"-twin")
		if err != nil {
			return err
		}
		reader = created
		defer reader.Close()
	}
	if writer == nil {
		return fmt.Errorf("workbench twin-state iceberg ingestion requires a writer")
	}
	return RunProjectionIngestion(ctx, reader, metrics, ProjectionIngestionAdapter[TwinStateProjection]{
		Stream: ProjectionStreamTwinState, WriteStage: ProjectionIngestionAppend,
		Project: func(message SimopsBrokerMessage) (TwinStateProjection, error) {
			return ProjectTwinState(message.Topic, message.Partition, message.Offset, message.Value, message.Headers...)
		},
		Persist: func(projection TwinStateProjection) (uint64, error) {
			return 1, writer.AppendTwin(ctx, projection)
		},
	})
}

func boolCount(written bool) uint64 {
	if written {
		return 1
	}
	return 0
}

type TwinProjector struct {
	cfg       WorkbenchConfig
	publisher TwinStatePublisher
	now       func() time.Time

	mu           sync.Mutex
	transitionMu sync.Mutex
	measured     map[string]ScadaTelemetryFrame
	result       *SimopsResultFrame
}

type TwinProjectorPersistence interface {
	TwinStatePublicationStore
	LatestMeasuredFrames(int) ([]ScadaTelemetryFrame, error)
	LatestResultFrames(int) ([]SimopsResultFrame, error)
}

func NewTwinProjector(cfg WorkbenchConfig, store TwinProjectorPersistence, eventLog TwinStatePublicationEventLog) (*TwinProjector, error) {
	if store == nil {
		store = NewInMemoryWorkbenchStore()
	}
	projector := &TwinProjector{
		cfg:       cfg,
		publisher: NewTwinStatePublisher(store, eventLog),
		now:       time.Now,
		measured:  make(map[string]ScadaTelemetryFrame),
	}
	measured, err := store.LatestMeasuredFrames(100)
	if err != nil {
		return nil, fmt.Errorf("hydrate Twin projector measured state: %w", err)
	}
	for _, frame := range measured {
		projector.measured[frame.TagID] = frame
	}
	results, err := store.LatestResultFrames(1)
	if err != nil {
		return nil, fmt.Errorf("hydrate Twin projector simulated state: %w", err)
	}
	if len(results) > 0 {
		result := results[0]
		projector.result = &result
	}
	return projector, nil
}

func RunTwinProjector(ctx context.Context, cfg WorkbenchConfig, scadaReader SimopsKafkaReader, resultReader SimopsKafkaReader, store TwinProjectorPersistence, eventLog TwinStatePublicationEventLog, metrics *SimopsConsumerMetrics) error {
	projector, err := NewTwinProjector(cfg, store, eventLog)
	if err != nil {
		return err
	}
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if scadaReader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ScadaTopic, cfg.TwinProjectorScadaGroup)
		if err != nil {
			return err
		}
		scadaReader = created
		defer scadaReader.Close()
	}
	if resultReader == nil {
		created, err := NewWorkbenchKafkaReader(cfg, cfg.ResultsTopic, cfg.TwinProjectorResultGroup)
		if err != nil {
			return err
		}
		resultReader = created
		defer resultReader.Close()
	}

	return RunBackgroundConsumers(ctx,
		BackgroundConsumer{Name: "measured-state-input", Consume: func(ctx context.Context) error {
			return projector.runScada(ctx, scadaReader, metrics)
		}},
		BackgroundConsumer{Name: "simulated-result-state-input", Consume: func(ctx context.Context) error {
			return projector.runResults(ctx, resultReader, metrics)
		}},
	)
}

func (p *TwinProjector) runScada(ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		source := WorkbenchProjectionPosition{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset}
		if err := p.consumeScada(ctx, reader, metrics, msg, source); err != nil {
			return err
		}
	}
}

func (p *TwinProjector) runResults(ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		source := WorkbenchProjectionPosition{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset}
		if err := p.consumeResult(ctx, reader, metrics, msg, source); err != nil {
			return err
		}
	}
}

func (p *TwinProjector) consumeScada(ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics, msg SimopsBrokerMessage, source WorkbenchProjectionPosition) error {
	p.transitionMu.Lock()
	defer p.transitionMu.Unlock()
	published, err := p.processScada(ctx, msg, source)
	return p.finishSourceMessage(ctx, reader, metrics, msg, source, published, err)
}

func (p *TwinProjector) consumeResult(ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics, msg SimopsBrokerMessage, source WorkbenchProjectionPosition) error {
	p.transitionMu.Lock()
	defer p.transitionMu.Unlock()
	published, err := p.processResult(ctx, msg, source)
	return p.finishSourceMessage(ctx, reader, metrics, msg, source, published, err)
}

func (p *TwinProjector) finishSourceMessage(ctx context.Context, reader SimopsKafkaReader, metrics *SimopsConsumerMetrics, msg SimopsBrokerMessage, source WorkbenchProjectionPosition, published bool, publicationErr error) error {
	if publicationErr != nil {
		return publicationErr
	}
	if published {
		metrics.IncFramesWritten(1)
		if err := p.publisher.Acknowledge(source); err != nil {
			return err
		}
	}
	if err := reader.CommitMessages(ctx, msg); err != nil {
		return err
	}
	metrics.MarkConsumed(msg.Offset)
	return nil
}

func (p *TwinProjector) processScada(ctx context.Context, msg SimopsBrokerMessage, source WorkbenchProjectionPosition) (bool, error) {
	if _, resumed, err := p.publisher.Resume(ctx, source); err != nil || resumed {
		return resumed, err
	}
	projection, err := ProjectScadaFrame(msg.Topic, msg.Partition, msg.Offset, msg.Value)
	if err != nil {
		return false, err
	}
	state, lineage, ok := p.applyScada(projection.Frame)
	if !ok {
		return false, nil
	}
	_, err = p.publisher.Publish(ctx, NewTwinStatePublication(source, p.cfg.TwinStateTopic, state, lineage))
	return err == nil, err
}

func (p *TwinProjector) processResult(ctx context.Context, msg SimopsBrokerMessage, source WorkbenchProjectionPosition) (bool, error) {
	if _, resumed, err := p.publisher.Resume(ctx, source); err != nil || resumed {
		return resumed, err
	}
	projection, err := ProjectSimopsResultFrame(msg.Topic, msg.Partition, msg.Offset, msg.Value)
	if err != nil {
		return false, err
	}
	state, lineage, ok := p.applyResult(projection.Frame)
	if !ok {
		return false, nil
	}
	_, err = p.publisher.Publish(ctx, NewTwinStatePublication(source, p.cfg.TwinStateTopic, state, lineage))
	return err == nil, err
}

func (p *TwinProjector) applyScada(frame ScadaTelemetryFrame) (DigitalTwinState, []DigitalTwinValueLineage, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.measured[frame.TagID] = frame
	if p.result == nil || len(p.result.Values) == 0 {
		return DigitalTwinState{}, nil, false
	}
	measured := make([]ScadaTelemetryFrame, 0, len(p.measured))
	for _, existing := range p.measured {
		measured = append(measured, existing)
	}
	state, lineage := BuildTwinStateFromData(measured, *p.result, p.now().UTC())
	return state, lineage, true
}

func (p *TwinProjector) applyResult(result SimopsResultFrame) (DigitalTwinState, []DigitalTwinValueLineage, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.result = &result
	measured := make([]ScadaTelemetryFrame, 0, len(p.measured))
	for _, frame := range p.measured {
		measured = append(measured, frame)
	}
	if len(measured) == 0 || len(result.Values) == 0 {
		return DigitalTwinState{}, nil, false
	}
	state, lineage := BuildTwinStateFromData(measured, result, p.now().UTC())
	return state, lineage, true
}

func BuildTwinStateFromData(measured []ScadaTelemetryFrame, result SimopsResultFrame, asOf time.Time) (DigitalTwinState, []DigitalTwinValueLineage) {
	entities := map[string]*DigitalTwinEntity{}
	lineage := []DigitalTwinValueLineage{}
	sourceIDs := []string{}

	for _, frame := range measured {
		entity := ensureTwinEntity(entities, frame.AssetID, displayNameForAsset(frame.AssetID))
		valueID := WorkbenchMeasuredValueIDPrefix + strings.TrimPrefix(frame.TagID, "TAG-")
		lineageID := WorkbenchMeasuredLineageIDPrefix + strings.TrimPrefix(frame.TagID, "TAG-")
		entity.Values = append(entity.Values, DigitalTwinValue{
			ValueID:    valueID,
			Label:      frame.TagID,
			ValueBasis: WorkbenchValueMeasured,
			Unit:       frame.Unit,
			Value:      frame.Value,
			Confidence: 0.92,
			Freshness:  TwinFreshness{AgeSec: 0, Status: "fresh"},
			LineageID:  lineageID,
			SourceIDs:  []string{frame.TagID},
		})
		sourceIDs = append(sourceIDs, frame.TagID)
		lineage = append(lineage, DigitalTwinValueLineage{
			SchemaVersion: WorkbenchLineageSchemaVersion,
			LineageID:     lineageID,
			ValueID:       valueID,
			ValueBasis:    WorkbenchValueMeasured,
			Inputs: []TwinLineageInput{{
				SourceKind: "scada-tag",
				SourceID:   frame.TagID,
				ValueBasis: WorkbenchValueMeasured,
			}},
			ProcessingSteps: []string{"Accept resident public-safe measured stand-in frame"},
			Artifacts:       []TwinLineageArtifact{},
		})
	}

	first := result.Values[0]
	if expected, ok := simopsResultValueByID(result.Values, WorkbenchSimulatedMarginValue); ok {
		first = expected
	}
	resultValue := rawObject(first.Value)
	resultEntity := ensureTwinEntity(entities, first.EntityID, displayNameForAsset(first.EntityID))
	resultEntity.Values = append(resultEntity.Values, DigitalTwinValue{
		ValueID:    first.ValueID,
		Label:      first.Label,
		ValueBasis: WorkbenchValueSimulated,
		Unit:       first.Unit,
		Value:      resultValue,
		Confidence: first.Confidence,
		Freshness:  TwinFreshness{AgeSec: 0, Status: "unknown"},
		LineageID:  WorkbenchSimulatedMarginLineage,
		SourceIDs:  []string{result.RunID},
	})
	lineage = append(lineage, DigitalTwinValueLineage{
		SchemaVersion: WorkbenchLineageSchemaVersion,
		LineageID:     WorkbenchSimulatedMarginLineage,
		ValueID:       first.ValueID,
		ValueBasis:    WorkbenchValueSimulated,
		Inputs: append([]TwinLineageInput{{
			SourceKind: "simulation-run",
			SourceID:   result.RunID,
			ValueBasis: WorkbenchValueSimulated,
		}}, result.LineageInputs...),
		ProcessingSteps: []string{"Accept run-scoped synthetic simulated result frame"},
		Artifacts:       append([]TwinLineageArtifact(nil), result.LineageArtifacts...),
	})

	imputed := imputedMarginValue(resultValue)
	resultEntity.Values = append(resultEntity.Values, DigitalTwinValue{
		ValueID:    WorkbenchImputedCoreMarginValue,
		Label:      "Imputed local margin",
		ValueBasis: WorkbenchValueImputed,
		Unit:       first.Unit,
		Value:      map[string]any{"scalar": imputed},
		Confidence: clampConfidence(first.Confidence - 0.05),
		Freshness:  TwinFreshness{AgeSec: 0, Status: "degraded"},
		LineageID:  WorkbenchCoreMarginLineage,
		SourceIDs:  append(sourceIDs, result.RunID, result.ModelID),
	})
	inputs := make([]TwinLineageInput, 0, len(sourceIDs)+2)
	for _, sourceID := range sourceIDs {
		inputs = append(inputs, TwinLineageInput{SourceKind: "scada-tag", SourceID: sourceID, ValueBasis: WorkbenchValueMeasured})
	}
	inputs = append(inputs,
		TwinLineageInput{SourceKind: "simulation-run", SourceID: result.RunID, ValueBasis: WorkbenchValueSimulated},
		TwinLineageInput{SourceKind: "digital-twin-model", SourceID: result.ModelID, ValueBasis: WorkbenchValueImputed},
	)
	lineage = append(lineage, DigitalTwinValueLineage{
		SchemaVersion: WorkbenchLineageSchemaVersion,
		LineageID:     WorkbenchCoreMarginLineage,
		ValueID:       WorkbenchImputedCoreMarginValue,
		ValueBasis:    WorkbenchValueImputed,
		Inputs:        inputs,
		ProcessingSteps: []string{
			"Read recent measured stand-in tags",
			"Read run-scoped simulated result frame",
			"Apply public-safe twin projection model without claiming validated physics",
		},
		Artifacts: append([]TwinLineageArtifact(nil), result.LineageArtifacts...),
	})

	ordered := make([]DigitalTwinEntity, 0, len(entities))
	for _, entity := range entities {
		sortTwinValues(entity.Values)
		ordered = append(ordered, *entity)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].EntityID < ordered[j].EntityID })
	return DigitalTwinState{
		SchemaVersion: WorkbenchTwinStateSchemaVersion,
		TwinID:        WorkbenchDefaultTwinID,
		AsOf:          asOf.UTC(),
		Entities:      ordered,
	}, lineage
}

func simopsResultValueByID(values []SimopsResultValue, valueID string) (SimopsResultValue, bool) {
	for _, value := range values {
		if value.ValueID == valueID {
			return value, true
		}
	}
	return SimopsResultValue{}, false
}

func ensureTwinEntity(entities map[string]*DigitalTwinEntity, entityID string, displayName string) *DigitalTwinEntity {
	if entity, ok := entities[entityID]; ok {
		return entity
	}
	entity := &DigitalTwinEntity{EntityID: entityID, DisplayName: displayName}
	entities[entityID] = entity
	return entity
}

func displayNameForAsset(assetID string) string {
	switch assetID {
	case "ASSET-CORE-A":
		return "Synthetic core region A"
	case "ASSET-THERMAL-LOOP-A":
		return "Synthetic thermal loop A"
	default:
		return assetID
	}
}

func rawObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func imputedMarginValue(result map[string]any) float64 {
	scalar, _ := result["scalar"].(float64)
	if scalar == 0 {
		return 0
	}
	return math.Round((scalar+2.5)*10) / 10
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func sortTwinValues(values []DigitalTwinValue) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].ValueBasis == values[j].ValueBasis {
			return values[i].ValueID < values[j].ValueID
		}
		return values[i].ValueBasis < values[j].ValueBasis
	})
}
