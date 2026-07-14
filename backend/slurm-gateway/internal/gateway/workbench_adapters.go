package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	workbenchTwinLineageHeader       = "radiant-workbench-lineage-v1"
	workbenchTwinPublicationIDHeader = "radiant-twin-publication-id-v1"
)

type WorkbenchEventLog interface {
	PublishScada(ctx context.Context, frame ScadaTelemetryFrame) error
	PublishResult(ctx context.Context, frame SimopsResultFrame) error
	PublishTwinState(ctx context.Context, publication TwinStatePublication) error
}

type MemoryWorkbenchEventLog struct {
	Store  WorkbenchStore
	offset atomic.Int64
}

func (l *MemoryWorkbenchEventLog) nextOffset() int64 {
	return l.offset.Add(1) - 1
}

func (l *MemoryWorkbenchEventLog) PublishScada(ctx context.Context, frame ScadaTelemetryFrame) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Store == nil {
		return nil
	}
	raw, _ := json.Marshal(frame)
	projection, err := ProjectScadaFrame("memory.scada.telemetry.v1", 0, l.nextOffset(), raw)
	if err != nil {
		return err
	}
	_, err = l.Store.SaveScadaProjection("", projection)
	return err
}

func (l *MemoryWorkbenchEventLog) PublishResult(ctx context.Context, frame SimopsResultFrame) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Store == nil {
		return nil
	}
	raw, _ := json.Marshal(frame)
	projection, err := ProjectSimopsResultFrame("memory.simops.results.v1", 0, l.nextOffset(), raw)
	if err != nil {
		return err
	}
	_, err = l.Store.SaveResultProjection("", projection)
	return err
}

func (l *MemoryWorkbenchEventLog) PublishTwinState(ctx context.Context, publication TwinStatePublication) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if l.Store == nil {
		return nil
	}
	raw, _ := json.Marshal(publication.State)
	lineageRaw, _ := json.Marshal(publication.Lineage)
	projection, err := ProjectTwinState("memory.digital-twin.state.v1", 0, l.nextOffset(), raw,
		SimopsBrokerHeader{Key: workbenchTwinLineageHeader, Value: lineageRaw},
		SimopsBrokerHeader{Key: workbenchTwinPublicationIDHeader, Value: []byte(publication.PublicationID)},
	)
	if err != nil {
		return err
	}
	_, err = l.Store.SaveTwinStateProjection("", projection)
	return err
}

type RedpandaWorkbenchEventLog struct {
	ScadaTopic     string
	ResultsTopic   string
	TwinStateTopic string
	scadaWriter    simopsBrokerWriter
	resultWriter   simopsBrokerWriter
	twinWriter     simopsBrokerWriter
}

func (l *RedpandaWorkbenchEventLog) PublishScada(ctx context.Context, frame ScadaTelemetryFrame) error {
	key := frame.SourceID + "|" + frame.TagID
	return publishWorkbenchMessage(ctx, l.scadaWriter, key, frame)
}

func (l *RedpandaWorkbenchEventLog) PublishResult(ctx context.Context, frame SimopsResultFrame) error {
	key := frame.RunID + "|" + frame.WorkerID
	return publishWorkbenchMessage(ctx, l.resultWriter, key, frame)
}

func (l *RedpandaWorkbenchEventLog) PublishTwinState(ctx context.Context, publication TwinStatePublication) error {
	raw, err := json.Marshal(publication.State)
	if err != nil {
		return err
	}
	lineageRaw, err := json.Marshal(publication.Lineage)
	if err != nil {
		return err
	}
	return publishWorkbenchBrokerMessage(ctx, l.twinWriter, SimopsBrokerMessage{
		Key:   []byte(strings.TrimSpace(publication.State.TwinID)),
		Value: raw,
		Headers: []SimopsBrokerHeader{
			{Key: workbenchTwinLineageHeader, Value: lineageRaw},
			{Key: workbenchTwinPublicationIDHeader, Value: []byte(publication.PublicationID)},
		},
	})
}

