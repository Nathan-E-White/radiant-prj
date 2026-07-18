package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTwinStatePublisherDependsOnlyOnPublicationPersistence(t *testing.T) {
	store := &focusedTwinStatePublicationStore{}
	events := &twinPublicationEventLog{}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("scada.telemetry.v1", 0, 5)

	outcome, err := publisher.Publish(context.Background(), publication)
	if err != nil || !outcome.Persisted || !outcome.Delivered || store.saved.PublicationID != publication.PublicationID {
		t.Fatalf("focused persistence outcome=%#v saved=%#v err=%v", outcome, store.saved, err)
	}
	if err := publisher.Acknowledge(publication.Source); err != nil || !store.saved.Acknowledged {
		t.Fatalf("focused acknowledgement saved=%#v err=%v", store.saved, err)
	}
}

func TestInMemoryTwinStatePublicationStoreContract(t *testing.T) {
	assertTwinStatePublicationStoreContract(t, NewInMemoryWorkbenchStore())
}

func assertTwinStatePublicationStoreContract(t *testing.T, store TwinStatePublicationStore) {
	t.Helper()
	publication := twinPublicationFixture("simops.results.v1", 2, 41)
	wantAsOf := publication.State.AsOf
	wantStep := publication.Lineage[0].ProcessingSteps[0]

	written, err := store.SaveTwinStatePublication(publication)
	if err != nil || !written {
		t.Fatalf("save publication written=%v err=%v", written, err)
	}
	publication.State.AsOf = publication.State.AsOf.Add(time.Hour)
	publication.Lineage[0].ProcessingSteps[0] = "caller mutation"
	written, err = store.SaveTwinStatePublication(publication)
	if err != nil || written {
		t.Fatalf("save duplicate publication written=%v err=%v", written, err)
	}
	canonical, err := store.GetTwinStatePublication(publication.PublicationID)
	if err != nil || !canonical.State.AsOf.Equal(wantAsOf) || canonical.Lineage[0].ProcessingSteps[0] != wantStep {
		t.Fatalf("load canonical publication=%#v err=%v", canonical, err)
	}
	if err := store.AcknowledgeTwinStatePublication(publication.PublicationID); err != nil {
		t.Fatalf("acknowledge publication: %v", err)
	}
	tombstone, err := store.GetTwinStatePublication(publication.PublicationID)
	if err != nil || !tombstone.Acknowledged || tombstone.State.SchemaVersion != "" || len(tombstone.Lineage) != 0 {
		t.Fatalf("load acknowledged publication=%#v err=%v", tombstone, err)
	}
}

func TestTwinStatePublisherPersistsBeforeEventDelivery(t *testing.T) {
	steps := []string{}
	store := &twinPublicationStore{WorkbenchStore: NewInMemoryWorkbenchStore(), steps: &steps}
	events := &twinPublicationEventLog{steps: &steps}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("scada.telemetry.v1", 2, 41)

	outcome, err := publisher.Publish(context.Background(), publication)
	if err != nil || !outcome.Persisted || outcome.Duplicate || !outcome.Delivered || outcome.Recovery != TwinStatePublicationComplete {
		t.Fatalf("unexpected publication outcome=%#v err=%v", outcome, err)
	}
	if want := []string{"persist", "event"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("publication order=%v want=%v", steps, want)
	}
}

func TestTwinStatePublisherDeliversOwnedCanonicalValue(t *testing.T) {
	publication := twinPublicationFixture("scada.telemetry.v1", 2, 42)
	wantAsOf := publication.State.AsOf
	store := &twinPublicationStore{WorkbenchStore: NewInMemoryWorkbenchStore()}
	store.afterSave = func() {
		publication.State.AsOf = publication.State.AsOf.Add(time.Hour)
		publication.State.Entities[0].Values[0].Value["scalar"] = -999.0
		publication.Lineage[0].ProcessingSteps[0] = "caller mutation after persistence"
	}
	events := &twinPublicationEventLog{}
	if _, err := NewTwinStatePublisher(store, events).Publish(context.Background(), publication); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(events.publications) != 1 || !events.publications[0].State.AsOf.Equal(wantAsOf) || events.publications[0].State.Entities[0].Values[0].Value["scalar"] == -999.0 || events.publications[0].Lineage[0].ProcessingSteps[0] == "caller mutation after persistence" {
		t.Fatalf("event diverged from persisted canonical value: %#v", events.publications)
	}
}

