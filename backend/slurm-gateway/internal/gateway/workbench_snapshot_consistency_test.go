package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestTwinStateEventCarriesLineageAsOneTransition(t *testing.T) {
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-TWIN-TRANSITION-EVENT"), time.Now().UTC())
	writer := &capturingKafkaWriter{}
	eventLog := &RedpandaWorkbenchEventLog{twinWriter: writer}
	if err := eventLog.PublishTwinState(context.Background(), state, lineage); err != nil {
		t.Fatalf("publish Twin transition: %v", err)
	}
	if len(writer.messages) != 1 {
		t.Fatalf("published messages=%d", len(writer.messages))
	}
	var publishedState DigitalTwinState
	if err := json.Unmarshal(writer.messages[0].Value, &publishedState); err != nil || publishedState.SchemaVersion != WorkbenchTwinStateSchemaVersion {
		t.Fatalf("published value is not a v1 Twin State: state=%#v err=%v", publishedState, err)
	}
	projection, err := ProjectTwinState("twin-transition", 0, 1, writer.messages[0].Value, writer.messages[0].Headers...)
	if err != nil || projection.State.TwinID != state.TwinID || !projection.LineagePresent || len(projection.Lineage) != len(lineage) {
		t.Fatalf("Twin transition round trip projection=%#v err=%v", projection, err)
	}

	legacyRaw, _ := json.Marshal(state)
	legacy, err := ProjectTwinState("legacy-twin-state", 0, 2, legacyRaw)
	if err != nil || legacy.State.TwinID != state.TwinID || legacy.LineagePresent || len(legacy.Lineage) != 0 {
		t.Fatalf("legacy Twin state compatibility projection=%#v err=%v", legacy, err)
	}
}

func TestInMemoryTwinTransitionReplacesTheActiveLineageSet(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-LINEAGE-REPLACE"), time.Now().UTC())
	first := TwinStateProjection{State: state, Lineage: lineage, LineagePresent: true, RedpandaTopic: "lineage-replace", RedpandaOffset: 1}
	if written, err := store.SaveTwinStateProjection("lineage-replace-test", first); err != nil || !written {
		t.Fatalf("save full lineage transition written=%v err=%v", written, err)
	}
	reduced := first
	reduced.RedpandaOffset = 2
	reduced.Lineage = lineage[:1]
	if written, err := store.SaveTwinStateProjection("lineage-replace-test", reduced); err != nil || !written {
		t.Fatalf("save reduced lineage transition written=%v err=%v", written, err)
	}
	afterReduced, _ := store.Snapshot()
	if afterReduced.Generation != 2 || len(afterReduced.Lineage) != 1 || afterReduced.Lineage[0].LineageID != lineage[0].LineageID {
		t.Fatalf("stale lineage survived replacement: %#v", afterReduced)
	}
	legacy := reduced
	legacy.RedpandaOffset = 3
	legacy.Lineage = nil
	legacy.LineagePresent = false
	if written, err := store.SaveTwinStateProjection("lineage-replace-test", legacy); err != nil || !written {
		t.Fatalf("save legacy state-only transition written=%v err=%v", written, err)
	}
	afterLegacy, _ := store.Snapshot()
	if afterLegacy.Generation != 3 || len(afterLegacy.Lineage) != 1 {
		t.Fatalf("legacy transition discarded separately published lineage: %#v", afterLegacy)
	}
	explicitEmpty := legacy
	explicitEmpty.RedpandaOffset = 4
	explicitEmpty.LineagePresent = true
	if written, err := store.SaveTwinStateProjection("lineage-replace-test", explicitEmpty); err != nil || !written {
		t.Fatalf("save explicitly empty lineage transition written=%v err=%v", written, err)
	}
	afterEmpty, _ := store.Snapshot()
	if afterEmpty.Generation != 4 || len(afterEmpty.Lineage) != 0 {
		t.Fatalf("explicit empty lineage did not clear active set: %#v", afterEmpty)
	}
}

func TestPostgresWorkbenchSnapshotUsesOneRepeatableReadTransaction(t *testing.T) {
	options := workbenchSnapshotTxOptions()
	if options.Isolation != sql.LevelRepeatableRead || !options.ReadOnly {
		t.Fatalf("Snapshot transaction must be read-only repeatable-read: %#v", options)
	}
}

