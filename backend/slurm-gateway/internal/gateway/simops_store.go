package gateway

import (
	"errors"
	"sync"
	"time"
)

var ErrSimopsRunNotFound = errors.New("simops run not found")
var ErrSimopsArtifactNotFound = errors.New("simops artifact not found")
var ErrSimopsConflict = errors.New("simops conflict")
var ErrDeliveryAttemptNotFound = errors.New("delivery attempt not found")
var ErrDeliveryAttemptNotFound = errors.New("delivery attempt not found")

type SimopsStore interface {
	DeliveryAttemptStore
	CreateRun(record SimopsRunRecord, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) (SimopsRunRecord, bool, error)
	SaveLaunch(runID string, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) error
	GetRunByIdempotency(identity string, key string) (SimopsRunRecord, error)
	GetRun(runID string) (SimopsRunRecord, error)
	ListWorkers(runID string) ([]SimopsWorkerRecord, error)
	ListCommands(runID string) ([]SimopsSpoolCommand, error)
	ListArtifacts(runID string) ([]SimopsArtifactRecord, error)
	UpdateRunLifecycle(runID string, lifecycle SimopsLifecycle) (SimopsRunRecord, error)
	UpdateRunLifecycleWithPublicationIntent(runID string, lifecycle SimopsLifecycle, event SimopsEvent) (SimopsRunRecord, SimopsPublicationIntent, error)
	UpdateWorkerFrames(runID string, workerID string, lifecycle SimopsLifecycle, framesDelta int) error
	UpdateWorkerObservedLifecycle(observation ObservedWorkerLifecycle) error
	SaveArtifact(record SimopsArtifactRecord) error
	SaveEvent(event SimopsEvent) error
	ListEvents(runID string) ([]SimopsEvent, error)
	ListPublicationIntents(runID string) ([]SimopsPublicationIntent, error)
	MarkPublicationIntentPublished(intentID string, event SimopsEvent) error
	MarkPublicationIntentFailed(intentID string, err error) error
	UpdateArtifactStatus(runID string, artifactID string, status string) error
	CreateDeliveryAttempt(DeliveryAttemptRequest) (DeliveryAttempt, bool, error)
	GetDeliveryAttempt(string) (DeliveryAttempt, error)
	PrepareDeliveryAttempt(string, string) error
	ResolveDeliveryAttempt(string, VerifiedDeliveryEvidence) error
	MarkDeliveryAttemptUnknown(string, string) error
	ListDeliveryAttempts(string) ([]DeliveryAttempt, error)
	ActiveRunCount() int
}

const (
	SimopsPublicationPending   = "pending"
	SimopsPublicationPublished = "published"
	SimopsPublicationFailed    = "failed"
)

type SimopsPublicationIntent struct {
	IntentID    string
	RunID       string
	Event       SimopsEvent
	State       string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
}

type InMemorySimopsStore struct {
	mu               sync.RWMutex
	runs             map[string]SimopsRunRecord
	idempotency      map[string]string
	workersByRun     map[string]map[string]SimopsWorkerRecord
	commandsByRun    map[string][]SimopsSpoolCommand
	artifactsByRun   map[string][]SimopsArtifactRecord
	eventsByRun      map[string][]SimopsEvent
	intentsByRun     map[string]map[string]SimopsPublicationIntent
	deliveryAttempts map[string]DeliveryAttempt
}

func NewInMemorySimopsStore() *InMemorySimopsStore {
	return &InMemorySimopsStore{
		runs:             make(map[string]SimopsRunRecord),
		idempotency:      make(map[string]string),
		workersByRun:     make(map[string]map[string]SimopsWorkerRecord),
		commandsByRun:    make(map[string][]SimopsSpoolCommand),
		artifactsByRun:   make(map[string][]SimopsArtifactRecord),
		eventsByRun:      make(map[string][]SimopsEvent),
		intentsByRun:     make(map[string]map[string]SimopsPublicationIntent),
		deliveryAttempts: make(map[string]DeliveryAttempt),
	}
}