func TestTwinStatePublicationIDIsStableOnlyForTheSameSourceCoordinate(t *testing.T) {
	base := twinPublicationFixture("scada.telemetry.v1", 2, 41)
	again := twinPublicationFixture("scada.telemetry.v1", 2, 41)
	if base.PublicationID != again.PublicationID {
		t.Fatalf("same source coordinate produced different IDs: %q %q", base.PublicationID, again.PublicationID)
	}
	variants := []TwinStatePublication{
		twinPublicationFixture("simops.results.v1", 2, 41),
		twinPublicationFixture("scada.telemetry.v1", 3, 41),
		twinPublicationFixture("scada.telemetry.v1", 2, 42),
	}
	for _, variant := range variants {
		if variant.PublicationID == base.PublicationID {
			t.Fatalf("distinct source coordinate reused publication ID %q: %#v", base.PublicationID, variant.Source)
		}
	}
}

func TestTwinStatePublisherStoreFailureDeliversNothingAndCanRetry(t *testing.T) {
	storeFailure := errors.New("Twin store unavailable")
	store := &twinPublicationStore{WorkbenchStore: NewInMemoryWorkbenchStore(), err: storeFailure}
	events := &twinPublicationEventLog{}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("simops.results.v1", 0, 7)

	outcome, err := publisher.Publish(context.Background(), publication)
	var publicationErr *TwinStatePublicationError
	if !errors.As(err, &publicationErr) || publicationErr.Stage != TwinStatePublicationPersistence || !errors.Is(err, storeFailure) {
		t.Fatalf("unexpected store failure outcome=%#v err=%T %v", outcome, err, err)
	}
	if outcome.Persisted || outcome.Delivered || outcome.Recovery != TwinStatePublicationRetry || events.calls != 0 {
		t.Fatalf("store failure leaked an event: outcome=%#v eventCalls=%d", outcome, events.calls)
	}

	store.err = nil
	outcome, err = publisher.Publish(context.Background(), publication)
	if err != nil || !outcome.Persisted || !outcome.Delivered || outcome.Duplicate {
		t.Fatalf("store recovery outcome=%#v err=%v", outcome, err)
	}
}

func TestTwinStatePublisherRejectsUnexplainedImputedStateAndAliasedIDs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TwinStatePublication)
	}{
		{name: "aliased publication ID", mutate: func(publication *TwinStatePublication) {
			publication.PublicationID = "twinpub-wrong-source"
		}},
		{name: "missing Imputed lineage", mutate: func(publication *TwinStatePublication) {
			publication.Lineage = nil
		}},
		{name: "mismatched Imputed lineage", mutate: func(publication *TwinStatePublication) {
			for index := range publication.Lineage {
				if publication.Lineage[index].ValueBasis == WorkbenchValueImputed {
					publication.Lineage[index].ValueID = "different-value"
				}
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &twinPublicationStore{WorkbenchStore: NewInMemoryWorkbenchStore()}
			events := &twinPublicationEventLog{}
			publication := twinPublicationFixture("simops.results.v1", 0, 8)
			test.mutate(&publication)
			if _, err := NewTwinStatePublisher(store, events).Publish(context.Background(), publication); err == nil {
				t.Fatal("expected publication validation failure")
			}
			if store.commits != 0 || events.calls != 0 {
				t.Fatalf("invalid publication escaped boundary: commits=%d eventCalls=%d", store.commits, events.calls)
			}
		})
	}
}

