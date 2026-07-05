package gateway

import (
	"errors"
	"sync"
)

var ErrJobNotFound = errors.New("job not found")

type JobStore interface {
	Save(record JobRecord)
	Get(jobID string) (JobRecord, error)
}

type InMemoryJobStore struct {
	mu   sync.RWMutex
	jobs map[string]JobRecord
}

func NewInMemoryJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{jobs: make(map[string]JobRecord)}
}

func (s *InMemoryJobStore) Save(record JobRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[record.JobID] = record
}

func (s *InMemoryJobStore) Get(jobID string) (JobRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.jobs[jobID]
	if !ok {
		return JobRecord{}, ErrJobNotFound
	}
	return record, nil
}
