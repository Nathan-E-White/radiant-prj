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

func RunWorkbenchScadaProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchStore, metrics *SimopsConsumerMetrics) error {
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
	return RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[ScadaProjection]{
		Stream: WorkbenchProjectionMeasured,
		Project: func(message SimopsBrokerMessage) (ScadaProjection, error) {
			return ProjectScadaFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection ScadaProjection) (bool, error) {
			return store.SaveScadaProjection(cfg.ScadaProjectionConsumerGroup, projection)
		},
	})
}

func RunWorkbenchResultProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchStore, metrics *SimopsConsumerMetrics) error {
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
	return RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[SimopsResultProjection]{
		Stream: WorkbenchProjectionSimulated,
		Project: func(message SimopsBrokerMessage) (SimopsResultProjection, error) {
			return ProjectSimopsResultFrame(message.Topic, message.Partition, message.Offset, message.Value)
		},
		Persist: func(projection SimopsResultProjection) (bool, error) {
			return store.SaveResultProjection(cfg.ResultProjectionConsumerGroup, projection)
		},
	})
}

func RunWorkbenchTwinProjectionConsumer(ctx context.Context, cfg WorkbenchConfig, reader SimopsKafkaReader, store WorkbenchStore, metrics *SimopsConsumerMetrics) error {
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
	return RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[TwinStateProjection]{
		Stream: WorkbenchProjectionTwin,
		Project: func(message SimopsBrokerMessage) (TwinStateProjection, error) {
			return ProjectTwinState(message.Topic, message.Partition, message.Offset, message.Value, message.Headers...)
		},
		Persist: func(projection TwinStateProjection) (bool, error) {
			return store.SaveTwinStateProjection(cfg.TwinProjectionConsumerGroup, projection)
		},
	})
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

func NewTwinProjector(cfg WorkbenchConfig, store WorkbenchStore, eventLog WorkbenchEventLog) (*TwinProjector, error) {
	if store == nil {
		store = NewInMemoryWorkbenchStore()
	}
	if eventLog == nil {
		eventLog = &MemoryWorkbenchEventLog{Store: store}
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

func RunTwinProjector(ctx context.Context, cfg WorkbenchConfig, scadaReader SimopsKafkaReader, resultReader SimopsKafkaReader, store WorkbenchStore, eventLog WorkbenchEventLog, metrics *SimopsConsumerMetrics) error {
	projector, err := NewTwinProjector(cfg, store, eventLog)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
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

	errs := make(chan error, 2)
	go func() { errs <- projector.runScada(ctx, scadaReader, metrics) }()
	go func() { errs <- projector.runResults(ctx, resultReader, metrics) }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
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