func TestInMemoryWorkbenchSnapshotAdvancesOneCoherentGenerationPerCommit(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	initial, err := store.Snapshot()
	if err != nil || initial.Generation != 0 || initial.State.SnapshotGeneration != 0 {
		t.Fatalf("unexpected initial Snapshot: snapshot=%#v err=%v", initial, err)
	}

	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scada, _ := ProjectScadaFrame("scada", 0, 1, scadaRaw)
	if written, err := store.SaveScadaProjection("scada-consumer", scada); err != nil || !written {
		t.Fatalf("save measured projection written=%v err=%v", written, err)
	}
	afterMeasured, _ := store.Snapshot()
	if afterMeasured.Generation != 1 || afterMeasured.State.SnapshotGeneration != 1 || len(afterMeasured.Measured) != 1 {
		t.Fatalf("measured commit did not produce generation 1: %#v", afterMeasured)
	}
	if written, err := store.SaveScadaProjection("scada-consumer", scada); err != nil || written {
		t.Fatalf("duplicate measured projection written=%v err=%v", written, err)
	}
	afterDuplicate, _ := store.Snapshot()
	if afterDuplicate.Generation != afterMeasured.Generation {
		t.Fatalf("duplicate advanced generation: before=%d after=%d", afterMeasured.Generation, afterDuplicate.Generation)
	}

	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-SNAPSHOT-GENERATION"))
	result, _ := ProjectSimopsResultFrame("results", 0, 2, resultRaw)
	if written, err := store.SaveResultProjection("result-consumer", result); err != nil || !written {
		t.Fatalf("save result projection written=%v err=%v", written, err)
	}
	twinState, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-SNAPSHOT-GENERATION"), time.Now().UTC())
	twinRaw, _ := json.Marshal(twinState)
	twin, _ := ProjectTwinState("twin", 0, 3, twinRaw)
	twin.Lineage = lineage
	twin.LineagePresent = true
	if written, err := store.SaveTwinStateProjection("twin-consumer", twin); err != nil || !written {
		t.Fatalf("save Twin projection written=%v err=%v", written, err)
	}
	if len(lineage) == 0 {
		t.Fatal("expected lineage fixture")
	}
	complete, _ := store.Snapshot()
	if complete.Generation != 3 || complete.State.SnapshotGeneration != 3 || len(complete.Results) != 1 || complete.Twin.SchemaVersion == "" || len(complete.Lineage) != len(lineage) {
		t.Fatalf("unexpected complete generation: %#v", complete)
	}
}

func TestInMemoryWorkbenchSnapshotOwnsMutableTransitionValues(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	scadaRaw, _ := json.Marshal(scadaFrameFixture())
	scada, _ := ProjectScadaFrame("owned-scada", 0, 1, scadaRaw)
	if written, err := store.SaveScadaProjection("owned-test", scada); err != nil || !written {
		t.Fatalf("save owned measured frame written=%v err=%v", written, err)
	}
	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-OWNED-SNAPSHOT"))
	result, _ := ProjectSimopsResultFrame("owned-result", 0, 2, resultRaw)
	if written, err := store.SaveResultProjection("owned-test", result); err != nil || !written {
		t.Fatalf("save owned result frame written=%v err=%v", written, err)
	}
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-OWNED-SNAPSHOT"), time.Now().UTC())
	projection := TwinStateProjection{State: state, Lineage: lineage, LineagePresent: true, RedpandaTopic: "owned-twin", RedpandaOffset: 3}
	if written, err := store.SaveTwinStateProjection("owned-test", projection); err != nil || !written {
		t.Fatalf("save owned transition written=%v err=%v", written, err)
	}
	before, _ := store.Snapshot()
	wantValue := before.Twin.Entities[0].Values[0].Value["scalar"]
	wantStep := before.Lineage[0].ProcessingSteps[0]
	wantMeasured := before.Measured[0].Value["scalar"]
	wantResult := string(before.Results[0].Values[0].Value)

	scada.Frame.Value["scalar"] = 777.0
	result.Frame.Values[0].Value[0] = 'X'
	state.Entities[0].Values[0].Value["scalar"] = 999.0
	lineage[0].ProcessingSteps[0] = "mutated input"
	before.Twin.Entities[0].Values[0].Value["scalar"] = 888.0
	before.Lineage[0].ProcessingSteps[0] = "mutated output"
	before.Measured[0].Value["scalar"] = 666.0
	before.Results[0].Values[0].Value[0] = 'Y'

	after, _ := store.Snapshot()
	if after.Generation != 3 || after.Twin.Entities[0].Values[0].Value["scalar"] != wantValue || after.Lineage[0].ProcessingSteps[0] != wantStep || after.Measured[0].Value["scalar"] != wantMeasured || string(after.Results[0].Values[0].Value) != wantResult {
		t.Fatalf("mutable alias changed committed generation: %#v", after)
	}
}

