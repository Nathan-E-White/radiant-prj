package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxWorkbenchBodyBytes = 256 * 1024

type WorkbenchController struct {
	cfg      WorkbenchConfig
	store    WorkbenchStore
	eventLog WorkbenchEventLog
	now      func() time.Time
}

func NewDefaultWorkbenchController(cfg WorkbenchConfig) (*WorkbenchController, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var store WorkbenchStore = NewInMemoryWorkbenchStore()
	if cfg.Store == "postgres" {
		postgresStore, err := NewPostgresWorkbenchStore(cfg.PostgresDSN)
		if err != nil {
			return nil, err
		}
		store = postgresStore
	}
	var eventLog WorkbenchEventLog = &MemoryWorkbenchEventLog{Store: store}
	if cfg.EventLog == "redpanda" {
		redpandaLog, err := NewRedpandaWorkbenchEventLog(cfg)
		if err != nil {
			return nil, err
		}
		eventLog = redpandaLog
	}
	return NewWorkbenchController(cfg, store, eventLog), nil
}

func NewWorkbenchController(cfg WorkbenchConfig, store WorkbenchStore, eventLog WorkbenchEventLog) *WorkbenchController {
	if store == nil {
		store = NewInMemoryWorkbenchStore()
	}
	if eventLog == nil {
		eventLog = &MemoryWorkbenchEventLog{Store: store}
	}
	return &WorkbenchController{
		cfg:      cfg,
		store:    store,
		eventLog: eventLog,
		now:      time.Now,
	}
}

func (c *WorkbenchController) RegisterSource(source ScadaResidentSourceDeclaration) (int, error) {
	if err := validateResidentSource(source); err != nil {
		return http.StatusUnprocessableEntity, err
	}
	if err := c.store.SaveResidentSource(source); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusAccepted, nil
}

func (c *WorkbenchController) IngestScada(ctx context.Context, body io.Reader) (int, int, error) {
	frames, err := decodeScadaFrames(body)
	if err != nil {
		return 0, http.StatusBadRequest, err
	}
	if len(frames) == 0 {
		return 0, http.StatusBadRequest, fmt.Errorf("at least one SCADA telemetry frame is required")
	}
	for _, frame := range frames {
		if err := c.validateScadaFrame(frame); err != nil {
			return 0, http.StatusUnprocessableEntity, err
		}
		if err := c.eventLog.PublishScada(ctx, frame); err != nil {
			return 0, http.StatusBadGateway, err
		}
	}
	return len(frames), http.StatusAccepted, nil
}

func (c *WorkbenchController) IngestResults(ctx context.Context, run SimopsRunRecord, body io.Reader) (int, int, error) {
	results, err := decodeResultFrames(body)
	if err != nil {
		return 0, http.StatusBadRequest, err
	}
	if len(results) == 0 {
		return 0, http.StatusBadRequest, fmt.Errorf("at least one simulated result frame is required")
	}
	for _, result := range results {
		if err := validateSimopsResultFrame(run, result); err != nil {
			return 0, http.StatusUnprocessableEntity, err
		}
		if err := c.eventLog.PublishResult(ctx, result); err != nil {
			return 0, http.StatusBadGateway, err
		}
	}
	return len(results), http.StatusAccepted, nil
}

func (c *WorkbenchController) Snapshot() (WorkbenchSnapshot, error) {
	return c.store.Snapshot()
}

func (c *WorkbenchController) Measured() ([]ScadaTelemetryFrame, error) {
	return c.store.LatestMeasuredFrames(100)
}

func (c *WorkbenchController) Twin() (DigitalTwinState, error) {
	return c.store.CurrentTwinState()
}

func (c *WorkbenchController) Lineage(valueID string) (DigitalTwinValueLineage, error) {
	return c.store.LineageForValue(valueID)
}

func (c *WorkbenchController) validateScadaFrame(frame ScadaTelemetryFrame) error {
	if frame.SchemaVersion != WorkbenchScadaSchemaVersion {
		return fmt.Errorf("unsupported SCADA telemetry schemaVersion")
	}
	if strings.TrimSpace(frame.SourceID) == "" {
		return fmt.Errorf("scada sourceId is required")
	}
	tag, err := c.store.GetResidentTag(frame.TagID)
	if errors.Is(err, ErrWorkbenchNotFound) {
		return fmt.Errorf("scada tagId %s has no resident source declaration", frame.TagID)
	}
	if err != nil {
		return err
	}
	if tag.ValueBasis != WorkbenchValueMeasured {
		return fmt.Errorf("resident source tag must be measured")
	}
	if frame.ValueBasis != WorkbenchValueMeasured {
		return fmt.Errorf("scada telemetry must stay valueBasis=measured")
	}
	if frame.SyntheticStatus != WorkbenchSyntheticPublicStandin {
		return fmt.Errorf("scada telemetry must stay public-safe stand-in data")
	}
	if frame.AssetID != tag.AssetID || frame.SignalKind != tag.SignalKind || frame.Unit != tag.Unit {
		return fmt.Errorf("scada telemetry does not match resident tag declaration")
	}
	if frame.Sequence == 0 {
		return fmt.Errorf("scada telemetry sequence must be positive")
	}
	if frame.SampledAt.IsZero() || frame.ObservedAt.IsZero() {
		return fmt.Errorf("scada telemetry timestamps are required")
	}
	if frame.Value == nil {
		return fmt.Errorf("scada telemetry value is required")
	}
	return nil
}