func TestTwinStatePublisherEventFailureRetriesDeliveryWithoutRepersisting(t *testing.T) {
	eventFailure := errors.New("event delivery ambiguous")
	store := &twinPublicationStore{WorkbenchStore: NewInMemoryWorkbenchStore()}
	events := &twinPublicationEventLog{err: eventFailure}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("scada.telemetry.v1", 1, 9)

	outcome, err := publisher.Publish(context.Background(), publication)
	var publicationErr *TwinStatePublicationError
	if !errors.As(err, &publicationErr) || publicationErr.Stage != TwinStatePublicationEventDelivery || !errors.Is(err, eventFailure) {
		t.Fatalf("unexpected event failure outcome=%#v err=%T %v", outcome, err, err)
	}
	if !outcome.Persisted || outcome.Duplicate || outcome.Delivered || outcome.Recovery != TwinStatePublicationRetry || store.commits != 1 {
		t.Fatalf("event failure did not preserve one explained state: outcome=%#v commits=%d", outcome, store.commits)
	}

	events.err = nil
	persistedAsOf := publication.State.AsOf
	publication.State.AsOf = publication.State.AsOf.Add(time.Hour)
	publication.Lineage[0].ProcessingSteps[0] = "different retry payload"
	outcome, err = publisher.Publish(context.Background(), publication)
	if err != nil || outcome.Persisted || !outcome.Duplicate || !outcome.Delivered || outcome.Recovery != TwinStatePublicationComplete || store.commits != 1 || events.calls != 2 {
		t.Fatalf("event retry outcome=%#v err=%v commits=%d eventCalls=%d", outcome, err, store.commits, events.calls)
	}
	if !events.publications[1].State.AsOf.Equal(persistedAsOf) || events.publications[1].Lineage[0].ProcessingSteps[0] == "different retry payload" {
		t.Fatalf("retry delivered caller drift instead of persisted publication: %#v", events.publications[1])
	}
}

func TestInMemoryTwinStatePublisherRetriesPersistedCanonicalPublication(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	events := &twinPublicationEventLog{err: errors.New("event delivery ambiguous")}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("scada.telemetry.v1", 2, 19)

	if _, err := publisher.Publish(context.Background(), publication); err == nil {
		t.Fatal("expected event delivery failure")
	}
	persistedAsOf := publication.State.AsOf
	publication.State.AsOf = publication.State.AsOf.Add(time.Hour)
	publication.Lineage[0].ProcessingSteps[0] = "different retry payload"
	events.err = nil

	outcome, err := publisher.Publish(context.Background(), publication)
	if err != nil || !outcome.Duplicate || !outcome.Delivered {
		t.Fatalf("retry outcome=%#v err=%v", outcome, err)
	}
	if len(events.publications) != 2 || !events.publications[1].State.AsOf.Equal(persistedAsOf) || events.publications[1].Lineage[0].ProcessingSteps[0] == "different retry payload" {
		t.Fatalf("retry did not deliver the in-memory canonical publication: %#v", events.publications)
	}
}

func TestTwinStatePublisherResumesPersistedPublicationAfterRestart(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	events := &twinPublicationEventLog{err: errors.New("event delivery ambiguous")}
	publication := twinPublicationFixture("simops.results.v1", 4, 27)
	if _, err := NewTwinStatePublisher(store, events).Publish(context.Background(), publication); err == nil {
		t.Fatal("expected event delivery failure")
	}

	events.err = nil
	outcome, found, err := NewTwinStatePublisher(store, events).Resume(context.Background(), publication.Source)
	if err != nil || !found || !outcome.Duplicate || !outcome.Delivered || outcome.Recovery != TwinStatePublicationComplete {
		t.Fatalf("restart resume outcome=%#v found=%v err=%v", outcome, found, err)
	}
	if len(events.publications) != 2 || events.publications[1].PublicationID != publication.PublicationID {
		t.Fatalf("restart did not redeliver the persisted publication: %#v", events.publications)
	}
}

