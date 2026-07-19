package gateway

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	SimopsEventWorkerTelemetry = "worker.telemetry"
	SimopsEventRunLifecycle    = "run.lifecycle"
)

type SimopsStreamQualityFields struct {
	Quality           string  `json:"quality,omitempty"`
	SourceLagMs       float64 `json:"sourceLagMs,omitempty"`
	CollectorLagMs    float64 `json:"collectorLagMs,omitempty"`
	DroppedFrameCount int64   `json:"droppedFrameCount,omitempty"`
}

type SimopsTelemetryProjection struct {
	ReceivedAt        time.Time
	EmittedAt         time.Time
	RunID             string
	ScenarioID        string
	WorkerID          string
	WorkerKind        SimopsWorkerKind
	Sequence          uint64
	PayloadType       string
	Quality           string
	SourceLagMs       sql.NullFloat64
	CollectorLagMs    sql.NullFloat64
	DroppedFrameCount int64
	Frame             json.RawMessage
	RedpandaTopic     string
	RedpandaPartition int
	RedpandaOffset    int64
}

func ProjectTelemetryEvent(topic string, partition int, offset int64, event SimopsEvent) (SimopsTelemetryProjection, bool, error) {
	if event.EventType != SimopsEventWorkerTelemetry {
		return SimopsTelemetryProjection{}, false, nil
	}
	if len(event.Frame) == 0 || string(event.Frame) == "null" {
		return SimopsTelemetryProjection{}, false, fmt.Errorf("worker telemetry event has no frame")
	}

	var frame SimopsTelemetryFrame
	if err := json.Unmarshal(event.Frame, &frame); err != nil {
		return SimopsTelemetryProjection{}, false, fmt.Errorf("decode telemetry frame: %w", err)
	}
	if strings.TrimSpace(frame.RunID) == "" {
		frame.RunID = event.RunID
	}
	if strings.TrimSpace(frame.WorkerID) == "" {
		frame.WorkerID = event.WorkerID
	}

	emittedAt, err := parseSimopsFrameTime(frame.EmittedAt)
	if err != nil {
		return SimopsTelemetryProjection{}, false, fmt.Errorf("decode emittedAt: %w", err)
	}
	receivedAt := event.OccurredAt.UTC()
	if strings.TrimSpace(frame.ReceivedAt) != "" {
		receivedAt, err = parseSimopsFrameTime(frame.ReceivedAt)
		if err != nil {
			return SimopsTelemetryProjection{}, false, fmt.Errorf("decode receivedAt: %w", err)
		}
	}
	if receivedAt.IsZero() {
		receivedAt = emittedAt
	}

	quality := parseSimopsStreamQuality(frame.StreamQuality)
	raw := append(json.RawMessage(nil), event.Frame...)
	return SimopsTelemetryProjection{
		ReceivedAt:        receivedAt.UTC(),
		EmittedAt:         emittedAt.UTC(),
		RunID:             frame.RunID,
		ScenarioID:        frame.ScenarioID,
		WorkerID:          frame.WorkerID,
		WorkerKind:        frame.WorkerKind,
		Sequence:          frame.Sequence,
		PayloadType:       frame.PayloadType,
		Quality:           quality.Quality,
		SourceLagMs:       nullableFloat64(quality.SourceLagMs, containsQualityField(frame.StreamQuality, "sourceLagMs")),
		CollectorLagMs:    nullableFloat64(quality.CollectorLagMs, containsQualityField(frame.StreamQuality, "collectorLagMs")),
		DroppedFrameCount: quality.DroppedFrameCount,
		Frame:             raw,
		RedpandaTopic:     normalizeTopic(topic),
		RedpandaPartition: partition,
		RedpandaOffset:    offset,
	}, true, nil
}

func parseSimopsFrameTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseSimopsStreamQuality(raw json.RawMessage) SimopsStreamQualityFields {
	if len(raw) == 0 || string(raw) == "null" {
		return SimopsStreamQualityFields{}
	}
	var fields SimopsStreamQualityFields
	_ = json.Unmarshal(raw, &fields)
	return fields
}

func nullableFloat64(value float64, valid bool) sql.NullFloat64 {
	return sql.NullFloat64{Float64: value, Valid: valid}
}

func containsQualityField(raw json.RawMessage, name string) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	_, ok := payload[name]
	return ok
}

type SimopsConsumerMetrics struct {
	mu                        sync.RWMutex
	brokerConnected           bool
	requiredBrokerConnections map[string]struct{}
	brokerConnections         map[string]bool
	lastConsumedOffset        int64
	framesWritten             uint64
	writeFailures             uint64
	batchFlushes              uint64
	subscriberCount           uint64
	lastError                 string
}

func NewSimopsConsumerMetrics() *SimopsConsumerMetrics {
	return &SimopsConsumerMetrics{
		requiredBrokerConnections: make(map[string]struct{}),
		brokerConnections:         make(map[string]bool),
		lastConsumedOffset:        -1,
	}
}

