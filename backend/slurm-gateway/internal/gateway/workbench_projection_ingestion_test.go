package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWorkbenchProjectionIngestionPersistsBeforeCommitAndCountsOnlyNewFrames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	steps := []string{}
	reader := &projectionIngestionTestReader{
		messages: []SimopsBrokerMessage{{Topic: "test", Partition: 2, Offset: 10}, {Topic: "test", Partition: 2, Offset: 11}},
		steps:    &steps,
		afterCommit: func(commits int) {
			if commits == 2 {
				cancel()
			}
		},
	}
	metrics := NewSimopsConsumerMetrics()
	persistCalls := 0
	adapter := WorkbenchProjectionIngestionAdapter[int]{
		Stream: WorkbenchProjectionMeasured,
		Project: func(message SimopsBrokerMessage) (int, error) {
			steps = append(steps, "project")
			return int(message.Offset), nil
		},
		Persist: func(projection int) (bool, error) {
			steps = append(steps, "persist")
			persistCalls++
			return persistCalls == 1, nil
		},
	}

	err := RunWorkbenchProjectionIngestion(ctx, reader, metrics, adapter)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected clean cancellation result, got %v", err)
	}
	wantSteps := []string{"fetch", "project", "persist", "commit", "fetch", "project", "persist", "commit", "fetch"}
	if !reflect.DeepEqual(steps, wantSteps) {
		t.Fatalf("persistence must precede each commit: got %v want %v", steps, wantSteps)
	}
	snapshot := metrics.Snapshot()
	if snapshot.LastConsumedOffset != 11 || snapshot.FramesWritten != 1 || snapshot.WriteFailures != 0 || snapshot.LastError != "" {
		t.Fatalf("duplicate must commit without counting a new frame: %#v", snapshot)
	}
}

func TestWorkbenchProjectionIngestionMakesFailureStagesAndMetricsExplicit(t *testing.T) {
	fetchFailure := errors.New("broker fetch failed")
	projectFailure := errors.New("invalid measured frame")
	persistFailure := errors.New("projection persistence failed")
	commitFailure := errors.New("consumer position commit failed")
	tests := []struct {
		name         string
		reader       *projectionIngestionTestReader
		project      func(SimopsBrokerMessage) (int, error)
		persist      func(int) (bool, error)
		wantStage    WorkbenchProjectionIngestionStage
		wantCause    error
		wantCommits  int
		wantFrames   uint64
		wantPosition bool
	}{
		{
			name: "fetch", reader: &projectionIngestionTestReader{fetchErr: fetchFailure},
			project:   func(SimopsBrokerMessage) (int, error) { t.Fatal("fetch failure must not project"); return 0, nil },
			persist:   func(int) (bool, error) { t.Fatal("fetch failure must not persist"); return false, nil },
			wantStage: WorkbenchProjectionIngestionFetch, wantCause: fetchFailure,
		},
		{
			name: "projection", reader: &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Offset: 20}}},
			project:   func(SimopsBrokerMessage) (int, error) { return 0, projectFailure },
			persist:   func(int) (bool, error) { t.Fatal("invalid projection must not persist"); return false, nil },
			wantStage: WorkbenchProjectionIngestionProject, wantCause: projectFailure, wantPosition: true,
		},
		{
			name: "persistence", reader: &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Offset: 21}}},
			project:   func(SimopsBrokerMessage) (int, error) { return 21, nil },
			persist:   func(int) (bool, error) { return false, persistFailure },
			wantStage: WorkbenchProjectionIngestionPersist, wantCause: persistFailure, wantPosition: true,
		},
		{
			name: "commit", reader: &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Offset: 22}}, commitErr: commitFailure},
			project:   func(SimopsBrokerMessage) (int, error) { return 22, nil },
			persist:   func(int) (bool, error) { return true, nil },
			wantStage: WorkbenchProjectionIngestionCommit, wantCause: commitFailure, wantCommits: 1, wantFrames: 1, wantPosition: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics := NewSimopsConsumerMetrics()
			err := RunWorkbenchProjectionIngestion(context.Background(), test.reader, metrics, WorkbenchProjectionIngestionAdapter[int]{
				Stream: WorkbenchProjectionMeasured, Project: test.project, Persist: test.persist,
			})
			var ingestionErr *WorkbenchProjectionIngestionError
			if !errors.As(err, &ingestionErr) || ingestionErr.Stage != test.wantStage {
				t.Fatalf("expected %s error, got %T %v", test.wantStage, err, err)
			}
			if test.wantCause != nil && !errors.Is(err, test.wantCause) {
				t.Fatalf("expected cause %v, got %v", test.wantCause, err)
			}
			if (ingestionErr.Position != nil) != test.wantPosition {
				t.Fatalf("unexpected position evidence: %#v", ingestionErr.Position)
			}
			if test.wantStage == WorkbenchProjectionIngestionFetch && strings.Contains(err.Error(), "[0]@0") {
				t.Fatalf("fetch failure fabricated a broker position: %v", err)
			}
			if len(test.reader.committed) != test.wantCommits {
				t.Fatalf("unexpected commits: %#v", test.reader.committed)
			}
			snapshot := metrics.Snapshot()
			if snapshot.WriteFailures != 1 || snapshot.LastError == "" || snapshot.FramesWritten != test.wantFrames || snapshot.LastConsumedOffset != -1 {
				t.Fatalf("expected explicit failure metrics, got %#v", snapshot)
			}
		})
	}
}