func TestTwinStatePublisherAcknowledgementReleasesCanonicalPayload(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	publication := twinPublicationFixture("simops.results.v1", 1, 29)
	events := &twinPublicationEventLog{}
	publisher := NewTwinStatePublisher(store, events)
	if _, err := publisher.Publish(context.Background(), publication); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := publisher.Acknowledge(publication.Source); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	tombstone, err := store.GetTwinStatePublication(publication.PublicationID)
	if err != nil || !tombstone.Acknowledged || tombstone.State.SchemaVersion != "" || len(tombstone.Lineage) != 0 {
		t.Fatalf("acknowledgement did not compact canonical payload: publication=%#v err=%v", tombstone, err)
	}
	outcome, found, err := publisher.Resume(context.Background(), publication.Source)
	if err != nil || !found || !outcome.Delivered || outcome.Recovery != TwinStatePublicationComplete || events.calls != 1 {
		t.Fatalf("acknowledged replay outcome=%#v found=%v events=%d err=%v", outcome, found, events.calls, err)
	}
	outcome, err = publisher.Publish(context.Background(), publication)
	if err != nil || !outcome.Duplicate || !outcome.Delivered || outcome.Recovery != TwinStatePublicationComplete || events.calls != 1 {
		t.Fatalf("acknowledged duplicate publish outcome=%#v events=%d err=%v", outcome, events.calls, err)
	}
}

func TestTwinProjectorRestartResumesBeforeCommittingSourceReplay(t *testing.T) {
	cfg := DefaultConfig().Workbench
	store := NewInMemoryWorkbenchStore()
	events := &twinPublicationEventLog{err: errors.New("event delivery ambiguous")}
	publication := twinPublicationFixture(cfg.ResultsTopic, 0, 31)
	if _, err := NewTwinStatePublisher(store, events).Publish(context.Background(), publication); err == nil {
		t.Fatal("expected event delivery failure")
	}

	events.err = nil
	commitStop := errors.New("stop after source commit")
	raw, err := json.Marshal(simopsResultFixture("RUN-RESTART-RESUME"))
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	reader := &projectionIngestionTestReader{
		messages:  []SimopsBrokerMessage{{Topic: cfg.ResultsTopic, Partition: 0, Offset: 31, Value: raw}},
		commitErr: commitStop,
	}
	projector, err := NewTwinProjector(cfg, store, events)
	if err != nil {
		t.Fatalf("restart projector: %v", err)
	}
	err = projector.runResults(context.Background(), reader, NewSimopsConsumerMetrics())
	if !errors.Is(err, commitStop) || len(reader.committed) != 1 {
		t.Fatalf("source replay was not committed after resume: commits=%#v err=%v", reader.committed, err)
	}
	if len(events.publications) != 2 || events.publications[1].PublicationID != publication.PublicationID {
		t.Fatalf("projector committed without resuming canonical delivery: %#v", events.publications)
	}
}