func (m *SimopsConsumerMetrics) MarkBrokerConnected(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.brokerConnected = connected
	m.markSoleRequiredBrokerConnection(connected)
}

func (m *SimopsConsumerMetrics) markSoleRequiredBrokerConnection(connected bool) {
	if len(m.requiredBrokerConnections) != 1 {
		return
	}
	if m.brokerConnections == nil {
		m.brokerConnections = make(map[string]bool)
	}
	for name := range m.requiredBrokerConnections {
		m.brokerConnections[name] = connected
	}
}

func (m *SimopsConsumerMetrics) RequireBrokerConnections(names ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.requiredBrokerConnections == nil {
		m.requiredBrokerConnections = make(map[string]struct{})
	}
	for _, name := range names {
		if name != "" {
			m.requiredBrokerConnections[name] = struct{}{}
		}
	}
}

func (m *SimopsConsumerMetrics) MarkBrokerConnection(name string, connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if name != "" {
		if m.brokerConnections == nil {
			m.brokerConnections = make(map[string]bool)
		}
		m.brokerConnections[name] = connected
	}
}

func (m *SimopsConsumerMetrics) MarkConsumed(offset int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastConsumedOffset = offset
	m.brokerConnected = true
	m.markSoleRequiredBrokerConnection(true)
}

func (m *SimopsConsumerMetrics) IncFramesWritten(delta uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.framesWritten += delta
}

func (m *SimopsConsumerMetrics) IncWriteFailures() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeFailures++
}

func (m *SimopsConsumerMetrics) IncBatchFlushes() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchFlushes++
}

func (m *SimopsConsumerMetrics) SetSubscriberCount(count uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriberCount = count
}

func (m *SimopsConsumerMetrics) SetLastError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		m.lastError = ""
		return
	}
	m.lastError = err.Error()
}

func (m *SimopsConsumerMetrics) Snapshot() SimopsConsumerMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	brokerConnected := m.brokerConnected
	connections := make(map[string]bool, len(m.requiredBrokerConnections))
	if len(m.requiredBrokerConnections) > 0 {
		brokerConnected = true
		for name := range m.requiredBrokerConnections {
			connected := m.brokerConnections[name]
			connections[name] = connected
			brokerConnected = brokerConnected && connected
		}
	}
	return SimopsConsumerMetricsSnapshot{
		BrokerConnected:    brokerConnected,
		BrokerConnections:  connections,
		LastConsumedOffset: m.lastConsumedOffset,
		FramesWritten:      m.framesWritten,
		WriteFailures:      m.writeFailures,
		BatchFlushes:       m.batchFlushes,
		SubscriberCount:    m.subscriberCount,
		LastError:          m.lastError,
	}
}

type SimopsConsumerMetricsSnapshot struct {
	BrokerConnected    bool            `json:"broker_connected"`
	BrokerConnections  map[string]bool `json:"broker_connections,omitempty"`
	LastConsumedOffset int64           `json:"last_consumed_offset"`
	FramesWritten      uint64          `json:"frames_written"`
	WriteFailures      uint64          `json:"write_failures"`
	BatchFlushes       uint64          `json:"batch_flush_count"`
	SubscriberCount    uint64          `json:"subscriber_count"`
	LastError          string          `json:"last_error,omitempty"`
}

func (s SimopsConsumerMetricsSnapshot) Ready() bool {
	return s.BrokerConnected && s.WriteFailures == 0 && s.LastError == ""
}

func (s SimopsConsumerMetricsSnapshot) Prometheus(prefix string) string {
	broker := 0
	if s.BrokerConnected {
		broker = 1
	}
	return fmt.Sprintf(`# HELP %[1]s_broker_connected Redpanda broker connectivity state.
# TYPE %[1]s_broker_connected gauge
%[1]s_broker_connected %d
# HELP %[1]s_last_consumed_offset Last consumed Redpanda offset.
# TYPE %[1]s_last_consumed_offset gauge
%[1]s_last_consumed_offset %d
# HELP %[1]s_frames_written Frames written by this consumer.
# TYPE %[1]s_frames_written counter
%[1]s_frames_written %d
# HELP %[1]s_write_failures Write failures seen by this consumer.
# TYPE %[1]s_write_failures counter
%[1]s_write_failures %d
# HELP %[1]s_batch_flush_count Batch flushes completed by this consumer.
# TYPE %[1]s_batch_flush_count counter
%[1]s_batch_flush_count %d
# HELP %[1]s_subscriber_count Active MoQ subscriber count.
# TYPE %[1]s_subscriber_count gauge
%[1]s_subscriber_count %d
`, prefix, broker, s.LastConsumedOffset, s.FramesWritten, s.WriteFailures, s.BatchFlushes, s.SubscriberCount)
}