func validateResidentSource(source ScadaResidentSourceDeclaration) error {
	if source.SchemaVersion != WorkbenchSourceSchemaVersion {
		return fmt.Errorf("unsupported resident source schemaVersion")
	}
	if source.Lifecycle != "resident" {
		return fmt.Errorf("resident source lifecycle must be resident")
	}
	if source.SyntheticStatus != WorkbenchSyntheticPublicStandin {
		return fmt.Errorf("resident source must stay public-safe stand-in data")
	}
	if source.Ingest.Topic != "scada.telemetry.v1" {
		return fmt.Errorf("resident source ingest topic must be scada.telemetry.v1")
	}
	if len(source.Tags) == 0 {
		return fmt.Errorf("resident source must declare at least one tag")
	}
	for _, tag := range source.Tags {
		if tag.ValueBasis != WorkbenchValueMeasured {
			return fmt.Errorf("resident source tag %s must stay valueBasis=measured", tag.TagID)
		}
	}
	return nil
}

func validateSimopsResultFrame(run SimopsRunRecord, frame SimopsResultFrame) error {
	if frame.SchemaVersion != WorkbenchResultSchemaVersion {
		return fmt.Errorf("unsupported simulated result schemaVersion")
	}
	if frame.RunID != run.RunID {
		return fmt.Errorf("simulated result runId does not match ingest path")
	}
	if frame.ScenarioID != run.ScenarioID {
		return fmt.Errorf("simulated result scenarioId does not match run")
	}
	if !allowedWorker(frame.WorkerKind) {
		return fmt.Errorf("simulated result workerKind is not supported")
	}
	if strings.TrimSpace(frame.WorkerID) == "" {
		return fmt.Errorf("simulated result workerId is required")
	}
	if frame.Sequence == 0 {
		return fmt.Errorf("simulated result sequence must be positive")
	}
	if strings.TrimSpace(frame.ProducedAt) == "" {
		return fmt.Errorf("simulated result producedAt is required")
	}
	if frame.ValueBasis != WorkbenchValueSimulated {
		return fmt.Errorf("SimOps workers must emit valueBasis=simulated results")
	}
	if frame.ValueBasis == WorkbenchValueImputed {
		return fmt.Errorf("imputed state must be produced by the digital twin projector")
	}
	if frame.SyntheticStatus != WorkbenchSyntheticPublicStandin {
		return fmt.Errorf("simulated result must stay public-safe stand-in data")
	}
	if strings.TrimSpace(frame.ModelID) == "" {
		return fmt.Errorf("simulated result modelId is required")
	}
	if len(frame.Values) == 0 {
		return fmt.Errorf("simulated result values are required")
	}
	for _, value := range frame.Values {
		if strings.TrimSpace(value.ResultID) == "" || strings.TrimSpace(value.ValueID) == "" {
			return fmt.Errorf("simulated result values must include resultId and valueId")
		}
		if value.Confidence < 0 || value.Confidence > 1 {
			return fmt.Errorf("simulated result confidence must be between 0 and 1")
		}
		if len(value.Value) == 0 || string(value.Value) == "null" {
			return fmt.Errorf("simulated result value payload is required")
		}
	}
	return nil
}

func decodeScadaFrames(body io.Reader) ([]ScadaTelemetryFrame, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxWorkbenchBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxWorkbenchBodyBytes {
		return nil, fmt.Errorf("SCADA payload exceeds %d bytes", maxWorkbenchBodyBytes)
	}
	var batch ScadaTelemetryBatch
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err == nil && len(batch.Frames) > 0 {
		return batch.Frames, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), maxWorkbenchBodyBytes)
	frames := []ScadaTelemetryFrame{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame ScadaTelemetryFrame
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			return nil, fmt.Errorf("invalid SCADA NDJSON frame: %w", err)
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return frames, nil
}

func decodeResultFrames(body io.Reader) ([]SimopsResultFrame, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxWorkbenchBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxWorkbenchBodyBytes {
		return nil, fmt.Errorf("simulated result payload exceeds %d bytes", maxWorkbenchBodyBytes)
	}
	var batch SimopsResultBatch
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err == nil && len(batch.Results) > 0 {
		return batch.Results, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), maxWorkbenchBodyBytes)
	results := []SimopsResultFrame{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame SimopsResultFrame
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			return nil, fmt.Errorf("invalid simulated result NDJSON frame: %w", err)
		}
		results = append(results, frame)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