func TestTwinProjectorRestartRebuildsStoreFailureFromHydratedJoin(t *testing.T) {
	cfg := DefaultConfig().Workbench
	baseStore := NewInMemoryWorkbenchStore()
	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scada, err := ProjectScadaFrame(cfg.ScadaTopic, 0, 5, scadaRaw)
	if err != nil {
		t.Fatalf("project SCADA fixture: %v", err)
	}
	if written, err := baseStore.SaveScadaProjection(cfg.ScadaProjectionConsumerGroup, scada); err != nil || !written {
		t.Fatalf("persist restart join input: written=%v err=%v", written, err)
	}

	storeFailure := errors.New("Twin store unavailable")
	store := &twinPublicationStore{WorkbenchStore: baseStore, err: storeFailure}
	events := &twinPublicationEventLog{}
	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-STORE-RESTART"))
	message := SimopsBrokerMessage{Topic: cfg.ResultsTopic, Partition: 0, Offset: 44, Value: resultRaw}
	firstReader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{message}, commitErr: errors.New("unexpected source commit")}
	projector, err := NewTwinProjector(cfg, store, events)
	if err != nil {
		t.Fatalf("first projector: %v", err)
	}
	if err := projector.runResults(context.Background(), firstReader, NewSimopsConsumerMetrics()); !errors.Is(err, storeFailure) {
		t.Fatalf("expected store failure, got %v", err)
	}
	if len(firstReader.committed) != 0 || events.calls != 0 {
		t.Fatalf("store failure committed or delivered source: commits=%#v eventCalls=%d", firstReader.committed, events.calls)
	}

	store.err = nil
	commitStop := errors.New("stop after recovered source commit")
	secondReader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{message}, commitErr: commitStop}
	projector, err = NewTwinProjector(cfg, store, events)
	if err != nil {
		t.Fatalf("restart projector: %v", err)
	}
	if err := projector.runResults(context.Background(), secondReader, NewSimopsConsumerMetrics()); !errors.Is(err, commitStop) {
		t.Fatalf("expected recovered commit stop, got %v", err)
	}
	if len(secondReader.committed) != 1 || events.calls != 1 {
		t.Fatalf("restart did not rebuild publication: commits=%#v eventCalls=%d", secondReader.committed, events.calls)
	}
}

func TestTwinProjectorAcknowledgesBeforeCommittingSource(t *testing.T) {
	cfg := DefaultConfig().Workbench
	baseStore := NewInMemoryWorkbenchStore()
	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scada, _ := ProjectScadaFrame(cfg.ScadaTopic, 0, 6, scadaRaw)
	if written, err := baseStore.SaveScadaProjection(cfg.ScadaProjectionConsumerGroup, scada); err != nil || !written {
		t.Fatalf("persist join input written=%v err=%v", written, err)
	}
	ackFailure := errors.New("publication acknowledgement unavailable")
	store := &twinPublicationStore{WorkbenchStore: baseStore, ackErr: ackFailure}
	projector, err := NewTwinProjector(cfg, store, &twinPublicationEventLog{})
	if err != nil {
		t.Fatalf("new projector: %v", err)
	}
	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-ACK-ORDER"))
	message := SimopsBrokerMessage{Topic: cfg.ResultsTopic, Partition: 0, Offset: 45, Value: resultRaw}
	reader := &projectionIngestionTestReader{messages: []SimopsBrokerMessage{message}, commitErr: errors.New("source commit must not run")}
	if err := projector.runResults(context.Background(), reader, NewSimopsConsumerMetrics()); !errors.Is(err, ackFailure) {
		t.Fatalf("expected acknowledgement failure, got %v", err)
	}
	if len(reader.committed) != 0 {
		t.Fatalf("source committed before acknowledgement: %#v", reader.committed)
	}
	publicationID := twinStatePublicationID(WorkbenchProjectionPosition{Topic: cfg.ResultsTopic, Partition: 0, Offset: 45})
	canonical, err := store.GetTwinStatePublication(publicationID)
	if err != nil || canonical.Acknowledged || canonical.State.SchemaVersion == "" {
		t.Fatalf("failed acknowledgement did not preserve retry payload: publication=%#v err=%v", canonical, err)
	}
}