func (s *InMemorySimopsStore) CreateDeliveryAttempt(request DeliveryAttemptRequest) (DeliveryAttempt, bool, error) {
	id, err := deliveryAttemptID(request)
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[request.RunID]; !ok {
		return DeliveryAttempt{}, false, ErrSimopsRunNotFound
	}
	if existing, ok := s.deliveryAttempts[id]; ok {
		return cloneDeliveryAttempt(existing), false, nil
	}
	now := time.Now().UTC()
	attempt := DeliveryAttempt{AttemptID: id, RunID: request.RunID, Target: request.Target, Coordinates: cloneDeliveryCoordinates(request.Coordinates), State: DeliveryAttemptPending, CreatedAt: now, UpdatedAt: now}
	s.deliveryAttempts[id] = attempt
	return cloneDeliveryAttempt(attempt), true, nil
}

func (s *InMemorySimopsStore) GetDeliveryAttempt(id string) (DeliveryAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	attempt, ok := s.deliveryAttempts[id]
	if !ok {
		return DeliveryAttempt{}, ErrDeliveryAttemptNotFound
	}
	return cloneDeliveryAttempt(attempt), nil
}

func (s *InMemorySimopsStore) PrepareDeliveryAttempt(id string, location string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.deliveryAttempts[id]
	if !ok {
		return ErrDeliveryAttemptNotFound
	}
	attempt.Location = location
	attempt.UpdatedAt = time.Now().UTC()
	s.deliveryAttempts[id] = attempt
	return nil
}

func (s *InMemorySimopsStore) ResolveDeliveryAttempt(id string, evidence VerifiedDeliveryEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.deliveryAttempts[id]
	if !ok {
		return ErrDeliveryAttemptNotFound
	}
	evidence.AttemptID = id
	evidence.Coordinates = cloneDeliveryCoordinates(attempt.Coordinates)
	attempt.Evidence = &evidence
	attempt.State = DeliveryAttemptResolved
	attempt.Reason = ""
	attempt.UpdatedAt = time.Now().UTC()
	s.deliveryAttempts[id] = attempt
	return nil
}

func (s *InMemorySimopsStore) MarkDeliveryAttemptUnknown(id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.deliveryAttempts[id]
	if !ok {
		return ErrDeliveryAttemptNotFound
	}
	attempt.State = DeliveryAttemptUnknown
	attempt.Reason = reason
	attempt.UpdatedAt = time.Now().UTC()
	s.deliveryAttempts[id] = attempt
	return nil
}

func (s *InMemorySimopsStore) ListDeliveryAttempts(runID string) ([]DeliveryAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	var attempts []DeliveryAttempt
	for _, attempt := range s.deliveryAttempts {
		if attempt.RunID == runID {
			attempts = append(attempts, cloneDeliveryAttempt(attempt))
		}
	}
	return attempts, nil
}

func (s *InMemorySimopsStore) CreateRun(record SimopsRunRecord, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) (SimopsRunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.IdempotencyKey != "" {
		if runID, ok := s.idempotency[idempotencyScope(record.SubmittedBy, record.IdempotencyKey)]; ok {
			existing, ok := s.runs[runID]
			if !ok {
				return SimopsRunRecord{}, false, ErrSimopsRunNotFound
			}
			return existing, false, nil
		}
	}

	if _, ok := s.runs[record.RunID]; ok {
		return SimopsRunRecord{}, false, ErrSimopsConflict
	}

	s.runs[record.RunID] = record
	if record.IdempotencyKey != "" {
		s.idempotency[idempotencyScope(record.SubmittedBy, record.IdempotencyKey)] = record.RunID
	}

	workerMap := make(map[string]SimopsWorkerRecord, len(workers))
	for _, worker := range workers {
		if existing, ok := workerMap[worker.WorkerID]; ok && existing.Frames > worker.Frames {
			worker.Frames = existing.Frames
			worker.Lifecycle = existing.Lifecycle
			worker.UpdatedAt = existing.UpdatedAt
		}
		workerMap[worker.WorkerID] = worker
	}
	s.workersByRun[record.RunID] = workerMap
	s.commandsByRun[record.RunID] = append([]SimopsSpoolCommand(nil), commands...)
	return record, true, nil
}

func (s *InMemorySimopsStore) SaveLaunch(runID string, workers []SimopsWorkerRecord, commands []SimopsSpoolCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runs[runID]; !ok {
		return ErrSimopsRunNotFound
	}

	workerMap := s.workersByRun[runID]
	if workerMap == nil {
		workerMap = make(map[string]SimopsWorkerRecord, len(workers))
	}
	for _, worker := range workers {
		workerMap[worker.WorkerID] = worker
	}
	s.workersByRun[runID] = workerMap
	existingCommands := s.commandsByRun[runID]
	commandIndexes := make(map[string]int, len(existingCommands))
	for index, command := range existingCommands {
		commandIndexes[command.CommandID] = index
	}
	for _, command := range commands {
		if index, ok := commandIndexes[command.CommandID]; ok {
			existingCommands[index] = command
			continue
		}
		commandIndexes[command.CommandID] = len(existingCommands)
		existingCommands = append(existingCommands, command)
	}
	s.commandsByRun[runID] = existingCommands
	return nil
}