func TestWorkbenchProjectionIngestionCancellationIsNotAWriteFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	metrics := NewSimopsConsumerMetrics()
	reader := &projectionIngestionTestReader{}
	err := RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[int]{
		Stream:  WorkbenchProjectionMeasured,
		Project: func(SimopsBrokerMessage) (int, error) { t.Fatal("cancellation must not project"); return 0, nil },
		Persist: func(int) (bool, error) { t.Fatal("cancellation must not persist"); return false, nil },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if snapshot := metrics.Snapshot(); snapshot.WriteFailures != 0 || snapshot.LastError != "" || len(reader.committed) != 0 {
		t.Fatalf("cancellation must not become a write failure: metrics=%#v commits=%#v", snapshot, reader.committed)
	}
}

func TestWorkbenchProjectionIngestionDoesNotCommitWhenCancelledAfterPersistence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	metrics := NewSimopsConsumerMetrics()
	reader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Topic: "measured", Offset: 23}}}
	persisted := false
	err := RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[int]{
		Stream:  WorkbenchProjectionMeasured,
		Project: func(message SimopsBrokerMessage) (int, error) { return int(message.Offset), nil },
		Persist: func(int) (bool, error) {
			persisted = true
			cancel()
			return true, nil
		},
	})
	if !persisted || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected persistence followed by cancellation, persisted=%v err=%v", persisted, err)
	}
	if snapshot := metrics.Snapshot(); len(reader.committed) != 0 || snapshot.FramesWritten != 1 || snapshot.WriteFailures != 0 {
		t.Fatalf("cancelled persistence must remain uncommitted for duplicate-safe replay: metrics=%#v commits=%#v", snapshot, reader.committed)
	}
}

func TestWorkbenchProjectionIngestionDoesNotClearAnotherStreamsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	metrics := NewSimopsConsumerMetrics()
	metrics.IncWriteFailures()
	metrics.SetLastError(errors.New("twin stream failed"))
	reader := &projectionIngestionTestReader{
		messages:    []SimopsBrokerMessage{{Offset: 24}},
		afterCommit: func(int) { cancel() },
	}
	err := RunWorkbenchProjectionIngestion(ctx, reader, metrics, WorkbenchProjectionIngestionAdapter[int]{
		Stream:  WorkbenchProjectionMeasured,
		Project: func(message SimopsBrokerMessage) (int, error) { return int(message.Offset), nil },
		Persist: func(int) (bool, error) { return true, nil },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation after proof message, got %v", err)
	}
	if snapshot := metrics.Snapshot(); snapshot.LastError != "twin stream failed" || snapshot.WriteFailures != 1 {
		t.Fatalf("healthy stream erased shared terminal error: %#v", snapshot)
	}
}

func TestInMemoryWorkbenchStoreDeduplicatesProjectionCoordinatesAcrossStreams(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scada, _ := ProjectScadaFrame("scada", 1, 40, scadaRaw)
	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-DEDUPE"))
	result, _ := ProjectSimopsResultFrame("results", 1, 41, resultRaw)
	twinState, _ := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-DEDUPE"), time.Now().UTC())
	twinRaw, _ := json.Marshal(twinState)
	twin, _ := ProjectTwinState("twin", 1, 42, twinRaw)

	first, err := store.SaveScadaProjection("scada-consumer", scada)
	duplicate, duplicateErr := store.SaveScadaProjection("scada-consumer", scada)
	if err != nil || duplicateErr != nil || !first || duplicate {
		t.Fatalf("unexpected SCADA dedupe outcomes first=%v duplicate=%v errors=%v/%v", first, duplicate, err, duplicateErr)
	}
	first, err = store.SaveResultProjection("result-consumer", result)
	duplicate, duplicateErr = store.SaveResultProjection("result-consumer", result)
	if err != nil || duplicateErr != nil || !first || duplicate {
		t.Fatalf("unexpected result dedupe outcomes first=%v duplicate=%v errors=%v/%v", first, duplicate, err, duplicateErr)
	}
	first, err = store.SaveTwinStateProjection("twin-consumer", twin)
	duplicate, duplicateErr = store.SaveTwinStateProjection("twin-consumer", twin)
	if err != nil || duplicateErr != nil || !first || duplicate {
		t.Fatalf("unexpected twin dedupe outcomes first=%v duplicate=%v errors=%v/%v", first, duplicate, err, duplicateErr)
	}
}