func TestTwinProjectorSerializesCrossStreamPublication(t *testing.T) {
	cfg := DefaultConfig().Workbench
	entered := make(chan string, 2)
	publisher := &blockingTwinPublisher{entered: entered}
	result := simopsResultFixture("RUN-SERIALIZED")
	projector := &TwinProjector{
		cfg: cfg, publisher: publisher, now: time.Now,
		measured: map[string]ScadaTelemetryFrame{scadaFrameFixture().TagID: scadaFrameFixture()}, result: &result,
	}
	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	resultRaw, _ := json.Marshal(result)
	scadaMessage := SimopsBrokerMessage{Topic: cfg.ScadaTopic, Partition: 0, Offset: 51, Value: scadaRaw}
	resultMessage := SimopsBrokerMessage{Topic: cfg.ResultsTopic, Partition: 0, Offset: 52, Value: resultRaw}
	commitEntered := make(chan struct{})
	releaseCommit := make(chan struct{})
	firstReader := &lifecycleCommitReader{entered: commitEntered, release: releaseCommit}
	secondReader := &lifecycleCommitReader{}
	metrics := NewSimopsConsumerMetrics()

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- projector.consumeScada(context.Background(), firstReader, metrics, scadaMessage, WorkbenchProjectionPosition{Topic: cfg.ScadaTopic, Offset: 51})
	}()
	if topic := <-entered; topic != cfg.ScadaTopic {
		t.Fatalf("first publication topic=%q", topic)
	}
	<-commitEntered
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- projector.consumeResult(context.Background(), secondReader, metrics, resultMessage, WorkbenchProjectionPosition{Topic: cfg.ResultsTopic, Offset: 52})
	}()
	overtook := ""
	select {
	case topic := <-entered:
		overtook = topic
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseCommit)
	if err := <-firstDone; err != nil {
		t.Fatalf("first publication: %v", err)
	}
	if overtook == "" {
		if topic := <-entered; topic != cfg.ResultsTopic {
			t.Fatalf("second publication topic=%q", topic)
		}
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second publication: %v", err)
	}
	if overtook != "" {
		t.Fatalf("second publication %q overtook blocked first publication", overtook)
	}
}

func TestTwinStatePublicationIDDeduplicatesDifferentBrokerOffsets(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	events := &twinPublicationEventLog{}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("simops.results.v1", 3, 12)
	if _, err := publisher.Publish(context.Background(), publication); err != nil {
		t.Fatalf("publish transition: %v", err)
	}
	if len(events.publications) != 1 {
		t.Fatalf("captured publications=%d", len(events.publications))
	}

	for offset := int64(100); offset <= 101; offset++ {
		projection, err := ProjectTwinStatePublication("digital-twin.state.v1", 0, offset, events.publications[0])
		if err != nil {
			t.Fatalf("project duplicate offset %d: %v", offset, err)
		}
		written, err := store.SaveTwinStateProjection("workbench-twin-projection", projection)
		if err != nil || written {
			t.Fatalf("semantic duplicate offset=%d written=%v err=%v", offset, written, err)
		}
	}
	snapshot, _ := store.Snapshot()
	if snapshot.Generation != 1 || len(snapshot.Lineage) != len(publication.Lineage) {
		t.Fatalf("duplicate delivery changed explained state: %#v", snapshot)
	}
}

func twinPublicationFixture(topic string, partition int, offset int64) TwinStatePublication {
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-PUBLICATION"), time.Now().UTC())
	return NewTwinStatePublication(WorkbenchProjectionPosition{Topic: topic, Partition: partition, Offset: offset}, "digital-twin.state.v1", state, lineage)
}

type twinPublicationStore struct {
	WorkbenchStore
	err        error
	steps      *[]string
	seen       map[string]struct{}
	commits    int
	projection TwinStateProjection
	afterSave  func()
	ackErr     error
}

type focusedTwinStatePublicationStore struct {
	saved TwinStatePublication
}

func (s *focusedTwinStatePublicationStore) SaveTwinStatePublication(publication TwinStatePublication) (bool, error) {
	if s.saved.PublicationID == publication.PublicationID {
		return false, nil
	}
	owned, err := cloneWorkbenchValue(publication)
	if err != nil {
		return false, err
	}
	s.saved = owned
	return true, nil
}

func (s *focusedTwinStatePublicationStore) GetTwinStatePublication(publicationID string) (TwinStatePublication, error) {
	if s.saved.PublicationID != publicationID {
		return TwinStatePublication{}, ErrWorkbenchNotFound
	}
	return cloneWorkbenchValue(s.saved)
}