func (s *InMemorySimopsStore) GetRunByIdempotency(identity string, key string) (SimopsRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runID, ok := s.idempotency[idempotencyScope(identity, key)]
	if !ok {
		return SimopsRunRecord{}, ErrSimopsRunNotFound
	}
	record, ok := s.runs[runID]
	if !ok {
		return SimopsRunRecord{}, ErrSimopsRunNotFound
	}
	return record, nil
}

func (s *InMemorySimopsStore) GetRun(runID string) (SimopsRunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.runs[runID]
	if !ok {
		return SimopsRunRecord{}, ErrSimopsRunNotFound
	}
	return record, nil
}

func (s *InMemorySimopsStore) ListWorkers(runID string) ([]SimopsWorkerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	workerMap := s.workersByRun[runID]
	workers := make([]SimopsWorkerRecord, 0, len(workerMap))
	for _, worker := range workerMap {
		workers = append(workers, worker)
	}
	return workers, nil
}

func (s *InMemorySimopsStore) ListCommands(runID string) ([]SimopsSpoolCommand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	return append([]SimopsSpoolCommand(nil), s.commandsByRun[runID]...), nil
}

func (s *InMemorySimopsStore) ListArtifacts(runID string) ([]SimopsArtifactRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	return append([]SimopsArtifactRecord(nil), s.artifactsByRun[runID]...), nil
}

func (s *InMemorySimopsStore) UpdateRunLifecycle(runID string, lifecycle SimopsLifecycle) (SimopsRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.runs[runID]
	if !ok {
		return SimopsRunRecord{}, ErrSimopsRunNotFound
	}
	record.Lifecycle = lifecycle
	record.UpdatedAt = time.Now().UTC()
	s.runs[runID] = record
	return record, nil
}

