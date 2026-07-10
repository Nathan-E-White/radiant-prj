package gateway

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseSbatchJobID(t *testing.T) {
	cases := map[string]string{
		"12345\n":                    "12345",
		"12345;cluster-a\n":          "12345",
		"Submitted batch job 67890":  "67890",
		" Submitted batch job 1357 ": "1357",
	}

	for input, want := range cases {
		got, err := parseSbatchJobID(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("parse %q: got %q want %q", input, got, want)
		}
	}
}

func TestParseSbatchJobIDRejectsBadOutput(t *testing.T) {
	for _, input := range []string{"", "not-a-job", "Submitted batch job nope"} {
		if _, err := parseSbatchJobID(input); err == nil {
			t.Fatalf("expected parse failure for %q", input)
		}
	}
}

func TestSbatchSpoolerBuildsBoundedArgsAndParsesOutput(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "transport-toy.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var gotName string
	var gotArgs []string
	spooler := SbatchSpooler{
		Command:        "sbatch",
		ScriptRoot:     root,
		AllowedScripts: csvSet("transport-toy"),
		Runner: runnerFunc(func(ctx context.Context, name string, args []string) (string, string, error) {
			gotName = name
			gotArgs = append([]string{}, args...)
			return "12345;cluster-a\n", "", nil
		}),
	}

	result, err := spooler.Submit(context.Background(), SubmitRequest{
		ScriptName: "transport-toy",
		Partition:  "transport",
		NodeCount:  2,
		RankCount:  8,
	}, "react-backend-client")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.JobID != "12345" {
		t.Fatalf("got job id %q", result.JobID)
	}
	if gotName != "sbatch" {
		t.Fatalf("got command %q", gotName)
	}

	wantArgs := []string{"--parsable", "--nodes=2", "--ntasks=8", "--partition=transport", script}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("got args %#v want %#v", gotArgs, wantArgs)
	}
}

func TestSbatchSpoolerRejectsUnsafeOrUnknownScripts(t *testing.T) {
	root := t.TempDir()
	spooler := SbatchSpooler{
		Command:        "sbatch",
		ScriptRoot:     root,
		AllowedScripts: csvSet("transport-toy"),
		Runner: runnerFunc(func(ctx context.Context, name string, args []string) (string, string, error) {
			t.Fatalf("runner should not be called for rejected script %q", args)
			return "", "", nil
		}),
	}

	for _, scriptName := range []string{
		"../outside",
		"/absolute/path",
		"nested/script",
		`nested\script`,
		"thermal-margin",
		"",
		"   ",
	} {
		t.Run(scriptName, func(t *testing.T) {
			_, err := spooler.Submit(context.Background(), SubmitRequest{
				ScriptName: scriptName,
				Partition:  "transport",
				NodeCount:  2,
				RankCount:  8,
			}, "react-backend-client")
			if err == nil {
				t.Fatalf("expected %q to be rejected", scriptName)
			}
		})
	}
}

func TestSbatchSpoolerReportsTimeout(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "transport-toy.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	spooler := SbatchSpooler{
		Command:        "sbatch",
		ScriptRoot:     root,
		AllowedScripts: csvSet("transport-toy"),
		Runner: runnerFunc(func(ctx context.Context, name string, args []string) (string, string, error) {
			<-ctx.Done()
			return "", "", ctx.Err()
		}),
	}

	_, err := spooler.Submit(ctx, SubmitRequest{
		ScriptName: "transport-toy",
		Partition:  "transport",
		NodeCount:  2,
		RankCount:  8,
	}, "react-backend-client")
	if err == nil {
		t.Fatalf("expected timeout")
	}
	if !strings.Contains(err.Error(), "timed out") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

type runnerFunc func(ctx context.Context, name string, args []string) (string, string, error)

func (f runnerFunc) Run(ctx context.Context, name string, args []string) (string, string, error) {
	return f(ctx, name, args)
}