func (s *focusedTwinStatePublicationStore) AcknowledgeTwinStatePublication(publicationID string) error {
	if s.saved.PublicationID != publicationID {
		return ErrWorkbenchNotFound
	}
	s.saved = TwinStatePublication{
		PublicationID:  s.saved.PublicationID,
		Source:         s.saved.Source,
		TwinStateTopic: s.saved.TwinStateTopic,
		Acknowledged:   true,
	}
	return nil
}

func (s *twinPublicationStore) SaveTwinStatePublication(publication TwinStatePublication) (bool, error) {
	if s.steps != nil {
		*s.steps = append(*s.steps, "persist")
	}
	if s.err != nil {
		return false, s.err
	}
	projection, err := ProjectTwinStatePublication(
		publication.TwinStateTopic,
		publication.Source.Partition,
		publication.Source.Offset,
		publication,
	)
	if err != nil {
		return false, err
	}
	if s.seen == nil {
		s.seen = make(map[string]struct{})
	}
	if _, ok := s.seen[projection.PublicationID]; ok {
		return false, nil
	}
	s.seen[projection.PublicationID] = struct{}{}
	s.commits++
	owned, err := cloneWorkbenchValue(projection)
	if err != nil {
		return false, err
	}
	s.projection = owned
	if s.afterSave != nil {
		s.afterSave()
	}
	return true, nil
}

func (s *twinPublicationStore) GetTwinStatePublication(publicationID string) (TwinStatePublication, error) {
	if s.projection.PublicationID != publicationID {
		return TwinStatePublication{}, ErrWorkbenchNotFound
	}
	return cloneWorkbenchValue(TwinStatePublication{
		PublicationID: publicationID, Source: s.projection.PublicationSource,
		TwinStateTopic: s.projection.RedpandaTopic, State: s.projection.State, Lineage: s.projection.Lineage,
	})
}

func (s *twinPublicationStore) AcknowledgeTwinStatePublication(publicationID string) error {
	if s.ackErr != nil {
		return s.ackErr
	}
	return s.WorkbenchStore.AcknowledgeTwinStatePublication(publicationID)
}

type twinPublicationEventLog struct {
	err          error
	steps        *[]string
	calls        int
	publications []TwinStatePublication
}

type blockingTwinPublisher struct {
	entered chan string
}

func (p *blockingTwinPublisher) Publish(_ context.Context, publication TwinStatePublication) (TwinStatePublicationOutcome, error) {
	p.entered <- publication.Source.Topic
	return TwinStatePublicationOutcome{PublicationID: publication.PublicationID, Delivered: true, Recovery: TwinStatePublicationComplete}, nil
}

func (*blockingTwinPublisher) Resume(_ context.Context, _ WorkbenchProjectionPosition) (TwinStatePublicationOutcome, bool, error) {
	return TwinStatePublicationOutcome{}, false, nil
}

func (*blockingTwinPublisher) Acknowledge(_ WorkbenchProjectionPosition) error { return nil }

type lifecycleCommitReader struct {
	entered chan struct{}
	release chan struct{}
}

func (*lifecycleCommitReader) FetchMessage(context.Context) (SimopsBrokerMessage, error) {
	return SimopsBrokerMessage{}, errors.New("not used")
}

func (r *lifecycleCommitReader) CommitMessages(_ context.Context, _ ...SimopsBrokerMessage) error {
	if r.entered != nil {
		close(r.entered)
	}
	if r.release != nil {
		<-r.release
	}
	return nil
}

func (*lifecycleCommitReader) Close() error { return nil }

func (*twinPublicationEventLog) PublishScada(context.Context, ScadaTelemetryFrame) error { return nil }
func (*twinPublicationEventLog) PublishResult(context.Context, SimopsResultFrame) error  { return nil }
func (l *twinPublicationEventLog) PublishTwinState(_ context.Context, publication TwinStatePublication) error {
	if l.steps != nil {
		*l.steps = append(*l.steps, "event")
	}
	l.calls++
	l.publications = append(l.publications, publication)
	return l.err
}