func (s *InMemorySimopsStore) UpdateRunLifecycleWithPublicationIntent(runID string, lifecycle SimopsLifecycle, event SimopsEvent) (SimopsRunRecord, SimopsPublicationIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.runs[runID]
	if !ok {
		return SimopsRunRecord{}, SimopsPublicationIntent{}, ErrSimopsRunNotFound
	}
	now := time.Now().UTC()
	record.Lifecycle = lifecycle
	record.UpdatedAt = now
	s.runs[runID] = record
	if event.RunID == "" {
		event.RunID = runID
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	intent := newSimopsPublicationIntent(event, now)
	if s.intentsByRun[runID] == nil {
		s.intentsByRun[runID] = make(map[string]SimopsPublicationIntent)
	}
	if existing, ok := s.intentsByRun[runID][intent.IntentID]; ok {
		return record, existing, nil
	}
	s.intentsByRun[runID][intent.IntentID] = intent
	return record, intent, nil
}

func (s *InMemorySimopsStore) UpdateWorkerFrames(runID string, workerID string, lifecycle SimopsLifecycle, framesDelta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[runID]; !ok {
		return ErrSimopsRunNotFound
	}
	workerMap := s.workersByRun[runID]
	worker, ok := workerMap[workerID]
	if !ok {
		return ErrSimopsRunNotFound
	}
	worker.Lifecycle = lifecycle
	worker.Frames += framesDelta
	worker.UpdatedAt = time.Now().UTC()
	workerMap[workerID] = worker
	return nil
}

func (s *InMemorySimopsStore) UpdateWorkerObservedLifecycle(observation ObservedWorkerLifecycle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[observation.RunID]; !ok {
		return ErrSimopsRunNotFound
	}
	workerMap := s.workersByRun[observation.RunID]
	worker, ok := workerMap[observation.WorkerID]
	if !ok {
		return ErrSimopsRunNotFound
	}
	applyObservedWorkerLifecycle(&worker, observation, time.Now)
	workerMap[observation.WorkerID] = worker
	return nil
}

func (s *InMemorySimopsStore) SaveArtifact(record SimopsArtifactRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[record.RunID]; !ok {
		return ErrSimopsRunNotFound
	}
	s.artifactsByRun[record.RunID] = append(s.artifactsByRun[record.RunID], record)
	return nil
}

func (s *InMemorySimopsStore) SaveEvent(event SimopsEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[event.RunID]; !ok {
		return ErrSimopsRunNotFound
	}
	s.eventsByRun[event.RunID] = append(s.eventsByRun[event.RunID], event)
	return nil
}

func (s *InMemorySimopsStore) UpdateArtifactStatus(runID string, artifactID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[runID]; !ok {
		return ErrSimopsRunNotFound
	}
	artifacts, ok := s.artifactsByRun[runID]
	if !ok {
		return ErrSimopsArtifactNotFound
	}
	for i, artifact := range artifacts {
		if artifact.ArtifactID == artifactID {
			artifact.Status = status
			artifacts[i] = artifact
			s.artifactsByRun[runID] = artifacts
			return nil
		}
	}
	return ErrSimopsArtifactNotFound
}

func (s *InMemorySimopsStore) ListEvents(runID string) ([]SimopsEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	return append([]SimopsEvent(nil), s.eventsByRun[runID]...), nil
}

func (s *InMemorySimopsStore) ListPublicationIntents(runID string) ([]SimopsPublicationIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.runs[runID]; !ok {
		return nil, ErrSimopsRunNotFound
	}
	intents := make([]SimopsPublicationIntent, 0, len(s.intentsByRun[runID]))
	for _, intent := range s.intentsByRun[runID] {
		intents = append(intents, intent)
	}
	return intents, nil
}

func (s *InMemorySimopsStore) MarkPublicationIntentPublished(intentID string, event SimopsEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for runID, intents := range s.intentsByRun {
		intent, ok := intents[intentID]
		if !ok {
			continue
		}
		now := time.Now().UTC()
		intent.State = SimopsPublicationPublished
		intent.LastError = ""
		intent.UpdatedAt = now
		intent.PublishedAt = &now
		intents[intentID] = intent
		if !eventAlreadySaved(s.eventsByRun[runID], event) {
			s.eventsByRun[runID] = append(s.eventsByRun[runID], event)
		}
		return nil
	}
	return ErrSimopsRunNotFound
}

func (s *InMemorySimopsStore) MarkPublicationIntentFailed(intentID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, intents := range s.intentsByRun {
		intent, ok := intents[intentID]
		if !ok {
			continue
		}
		intent.State = SimopsPublicationFailed
		intent.Attempts++
		if err != nil {
			intent.LastError = err.Error()
		}
		intent.UpdatedAt = time.Now().UTC()
		intents[intentID] = intent
		return nil
	}
	return ErrSimopsRunNotFound
}

func (s *InMemorySimopsStore) ActiveRunCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, run := range s.runs {
		switch run.Lifecycle {
		case SimopsCreated, SimopsStarting, SimopsStreaming, SimopsDegraded:
			count++
		}
	}
	return count
}

func newSimopsPublicationIntent(event SimopsEvent, now time.Time) SimopsPublicationIntent {
	return SimopsPublicationIntent{
		IntentID:  simopsPublicationIntentID(event),
		RunID:     event.RunID,
		Event:     event,
		State:     SimopsPublicationPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func simopsPublicationIntentID(event SimopsEvent) string {
	return event.RunID + "|" + event.EventType + "|" + string(event.Lifecycle)
}

func eventAlreadySaved(events []SimopsEvent, event SimopsEvent) bool {
	for _, existing := range events {
		if simopsPublicationIntentID(existing) == simopsPublicationIntentID(event) && existing.WorkerID == event.WorkerID {
			return true
		}
	}
	return false
}

func idempotencyScope(identity string, key string) string {
	return identity + "|" + key
}

func applyObservedWorkerLifecycle(worker *SimopsWorkerRecord, observation ObservedWorkerLifecycle, now func() time.Time) {
	observedAt := observation.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = now().UTC()
	}
	worker.ObservedLifecycle = observation.State
	worker.ObservedReason = observation.Reason
	worker.ObservedMessage = observation.Message
	worker.Runtime = observation.Runtime
	worker.RuntimeID = observation.RuntimeID
	worker.ObservedExitCode = observation.ExitCode
	worker.ObservedAt = &observedAt
	worker.UpdatedAt = observedAt
	if len(observation.Labels) > 0 {
		worker.Labels = copyStringMap(observation.Labels)
	}
}

func copyStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
