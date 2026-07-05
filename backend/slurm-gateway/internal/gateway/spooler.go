package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SubmitResult struct {
	JobID   string
	State   JobState
	Message string
	Mode    string
}

type SlurmSpooler interface {
	Submit(ctx context.Context, req SubmitRequest, identity string) (SubmitResult, error)
}

type MockSpooler struct{}

func (s MockSpooler) Submit(ctx context.Context, req SubmitRequest, identity string) (SubmitResult, error) {
	select {
	case <-ctx.Done():
		return SubmitResult{}, ctx.Err()
	default:
	}

	hasher := fnv.New32a()
	_, _ = fmt.Fprintf(hasher, "%s|%s|%d|%d", req.ScriptName, req.Partition, req.NodeCount, req.RankCount)

	return SubmitResult{
		JobID:   fmt.Sprintf("MOCK-%08x", hasher.Sum32()),
		State:   StateQueued,
		Message: "Job queued by mock Slurm spooler",
		Mode:    string(ModeMock),
	}, nil
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args []string) (stdout string, stderr string, err error)
}

type RealCommandRunner struct{}

func (r RealCommandRunner) Run(ctx context.Context, name string, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type SbatchSpooler struct {
	Command    string
	ScriptRoot string
	Runner     CommandRunner
}

func (s SbatchSpooler) Submit(ctx context.Context, req SubmitRequest, identity string) (SubmitResult, error) {
	runner := s.Runner
	if runner == nil {
		runner = RealCommandRunner{}
	}

	scriptPath, err := safeScriptPath(s.ScriptRoot, req.ScriptName)
	if err != nil {
		return SubmitResult{}, err
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return SubmitResult{}, fmt.Errorf("script is not available under configured script root: %w", err)
	}

	args := []string{
		"--parsable",
		fmt.Sprintf("--nodes=%d", req.NodeCount),
		fmt.Sprintf("--ntasks=%d", req.RankCount),
		"--partition=" + req.Partition,
		scriptPath,
	}

	stdout, stderr, err := runner.Run(ctx, s.Command, args)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return SubmitResult{}, fmt.Errorf("sbatch timed out: %w", ctx.Err())
		}
		return SubmitResult{}, fmt.Errorf("sbatch failed: %w: %s", err, strings.TrimSpace(stderr))
	}

	jobID, err := parseSbatchJobID(stdout)
	if err != nil {
		return SubmitResult{}, err
	}

	return SubmitResult{
		JobID:   jobID,
		State:   StateQueued,
		Message: "Job queued by sbatch",
		Mode:    string(ModeSbatch),
	}, nil
}

func parseSbatchJobID(stdout string) (string, error) {
	output := strings.TrimSpace(stdout)
	if output == "" {
		return "", fmt.Errorf("sbatch returned an empty job id")
	}
	if strings.HasPrefix(output, "Submitted batch job ") {
		output = strings.TrimPrefix(output, "Submitted batch job ")
	}
	if index := strings.Index(output, ";"); index >= 0 {
		output = output[:index]
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return "", fmt.Errorf("sbatch returned an empty job id")
	}
	for _, ch := range output {
		if ch < '0' || ch > '9' {
			return "", fmt.Errorf("sbatch returned non-numeric job id %q", output)
		}
	}
	return output, nil
}

func safeScriptPath(root string, scriptName string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve script root: %w", err)
	}

	candidate, err := filepath.Abs(filepath.Join(cleanRoot, scriptName+".sh"))
	if err != nil {
		return "", fmt.Errorf("resolve script path: %w", err)
	}

	if candidate != cleanRoot && !strings.HasPrefix(candidate, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("script path escapes configured root")
	}

	return candidate, nil
}
