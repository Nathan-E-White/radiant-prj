package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type TwinStatePublication struct {
	PublicationID  string                      `json:"publicationId"`
	Source         WorkbenchProjectionPosition `json:"source"`
	TwinStateTopic string                      `json:"twinStateTopic"`
	State          DigitalTwinState            `json:"state"`
	Lineage        []DigitalTwinValueLineage   `json:"lineage"`
	Acknowledged   bool                        `json:"acknowledged,omitempty"`
}

type TwinStatePublicationRecovery string

const (
	TwinStatePublicationComplete TwinStatePublicationRecovery = "complete"
	TwinStatePublicationRetry    TwinStatePublicationRecovery = "retry_same_publication"
)

type TwinStatePublicationStage string

const (
	TwinStatePublicationPersistence   TwinStatePublicationStage = "persistence"
	TwinStatePublicationEventDelivery TwinStatePublicationStage = "event_delivery"
)

type TwinStatePublicationOutcome struct {
	PublicationID string                       `json:"publicationId"`
	Persisted     bool                         `json:"persisted"`
	Duplicate     bool                         `json:"duplicate"`
	Delivered     bool                         `json:"delivered"`
	Recovery      TwinStatePublicationRecovery `json:"recovery"`
}

type TwinStatePublicationError struct {
	Stage   TwinStatePublicationStage
	Outcome TwinStatePublicationOutcome
	Cause   error
}

func (e *TwinStatePublicationError) Error() string {
	return fmt.Sprintf("Twin State publication %s failed during %s: %v", e.Outcome.PublicationID, e.Stage, e.Cause)
}

func (e *TwinStatePublicationError) Unwrap() error { return e.Cause }

// TwinStatePublisher is the complete Twin publication boundary. Publish first
// commits Twin State, its active Lineage set, and the canonical publication as
// one store transition, then delivers that persisted publication to the event
// adapter. Persistence failure delivers nothing. Delivery failure returns a
// retry outcome; replaying the same publicationId skips persistence and
// redelivers the stored bytes, while downstream semantic deduplication absorbs
// ambiguous duplicate events. Resume checks for that persisted publication by
// source coordinate so a restarted projector can finish delivery before it
// rebuilds or commits the replayed source message.
type TwinStatePublisher interface {
	Publish(context.Context, TwinStatePublication) (TwinStatePublicationOutcome, error)
	Resume(context.Context, WorkbenchProjectionPosition) (TwinStatePublicationOutcome, bool, error)
	Acknowledge(WorkbenchProjectionPosition) error
}

type orderedTwinStatePublisher struct {
	store    WorkbenchStore
	eventLog WorkbenchEventLog
}

func NewTwinStatePublisher(store WorkbenchStore, eventLog WorkbenchEventLog) TwinStatePublisher {
	if store == nil {
		store = NewInMemoryWorkbenchStore()
	}
	if eventLog == nil {
		eventLog = &MemoryWorkbenchEventLog{Store: store}
	}
	return &orderedTwinStatePublisher{store: store, eventLog: eventLog}
}

func NewTwinStatePublication(source WorkbenchProjectionPosition, twinStateTopic string, state DigitalTwinState, lineage []DigitalTwinValueLineage) TwinStatePublication {
	return TwinStatePublication{
		PublicationID:  twinStatePublicationID(source),
		Source:         source,
		TwinStateTopic: strings.TrimSpace(twinStateTopic),
		State:          state,
		Lineage:        lineage,
	}
}

func twinStatePublicationID(source WorkbenchProjectionPosition) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%d", source.Topic, source.Partition, source.Offset)))
	return "twinpub-" + hex.EncodeToString(hash[:16])
}

func (p *orderedTwinStatePublisher) Resume(ctx context.Context, source WorkbenchProjectionPosition) (TwinStatePublicationOutcome, bool, error) {
	publicationID := twinStatePublicationID(source)
	outcome := TwinStatePublicationOutcome{
		PublicationID: publicationID,
		Duplicate:     true,
		Recovery:      TwinStatePublicationRetry,
	}
	publication, err := p.store.GetTwinStatePublication(publicationID)
	if errors.Is(err, ErrWorkbenchNotFound) {
		return TwinStatePublicationOutcome{PublicationID: publicationID, Recovery: TwinStatePublicationRetry}, false, nil
	}
	if err != nil {
		return outcome, false, &TwinStatePublicationError{Stage: TwinStatePublicationPersistence, Outcome: outcome, Cause: err}
	}
	if publication.Acknowledged {
		outcome.Delivered = true
		outcome.Recovery = TwinStatePublicationComplete
		return outcome, true, nil
	}
	if err := p.eventLog.PublishTwinState(ctx, publication); err != nil {
		return outcome, true, &TwinStatePublicationError{Stage: TwinStatePublicationEventDelivery, Outcome: outcome, Cause: err}
	}
	outcome.Delivered = true
	outcome.Recovery = TwinStatePublicationComplete
	return outcome, true, nil
}