func TestWorkbenchProjectionStreamAdaptersPreserveDistinctValueBasisContracts(t *testing.T) {
	commitFailure := errors.New("stop after proof message")
	cfg := DefaultConfig().Workbench
	store := &projectionIngestionCaptureStore{WorkbenchStore: NewInMemoryWorkbenchStore()}

	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scadaReader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Topic: cfg.ScadaTopic, Offset: 31, Value: scadaRaw}}, commitErr: commitFailure}
	err := RunWorkbenchScadaProjectionConsumer(context.Background(), cfg, scadaReader, store, NewSimopsConsumerMetrics())
	assertProjectionIngestionStream(t, err, WorkbenchProjectionMeasured)
	if store.scada.Frame.ValueBasis != WorkbenchValueMeasured {
		t.Fatalf("SCADA adapter collapsed measured basis: %#v", store.scada.Frame)
	}

	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-INGESTION-PROOF"))
	resultReader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Topic: cfg.ResultsTopic, Offset: 32, Value: resultRaw}}, commitErr: commitFailure}
	err = RunWorkbenchResultProjectionConsumer(context.Background(), cfg, resultReader, store, NewSimopsConsumerMetrics())
	assertProjectionIngestionStream(t, err, WorkbenchProjectionSimulated)
	if store.result.Frame.ValueBasis != WorkbenchValueSimulated {
		t.Fatalf("result adapter collapsed simulated basis: %#v", store.result.Frame)
	}

	twinState, _ := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-INGESTION-PROOF"), time.Now().UTC())
	twinRaw, _ := json.Marshal(twinState)
	twinReader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{{Topic: cfg.TwinStateTopic, Offset: 33, Value: twinRaw}}, commitErr: commitFailure}
	err = RunWorkbenchTwinProjectionConsumer(context.Background(), cfg, twinReader, store, NewSimopsConsumerMetrics())
	assertProjectionIngestionStream(t, err, WorkbenchProjectionTwin)
	bases := map[WorkbenchValueBasis]bool{}
	for _, entity := range store.twin.State.Entities {
		for _, value := range entity.Values {
			bases[value.ValueBasis] = true
		}
	}
	if !bases[WorkbenchValueMeasured] || !bases[WorkbenchValueSimulated] || !bases[WorkbenchValueImputed] {
		t.Fatalf("twin adapter collapsed mixed value bases: %#v", bases)
	}
}

func assertProjectionIngestionStream(t *testing.T, err error, want WorkbenchProjectionStream) {
	t.Helper()
	var ingestionErr *WorkbenchProjectionIngestionError
	if !errors.As(err, &ingestionErr) || ingestionErr.Stream != want || ingestionErr.Stage != WorkbenchProjectionIngestionCommit {
		t.Fatalf("expected %s commit error, got %T %v", want, err, err)
	}
}

type projectionIngestionTestReader struct {
	messages    []SimopsBrokerMessage
	committed   []SimopsBrokerMessage
	commitErr   error
	steps       *[]string
	afterCommit func(int)
	fetchErr    error
}

func (r *projectionIngestionTestReader) FetchMessage(ctx context.Context) (SimopsBrokerMessage, error) {
	if r.steps != nil {
		*r.steps = append(*r.steps, "fetch")
	}
	if r.fetchErr != nil {
		return SimopsBrokerMessage{}, r.fetchErr
	}
	if len(r.messages) == 0 {
		<-ctx.Done()
		return SimopsBrokerMessage{}, ctx.Err()
	}
	message := r.messages[0]
	r.messages = r.messages[1:]
	return message, nil
}

func (r *projectionIngestionTestReader) CommitMessages(_ context.Context, messages ...SimopsBrokerMessage) error {
	if r.steps != nil {
		*r.steps = append(*r.steps, "commit")
	}
	r.committed = append(r.committed, messages...)
	if r.afterCommit != nil {
		r.afterCommit(len(r.committed))
	}
	return r.commitErr
}

func (*projectionIngestionTestReader) Close() error { return nil }

type projectionIngestionCaptureStore struct {
	WorkbenchStore
	scada  ScadaProjection
	result SimopsResultProjection
	twin   TwinStateProjection
}

func (s *projectionIngestionCaptureStore) SaveScadaProjection(_ string, projection ScadaProjection) (bool, error) {
	s.scada = projection
	return true, nil
}

func (s *projectionIngestionCaptureStore) SaveResultProjection(_ string, projection SimopsResultProjection) (bool, error) {
	s.result = projection
	return true, nil
}

func (s *projectionIngestionCaptureStore) SaveTwinStateProjection(_ string, projection TwinStateProjection) (bool, error) {
	s.twin = projection
	return true, nil
}
