package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type SimopsMoQTrackMessage struct {
	Track      string          `json:"track"`
	RunID      string          `json:"run_id"`
	WorkerID   string          `json:"worker_id,omitempty"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
	Offset     int64           `json:"offset,omitempty"`
}

type SimopsMoQWireMessage struct {
	Protocol   string          `json:"protocol"`
	Namespace  string          `json:"namespace"`
	Track      string          `json:"track"`
	RunID      string          `json:"run_id"`
	WorkerID   string          `json:"worker_id,omitempty"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
	Offset     int64           `json:"offset,omitempty"`
}

func NewSimopsMoQWireMessage(message SimopsMoQTrackMessage) SimopsMoQWireMessage {
	return SimopsMoQWireMessage{
		Protocol:   "moq-webtransport",
		Namespace:  "radiant/simops/" + message.RunID,
		Track:      message.Track,
		RunID:      message.RunID,
		WorkerID:   message.WorkerID,
		EventType:  message.EventType,
		Payload:    cloneRawMessage(message.Payload),
		OccurredAt: message.OccurredAt,
		Offset:     message.Offset,
	}
}

type SimopsMoQTrackRouter struct {
	mu     sync.RWMutex
	tracks map[string]SimopsMoQTrackMessage
}

func NewSimopsMoQTrackRouter() *SimopsMoQTrackRouter {
	return &SimopsMoQTrackRouter{tracks: make(map[string]SimopsMoQTrackMessage)}
}

func (r *SimopsMoQTrackRouter) ApplyEvent(event SimopsEvent, offset int64) ([]SimopsMoQTrackMessage, error) {
	if r == nil {
		return nil, fmt.Errorf("moq track router is nil")
	}
	messages := []SimopsMoQTrackMessage{}
	switch event.EventType {
	case SimopsEventRunLifecycle:
		messages = append(messages, SimopsMoQTrackMessage{
			Track:      "lifecycle",
			RunID:      event.RunID,
			EventType:  event.EventType,
			Payload:    lifecyclePayload(event),
			OccurredAt: event.OccurredAt,
			Offset:     offset,
		})
	case SimopsEventWorkerTelemetry:
		if strings.TrimSpace(event.WorkerID) == "" {
			return nil, fmt.Errorf("worker telemetry event requires worker_id for MoQ track routing")
		}
		messages = append(messages, SimopsMoQTrackMessage{
			Track:      "workers/" + event.WorkerID + "/telemetry",
			RunID:      event.RunID,
			WorkerID:   event.WorkerID,
			EventType:  event.EventType,
			Payload:    cloneRawMessage(event.Frame),
			OccurredAt: event.OccurredAt,
			Offset:     offset,
		})
		quality, ok := qualityPayload(event.Frame)
		if ok {
			messages = append(messages, SimopsMoQTrackMessage{
				Track:      "workers/" + event.WorkerID + "/quality",
				RunID:      event.RunID,
				WorkerID:   event.WorkerID,
				EventType:  event.EventType,
				Payload:    quality,
				OccurredAt: event.OccurredAt,
				Offset:     offset,
			})
		}
	case SimopsArtifactIntentEventType:
		messages = append(messages, SimopsMoQTrackMessage{
			Track:      "artifacts",
			RunID:      event.RunID,
			EventType:  event.EventType,
			Payload:    cloneRawMessage(event.Frame),
			OccurredAt: event.OccurredAt,
			Offset:     offset,
		})
	default:
		return nil, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, message := range messages {
		r.tracks[message.Track] = message
	}
	return messages, nil
}

func (r *SimopsMoQTrackRouter) Snapshot() []SimopsMoQTrackMessage {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tracks := make([]SimopsMoQTrackMessage, 0, len(r.tracks))
	for _, message := range r.tracks {
		tracks = append(tracks, message)
	}
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Track < tracks[j].Track
	})
	return tracks
}

type SimopsMoQTrackSink interface {
	PublishMoQTracks([]SimopsMoQTrackMessage)
}

type SimopsMoQTrackHub struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]chan SimopsMoQTrackMessage
}

func NewSimopsMoQTrackHub() *SimopsMoQTrackHub {
	return &SimopsMoQTrackHub{subscribers: make(map[uint64]chan SimopsMoQTrackMessage)}
}

func (h *SimopsMoQTrackHub) Subscribe(buffer int) (<-chan SimopsMoQTrackMessage, func(), uint64) {
	if h == nil {
		closed := make(chan SimopsMoQTrackMessage)
		close(closed)
		return closed, func() {}, 0
	}
	if buffer <= 0 {
		buffer = 1
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	ch := make(chan SimopsMoQTrackMessage, buffer)
	h.subscribers[id] = ch
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if subscribed, ok := h.subscribers[id]; ok {
			delete(h.subscribers, id)
			close(subscribed)
		}
	}
	return ch, cancel, id
}

func (h *SimopsMoQTrackHub) PublishMoQTracks(messages []SimopsMoQTrackMessage) {
	if h == nil || len(messages) == 0 {
		return
	}
	h.mu.RLock()
	subscribers := make([]chan SimopsMoQTrackMessage, 0, len(h.subscribers))
	for _, ch := range h.subscribers {
		subscribers = append(subscribers, ch)
	}
	h.mu.RUnlock()

	for _, message := range messages {
		for _, ch := range subscribers {
			select {
			case ch <- message:
			default:
				select {
				case <-ch:
				default:
				}
				select {
				case ch <- message:
				default:
				}
			}
		}
	}
}

func (h *SimopsMoQTrackHub) SubscriberCount() uint64 {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return uint64(len(h.subscribers))
}

func RunMoQTrackConsumer(ctx context.Context, cfg SimopsConfig, reader SimopsKafkaReader, router *SimopsMoQTrackRouter, metrics *SimopsConsumerMetrics, sinks ...SimopsMoQTrackSink) error {
	if metrics == nil {
		metrics = NewSimopsConsumerMetrics()
	}
	if router == nil {
		return fmt.Errorf("moq track consumer requires router")
	}
	if reader == nil {
		created, err := NewSimopsKafkaReader(cfg, cfg.MoQConsumerGroup)
		if err != nil {
			return err
		}
		reader = created
	}
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			metrics.MarkBrokerConnected(false)
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.MarkConsumed(msg.Offset)
		event, err := decodeSimopsKafkaEvent(msg)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
				return commitErr
			}
			continue
		}
		event.RedpandaTopic = msg.Topic
		event.RedpandaPartition = msg.Partition
		event.RedpandaOffset = msg.Offset
		messages, err := router.ApplyEvent(event, msg.Offset)
		if err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		if len(messages) > 0 {
			metrics.IncFramesWritten(uint64(len(messages)))
			for _, sink := range sinks {
				sink.PublishMoQTracks(messages)
			}
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			metrics.IncWriteFailures()
			metrics.SetLastError(err)
			return err
		}
		metrics.SetLastError(nil)
	}
}

func lifecyclePayload(event SimopsEvent) json.RawMessage {
	payload, _ := json.Marshal(map[string]string{
		"run_id":    event.RunID,
		"lifecycle": string(event.Lifecycle),
	})
	return payload
}

func qualityPayload(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var frame SimopsTelemetryFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, false
	}
	if len(frame.StreamQuality) == 0 || string(frame.StreamQuality) == "null" {
		return nil, false
	}
	return cloneRawMessage(frame.StreamQuality), true
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