func (p *orderedTwinStatePublisher) Acknowledge(source WorkbenchProjectionPosition) error {
	return p.store.AcknowledgeTwinStatePublication(twinStatePublicationID(source))
}

func (p *orderedTwinStatePublisher) Publish(ctx context.Context, publication TwinStatePublication) (TwinStatePublicationOutcome, error) {
	outcome := TwinStatePublicationOutcome{PublicationID: publication.PublicationID, Recovery: TwinStatePublicationRetry}
	if err := validateTwinStatePublication(publication); err != nil {
		return outcome, &TwinStatePublicationError{Stage: TwinStatePublicationPersistence, Outcome: outcome, Cause: err}
	}
	projection, err := ProjectTwinStatePublication(publication.TwinStateTopic, publication.Source.Partition, publication.Source.Offset, publication)
	if err != nil {
		return outcome, &TwinStatePublicationError{Stage: TwinStatePublicationPersistence, Outcome: outcome, Cause: err}
	}
	written, err := p.store.SaveTwinStateProjection("", projection)
	if err != nil {
		return outcome, &TwinStatePublicationError{Stage: TwinStatePublicationPersistence, Outcome: outcome, Cause: err}
	}
	outcome.Persisted = written
	outcome.Duplicate = !written
	canonical, err := p.store.GetTwinStatePublication(publication.PublicationID)
	if err != nil {
		return outcome, &TwinStatePublicationError{Stage: TwinStatePublicationPersistence, Outcome: outcome, Cause: err}
	}
	if canonical.Acknowledged {
		outcome.Delivered = true
		outcome.Recovery = TwinStatePublicationComplete
		return outcome, nil
	}
	if err := p.eventLog.PublishTwinState(ctx, canonical); err != nil {
		return outcome, &TwinStatePublicationError{Stage: TwinStatePublicationEventDelivery, Outcome: outcome, Cause: err}
	}
	outcome.Delivered = true
	outcome.Recovery = TwinStatePublicationComplete
	return outcome, nil
}

func validateTwinStatePublication(publication TwinStatePublication) error {
	if publication.PublicationID != twinStatePublicationID(publication.Source) {
		return errors.New("Twin State publicationId does not match its source coordinate")
	}
	if strings.TrimSpace(publication.TwinStateTopic) == "" {
		return errors.New("Twin State publication requires an output topic")
	}
	if strings.TrimSpace(publication.State.SchemaVersion) == "" || strings.TrimSpace(publication.State.TwinID) == "" {
		return errors.New("Twin State publication requires a versioned Twin State")
	}
	lineageByID := make(map[string]DigitalTwinValueLineage, len(publication.Lineage))
	for _, lineage := range publication.Lineage {
		if _, duplicate := lineageByID[lineage.LineageID]; duplicate {
			return fmt.Errorf("Twin State publication has duplicate lineageId %q", lineage.LineageID)
		}
		lineageByID[lineage.LineageID] = lineage
	}
	for _, entity := range publication.State.Entities {
		for _, value := range entity.Values {
			if value.ValueBasis != WorkbenchValueImputed {
				continue
			}
			lineage, ok := lineageByID[value.LineageID]
			if strings.TrimSpace(value.LineageID) == "" || !ok || lineage.ValueID != value.ValueID || lineage.ValueBasis != WorkbenchValueImputed {
				return fmt.Errorf("Imputed Twin value %q requires matching Imputed lineage", value.ValueID)
			}
		}
	}
	return nil
}

func ProjectTwinStatePublication(topic string, partition int, offset int64, publication TwinStatePublication) (TwinStateProjection, error) {
	if err := validateTwinStatePublication(publication); err != nil {
		return TwinStateProjection{}, err
	}
	return TwinStateProjection{
		AsOf:              publication.State.AsOf.UTC(),
		State:             publication.State,
		Lineage:           publication.Lineage,
		LineagePresent:    true,
		PublicationID:     publication.PublicationID,
		PublicationSource: publication.Source,
		RedpandaTopic:     normalizeTopic(topic),
		RedpandaPartition: partition,
		RedpandaOffset:    offset,
	}, nil
}
