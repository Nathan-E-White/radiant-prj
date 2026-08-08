package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type DeliveryAttemptState string

const (
	DeliveryAttemptPending  DeliveryAttemptState = "pending"
	DeliveryAttemptUnknown  DeliveryAttemptState = "unknown"
	DeliveryAttemptResolved DeliveryAttemptState = "resolved"
)

type DeliveryAssurance string

const (
	DeliveryAssuranceManifestWritten             DeliveryAssurance = "manifest-written"
	DeliveryAssuranceExternalCommandAcknowledged DeliveryAssurance = "external-command-acknowledged"
	DeliveryAssuranceIcebergReadbackVerified     DeliveryAssurance = "iceberg-readback-verified"
	DeliveryAssuranceDisabled                    DeliveryAssurance = "delivery-disabled"
)

type DeliveryReconciliation string

const DeliveryReconciliationVerified DeliveryReconciliation = "verified"

type DeliveryCoordinate struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
}

type DeliveryAttemptRequest struct {
	RunID       string               `json:"run_id"`
	Target      string               `json:"target"`
	Coordinates []DeliveryCoordinate `json:"coordinates"`
}

type VerifiedDeliveryEvidence struct {
	AttemptID      string                 `json:"attempt_id"`
	Assurance      DeliveryAssurance      `json:"assurance"`
	Coordinates    []DeliveryCoordinate   `json:"coordinates"`
	Reconciliation DeliveryReconciliation `json:"reconciliation"`
	ObservedAt     time.Time              `json:"observed_at"`
}

type DeliveryAttempt struct {
	AttemptID   string                    `json:"attempt_id"`
	RunID       string                    `json:"run_id"`
	Target      string                    `json:"target"`
	Coordinates []DeliveryCoordinate      `json:"coordinates"`
	State       DeliveryAttemptState      `json:"state"`
	Reason      string                    `json:"reason,omitempty"`
	Evidence    *VerifiedDeliveryEvidence `json:"evidence,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}

type DeliveryAttemptStore interface {
	CreateDeliveryAttempt(DeliveryAttemptRequest) (DeliveryAttempt, bool, error)
	GetDeliveryAttempt(string) (DeliveryAttempt, error)
	ResolveDeliveryAttempt(string, VerifiedDeliveryEvidence) error
	MarkDeliveryAttemptUnknown(string, string) error
	ListDeliveryAttempts(string) ([]DeliveryAttempt, error)
}

func deliveryAttemptID(request DeliveryAttemptRequest) (string, error) {
	if strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.Target) == "" || len(request.Coordinates) == 0 {
		return "", fmt.Errorf("delivery attempt requires run, target, and coordinates")
	}
	b := strings.Builder{}
	b.WriteString(request.RunID)
	b.WriteByte('|')
	b.WriteString(request.Target)
	for _, coordinate := range request.Coordinates {
		if strings.TrimSpace(coordinate.Topic) == "" || coordinate.Partition < 0 || coordinate.Offset < 0 {
			return "", fmt.Errorf("delivery attempt has invalid coordinate")
		}
		fmt.Fprintf(&b, "|%s/%d/%d", coordinate.Topic, coordinate.Partition, coordinate.Offset)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "delivery-" + hex.EncodeToString(sum[:16]), nil
}

func cloneDeliveryCoordinates(coordinates []DeliveryCoordinate) []DeliveryCoordinate {
	return append([]DeliveryCoordinate(nil), coordinates...)
}

func cloneDeliveryAttempt(attempt DeliveryAttempt) DeliveryAttempt {
	attempt.Coordinates = cloneDeliveryCoordinates(attempt.Coordinates)
	if attempt.Evidence != nil {
		evidence := *attempt.Evidence
		evidence.Coordinates = cloneDeliveryCoordinates(evidence.Coordinates)
		attempt.Evidence = &evidence
	}
	return attempt
}
