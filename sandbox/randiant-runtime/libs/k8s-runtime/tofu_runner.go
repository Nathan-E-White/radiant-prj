package k8sruntime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type OpenTofuRunner interface {
	Init(ctx context.Context, vars map[string]string) error
	Plan(ctx context.Context, vars map[string]string) (string, error)
	Apply(ctx context.Context, vars map[string]string) error
	Destroy(ctx context.Context, vars map[string]string) error
}

type ExecTofuRunner struct {
	Binary         string
	Dir            string
	Stdout         io.Writer
	KubeConfigPath string
}

func NewExecTofuRunner(binary, dir string, stdout io.Writer) *ExecTofuRunner {
	if binary == "" {
		binary = "tofu"
	}
	if stdout == nil {
		stdout = io.Discard
	}
	return &ExecTofuRunner{Binary: binary, Dir: dir, Stdout: stdout}
}

func (r *ExecTofuRunner) Init(ctx context.Context, _ map[string]string) error {
	_, err := r.run(ctx, []string{"init", "-input=false"}, nil)
	return err
}

func (r *ExecTofuRunner) Plan(ctx context.Context, vars map[string]string) (string, error) {
	return r.run(ctx, []string{"plan", "-input=false", "-no-color"}, vars)
}

func (r *ExecTofuRunner) Apply(ctx context.Context, vars map[string]string) error {
	_, err := r.run(ctx, []string{"apply", "-input=false", "-auto-approve"}, vars)
	return err
}

func (r *ExecTofuRunner) Destroy(ctx context.Context, vars map[string]string) error {
	_, err := r.run(ctx, []string{"destroy", "-input=false", "-auto-approve"}, vars)
	return err
}

func (r *ExecTofuRunner) run(ctx context.Context, args []string, vars map[string]string) (string, error) {
	command := append([]string{}, args...)
	command = append(command, buildVarFlags(vars)...)
	cmd := exec.CommandContext(ctx, r.Binary, command...)
	cmd.Dir = r.Dir

	env := os.Environ()
	if r.KubeConfigPath != "" {
		env = append(env, "KUBECONFIG="+r.KubeConfigPath)
	}
	cmd.Env = env

	var output bytes.Buffer
	writer := io.MultiWriter(r.Stdout, &output)
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()
	return strings.TrimSpace(output.String()), err
}

func buildVarFlags(vars map[string]string) []string {
	if len(vars) == 0 {
		return nil
	}

	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(vars)*2)
	for _, key := range keys {
		args = append(args, "-var", fmt.Sprintf("%s=%s", key, vars[key]))
	}
	return args
}