func publishWorkbenchMessage(ctx context.Context, writer simopsBrokerWriter, key string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return publishWorkbenchBrokerMessage(ctx, writer, SimopsBrokerMessage{Key: []byte(key), Value: raw})
}

func publishWorkbenchBrokerMessage(ctx context.Context, writer simopsBrokerWriter, message SimopsBrokerMessage) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if writer == nil {
		return fmt.Errorf("workbench redpanda event log requires writer")
	}
	return writer.WriteMessages(ctx, message)
}

func ProjectScadaFrame(topic string, partition int, offset int64, raw json.RawMessage) (ScadaProjection, error) {
	var frame ScadaTelemetryFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return ScadaProjection{}, fmt.Errorf("decode scada frame: %w", err)
	}
	observedAt := frame.ObservedAt.UTC()
	if observedAt.IsZero() {
		return ScadaProjection{}, fmt.Errorf("scada observedAt is required")
	}
	sampledAt := frame.SampledAt.UTC()
	if sampledAt.IsZero() {
		sampledAt = observedAt
	}
	return ScadaProjection{
		ObservedAt:        observedAt,
		SampledAt:         sampledAt,
		Frame:             frame,
		Raw:               append(json.RawMessage(nil), raw...),
		RedpandaTopic:     normalizeTopic(topic),
		RedpandaPartition: partition,
		RedpandaOffset:    offset,
	}, nil
}

func ProjectSimopsResultFrame(topic string, partition int, offset int64, raw json.RawMessage) (SimopsResultProjection, error) {
	var frame SimopsResultFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return SimopsResultProjection{}, fmt.Errorf("decode simops result frame: %w", err)
	}
	producedAt, err := parseSimopsFrameTime(frame.ProducedAt)
	if err != nil {
		return SimopsResultProjection{}, fmt.Errorf("decode producedAt: %w", err)
	}
	receivedAt := producedAt
	if strings.TrimSpace(frame.ReceivedAt) != "" {
		receivedAt, err = parseSimopsFrameTime(frame.ReceivedAt)
		if err != nil {
			return SimopsResultProjection{}, fmt.Errorf("decode receivedAt: %w", err)
		}
	}
	return SimopsResultProjection{
		ProducedAt:        producedAt,
		ReceivedAt:        receivedAt,
		Frame:             frame,
		Raw:               append(json.RawMessage(nil), raw...),
		RedpandaTopic:     normalizeTopic(topic),
		RedpandaPartition: partition,
		RedpandaOffset:    offset,
	}, nil
}

func ProjectTwinState(topic string, partition int, offset int64, raw json.RawMessage, headers ...SimopsBrokerHeader) (TwinStateProjection, error) {
	var state DigitalTwinState
	if err := json.Unmarshal(raw, &state); err != nil {
		return TwinStateProjection{}, fmt.Errorf("decode twin state: %w", err)
	}
	lineage := []DigitalTwinValueLineage(nil)
	lineagePresent := false
	publicationID := ""
	for _, header := range headers {
		switch header.Key {
		case workbenchTwinLineageHeader:
			lineagePresent = true
			if err := json.Unmarshal(header.Value, &lineage); err != nil {
				return TwinStateProjection{}, fmt.Errorf("decode Twin lineage header: %w", err)
			}
		case workbenchTwinPublicationIDHeader:
			publicationID = strings.TrimSpace(string(header.Value))
		}
	}
	if state.AsOf.IsZero() {
		state.AsOf = timeNowUTC()
	}
	return TwinStateProjection{
		AsOf:              state.AsOf.UTC(),
		State:             state,
		Lineage:           lineage,
		LineagePresent:    lineagePresent,
		PublicationID:     publicationID,
		PublicationSource: WorkbenchProjectionPosition{Topic: normalizeTopic(topic), Partition: partition, Offset: offset},
		Raw:               append(json.RawMessage(nil), raw...),
		RedpandaTopic:     normalizeTopic(topic),
		RedpandaPartition: partition,
		RedpandaOffset:    offset,
	}, nil
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