func TestInMemoryTwinTransitionRollsBackBeforeGenerationAndCanRetry(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-INTERRUPTED-TWIN"), time.Now().UTC())
	invalidState, _ := cloneWorkbenchValue(state)
	projection := TwinStateProjection{State: invalidState, Lineage: lineage, LineagePresent: true, RedpandaTopic: "interrupted", RedpandaOffset: 1}
	projection.State.Entities[0].Values[0].Value = map[string]any{"invalid": func() {}}
	if _, err := store.SaveTwinStateProjection("interrupted-test", projection); err == nil {
		t.Fatal("expected interrupted Twin transition to fail")
	}
	afterFailure, _ := store.Snapshot()
	if afterFailure.Generation != 0 || len(afterFailure.Lineage) != 0 || afterFailure.Twin.SchemaVersion != "" {
		t.Fatalf("failed transition became visible: %#v", afterFailure)
	}
	projection.State = state
	if written, err := store.SaveTwinStateProjection("interrupted-test", projection); err != nil || !written {
		t.Fatalf("retry interrupted Twin transition written=%v err=%v", written, err)
	}
	afterRecovery, _ := store.Snapshot()
	if afterRecovery.Generation != 1 || afterRecovery.Twin.SchemaVersion == "" || len(afterRecovery.Lineage) != len(lineage) {
		t.Fatalf("retry did not publish one complete transition: %#v", afterRecovery)
	}
	projection.State = invalidState
	if written, err := store.SaveTwinStateProjection("interrupted-test", projection); err != nil || written {
		t.Fatalf("invalid replay should be ignored after commit written=%v err=%v", written, err)
	}
}

func TestInMemoryWorkbenchSnapshotDoesNotMixConcurrentReadMoments(t *testing.T) {
	store := NewInMemoryWorkbenchStore()
	const writes = 200
	done := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		defer close(done)
		for sequence := 1; sequence <= writes; sequence++ {
			frame := scadaFrameFixture()
			frame.Sequence = uint64(sequence)
			frame.ObservedAt = frame.ObservedAt.Add(time.Duration(sequence) * time.Millisecond)
			raw, _ := json.Marshal(frame)
			projection, _ := ProjectScadaFrame("concurrent-scada", 0, int64(sequence), raw)
			if written, err := store.SaveScadaProjection("snapshot-reader-proof", projection); err != nil || !written {
				t.Errorf("save sequence %d written=%v err=%v", sequence, written, err)
				return
			}
		}
	}()

	lastGeneration := uint64(0)
	for {
		snapshot, err := store.Snapshot()
		if err != nil {
			t.Fatalf("read concurrent Snapshot: %v", err)
		}
		if snapshot.Generation < lastGeneration || snapshot.State.SnapshotGeneration != snapshot.Generation {
			t.Fatalf("generation moved backward or split: last=%d snapshot=%#v", lastGeneration, snapshot)
		}
		if got := snapshot.State.ValueBasisSummary[WorkbenchValueMeasured]; got != len(snapshot.Measured) {
			t.Fatalf("state summary and measured payload came from different moments: summary=%d frames=%d generation=%d", got, len(snapshot.Measured), snapshot.Generation)
		}
		lastGeneration = snapshot.Generation
		select {
		case <-done:
			writer.Wait()
			final, _ := store.Snapshot()
			if final.Generation != writes || len(final.Measured) != 1 || final.Measured[0].Sequence != writes {
				t.Fatalf("unexpected final Snapshot: %#v", final)
			}
			return
		default:
		}
	}
}
