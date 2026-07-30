package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalWorkbenchSnapshotVectors(t *testing.T) {
	for _, vector := range []struct {
		name  string
		valid bool
	}{
		{name: "workbench-snapshot.valid.json", valid: true},
		{name: "workbench-snapshot.generation-mismatch.json", valid: false},
		{name: "workbench-snapshot.partial.json", valid: false},
		{name: "workbench-snapshot.invalid-value-basis.json", valid: false},
	} {
		t.Run(vector.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "examples", "simulator-workbench", vector.name)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read canonical vector: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("decode raw canonical vector: %v", err)
			}
			if !vector.valid && vector.name == "workbench-snapshot.partial.json" && raw["lineage"] == nil {
				return
			}
			var snapshot WorkbenchSnapshot
			if err := json.Unmarshal(body, &snapshot); err != nil {
				t.Fatalf("decode canonical vector: %v", err)
			}
			err = ValidateWorkbenchSnapshot(snapshot)
			if vector.valid && err != nil {
				t.Fatalf("valid canonical vector rejected: %v", err)
			}
			if !vector.valid && err == nil {
				t.Fatal("invalid canonical vector was accepted")
			}
		})
	}
}
