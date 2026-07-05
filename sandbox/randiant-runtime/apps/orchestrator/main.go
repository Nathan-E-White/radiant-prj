package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	containerruntime "github.com/radiant/container-runtime"
	k8sruntime "github.com/radiant/k8s-runtime"
	kaliruntime "github.com/radiant/surveillance-runtime"
)

const (
	defaultDockerMessage = "hello from docker container"
	defaultK8sMessage    = "hello from kubernetes"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "docker":
		if len(os.Args) < 3 || os.Args[2] != "run" {
			printUsage()
			return
		}
		if err := runDocker(os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "k8s":
		if len(os.Args) < 3 || os.Args[2] != "run" {
			printUsage()
			return
		}
		if err := runK8S(os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "monitor":
		if len(os.Args) < 3 || os.Args[2] != "run" {
			printUsage()
			return
		}
		if err := runMonitor(os.Args[3:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  orchestrator docker run --message <text> [--network <net>]")
	fmt.Println("  orchestrator k8s run --message <text> [--tofu-binary tofu] [--namespace default] [--cluster-name orchestrator-kind]")
	fmt.Println("  orchestrator monitor run --backend docker|k8s --mode passive|active|dual [--duration 30s] [--network <name>] [--targets <cidr/ip>] [--allow-external]")
}

func runDocker(args []string) error {
	flags := flag.NewFlagSet("docker run", flag.ExitOnError)
	message := flags.String("message", defaultDockerMessage, "message to echo from the toy alpine container")
	network := flags.String("network", "bridge", "docker network to run target container on")
	if err := flags.Parse(args); err != nil {
		return err
	}

	fmt.Println("status: preparing docker runtime")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dockerManager, err := containerruntime.NewDockerManagerFromEnv()
	if err != nil {
		return err
	}
	defer dockerManager.Close()

	targetSpec := containerruntime.NewToyEchoSpec(*message)
	targetSpec.NetworkMode = *network
	targetHandle, err := dockerManager.Start(ctx, targetSpec)
	if err != nil {
		return err
	}
	fmt.Println("status: container started", targetHandle.ID)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	waitErrCh := make(chan error, 1)
	go func() {
		_, err := dockerManager.Wait(ctx, targetHandle.ID)
		waitErrCh <- err
	}()

	logErrCh := make(chan error, 1)
	go func() {
		logErrCh <- dockerManager.StreamLogs(ctx, targetHandle.ID, os.Stdout)
	}()

	select {
	case sig := <-sigCh:
		fmt.Println("status: signal received", sig)
		cancel()
	case err := <-waitErrCh:
		if err != nil {
			fmt.Println("status: container wait ended with error", err)
		}
	case err := <-logErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	cleanupContainer(ctx, dockerManager, targetHandle.ID)
	return nil
}

func runK8S(args []string) error {
	flags := flag.NewFlagSet("k8s run", flag.ExitOnError)
	message := flags.String("message", defaultK8sMessage, "message to echo from kubernetes")
	namespace := flags.String("namespace", "default", "kubernetes namespace to deploy into")
	cluster := flags.String("cluster-name", "orchestrator-kind", "name of the local kind cluster")
	tofuBinary := flags.String("tofu-binary", "tofu", "OpenTofu binary path")
	tofuDir := flags.String("tofu-dir", "infra/tofu/k8s", "OpenTofu working directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	absDir, err := filepath.Abs(*tofuDir)
	if err != nil {
		return err
	}

	dockerManager, err := containerruntime.NewDockerManagerFromEnv()
	if err != nil {
		return err
	}
	defer dockerManager.Close()

	runner := k8sruntime.NewExecTofuRunner(*tofuBinary, absDir, os.Stdout)
	k8sManager := k8sruntime.NewKubernetesManager(k8sruntime.KubernetesManagerConfig{
		Runner:       runner,
		Docker:       dockerManager,
		ClusterName:  *cluster,
		Namespace:    *namespace,
		ReadyTimeout: 120 * time.Second,
	})

	defer func() {
		destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer destroyCancel()
		if err := k8sManager.Destroy(destroyCtx); err != nil {
			fmt.Println("warning: cleanup failed:", err)
		}
	}()

	fmt.Println("status: ensure kind cluster")
	if err := k8sManager.EnsureKindCluster(ctx); err != nil {
		return err
	}

	fmt.Println("status: apply terraform")
	if err := k8sManager.Apply(ctx, map[string]string{"echo_message": *message}); err != nil {
		return err
	}

	fmt.Println("status: wait for pod")
	podName, err := k8sManager.WaitForReady(ctx, *namespace, "app=orchestrator-alpine")
	if err != nil {
		return err
	}
	fmt.Println("status: pod ready", podName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	logsCh := make(chan error, 1)
	go func() {
		logsCh <- k8sManager.StreamPodLogs(ctx, *namespace, podName, os.Stdout)
	}()

	select {
	case sig := <-sigCh:
		fmt.Println("status: signal received", sig)
		cancel()
	case err := <-logsCh:
		if err != nil {
			fmt.Println("status: logs stream ended with error", err)
		}
	}

	fmt.Println("status: draining")
	return nil
}

func runMonitor(args []string) error {
	flags := flag.NewFlagSet("monitor run", flag.ExitOnError)
	backend := flags.String("backend", "docker", "execution backend for the target container flow")
	mode := flags.String("mode", string(kaliruntime.SurveillanceModeDual), "observer mode: passive, active, or dual")
	duration := flags.String("duration", "30s", "observation duration, e.g. 30s, 2m")
	network := flags.String("network", "", "docker/kind network to attach observer into")
	targetsRaw := flags.String("targets", "", "comma-separated target IPs/CIDRs for active probing")
	allowExternal := flags.Bool("allow-external", false, "allow external targets for active scans")
	pcapFilter := flags.String("pcap-filter", "", "tcpdump filter expression")
	nmapArgsRaw := flags.String("nmap-args", "", "comma-separated nmap args, e.g. \"-T4,-sV\"")
	outputFormat := flags.String("output-format", "text", "nmap output format: text, json, grepable")
	runAsRoot := flags.Bool("run-as-root", false, "run kali observer as root")
	kaliImage := flags.String("kali-image", "", "override default kali observer image")
	targetMessage := flags.String("target-message", defaultDockerMessage, "message from the target container")
	namespace := flags.String("namespace", "default", "kubernetes namespace for k8s backend")
	cluster := flags.String("cluster-name", "orchestrator-kind", "kind cluster name for k8s backend")
	tofuBinary := flags.String("tofu-binary", "tofu", "OpenTofu binary path")
	tofuDir := flags.String("tofu-dir", "infra/tofu/k8s", "OpenTofu working directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	parsedMode, err := parseSurveillanceMode(*mode)
	if err != nil {
		return err
	}
	parsedDuration, err := time.ParseDuration(strings.TrimSpace(*duration))
	if err != nil {
		return err
	}
	parsedTargets := splitCommaValues(*targetsRaw)
	parsedNmapArgs := splitCommaValues(*nmapArgsRaw)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch strings.TrimSpace(strings.ToLower(*backend)) {
	case "docker":
		return runMonitorDocker(ctx, runMonitorDockerOptions{
			Message:         *targetMessage,
			Mode:            parsedMode,
			Duration:        parsedDuration,
			Network:         strings.TrimSpace(*network),
			Targets:         parsedTargets,
			PCAPFilter:      *pcapFilter,
			NMapArgs:        parsedNmapArgs,
			AllowExternal:   *allowExternal,
			OutputFormat:    *outputFormat,
			RunAsRoot:       *runAsRoot,
			KaliImage:       *kaliImage,
		})
	case "k8s":
		return runMonitorK8S(ctx, runMonitorK8SOptions{
			Message:         *targetMessage,
			Mode:            parsedMode,
			Duration:        parsedDuration,
			Network:         strings.TrimSpace(*network),
			Targets:         parsedTargets,
			PCAPFilter:      *pcapFilter,
			NMapArgs:        parsedNmapArgs,
			AllowExternal:   *allowExternal,
			OutputFormat:    *outputFormat,
			RunAsRoot:       *runAsRoot,
			KaliImage:       *kaliImage,
			Namespace:       *namespace,
			Cluster:         *cluster,
			TofuBinary:      *tofuBinary,
			TofuDir:         *tofuDir,
		})
	default:
		return fmt.Errorf("unsupported backend %q", *backend)
	}
}

func runMonitorDocker(ctx context.Context, options runMonitorDockerOptions) error {
	dockerManager, err := containerruntime.NewDockerManagerFromEnv()
	if err != nil {
		return err
	}
	defer dockerManager.Close()

	targetSpec := containerruntime.NewToyEchoSpec(options.Message)
	targetSpec.NetworkMode = options.Network
	targetSpec.Name = "orchestrator-target"
	targetHandle, err := dockerManager.Start(ctx, targetSpec)
	if err != nil {
		return err
	}
	fmt.Println("status: target started", targetHandle.ID)

	observerTargets := append([]string{}, options.Targets...)
	if options.Mode != kaliruntime.SurveillanceModePassive && len(observerTargets) == 0 {
		if inferred, inferErr := inferDockerSubnet(ctx, options.Network); inferErr == nil && inferred != "" {
			observerTargets = append(observerTargets, inferred)
		}
	}

	observer := kaliruntime.NewKaliObserver(dockerManager)
	observerSpec := kaliruntime.KaliObserverSpec{
		Mode:                options.Mode,
		Image:               options.KaliImage,
		Network:             options.Network,
		Interface:           "any",
		Duration:            options.Duration,
		Targets:             observerTargets,
		PCAPFilter:          options.PCAPFilter,
		NMapArgs:            options.NMapArgs,
		OutputFormat:        options.OutputFormat,
		RunAsRootIfNeeded:   options.RunAsRoot,
		AllowExternalTarget: options.AllowExternal,
	}

	observerHandle, err := observer.Start(ctx, observerSpec)
	if err != nil {
		return err
	}
	fmt.Println("status: kali observer started", observerHandle.ContainerID)

	observerState := make(chan error, 1)
	go func() {
		observerState <- observer.StreamTelemetry(ctx, observerHandle.ContainerID, os.Stdout)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	waitState := make(chan error, 1)
	go func() {
		_, err := observer.Wait(ctx, observerHandle.ContainerID)
		waitState <- err
	}()

	select {
	case sig := <-sigCh:
		fmt.Println("status: signal received", sig)
		cancel()
	case err := <-observerState:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case err := <-waitState:
		if err != nil {
			fmt.Println("status: observer wait returned:", err)
		}
	}

	cleanupMonitor(ctx, observer, observerHandle.ContainerID, dockerManager, targetHandle.ID)
	return nil
}

type runMonitorDockerOptions struct {
	Message       string
	Mode          kaliruntime.SurveillanceMode
	Duration      time.Duration
	Network       string
	Targets       []string
	PCAPFilter    string
	NMapArgs      []string
	AllowExternal bool
	OutputFormat  string
	RunAsRoot     bool
	KaliImage     string
}

type runMonitorK8SOptions struct {
	Message       string
	Mode          kaliruntime.SurveillanceMode
	Duration      time.Duration
	Network       string
	Targets       []string
	PCAPFilter    string
	NMapArgs      []string
	AllowExternal bool
	OutputFormat  string
	RunAsRoot     bool
	KaliImage     string
	Namespace     string
	Cluster       string
	TofuBinary    string
	TofuDir       string
}

func runMonitorK8S(ctx context.Context, options runMonitorK8SOptions) error {
	absDir, err := filepath.Abs(options.TofuDir)
	if err != nil {
		return err
	}

	dockerManager, err := containerruntime.NewDockerManagerFromEnv()
	if err != nil {
		return err
	}
	defer dockerManager.Close()

	runner := k8sruntime.NewExecTofuRunner(options.TofuBinary, absDir, os.Stdout)
	k8sManager := k8sruntime.NewKubernetesManager(k8sruntime.KubernetesManagerConfig{
		Runner:       runner,
		Docker:       dockerManager,
		ClusterName:  options.Cluster,
		Namespace:    options.Namespace,
		ReadyTimeout: 120 * time.Second,
	})

	defer func() {
		destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer destroyCancel()
		if err := k8sManager.Destroy(destroyCtx); err != nil {
			fmt.Println("warning: k8s cleanup failed:", err)
		}
	}()

	if err := k8sManager.EnsureKindCluster(ctx); err != nil {
		return err
	}
	if err := k8sManager.Apply(ctx, map[string]string{"echo_message": options.Message}); err != nil {
		return err
	}
	podName, err := k8sManager.WaitForReady(ctx, options.Namespace, "app=orchestrator-alpine")
	if err != nil {
		return err
	}

	observerTargets := append([]string{}, options.Targets...)
	if options.Mode != kaliruntime.SurveillanceModePassive && len(observerTargets) == 0 {
		podCIDRs, err := k8sManager.GetPodCIDRs(ctx, options.Namespace, "app=orchestrator-alpine")
		if err != nil {
			return err
		}
		observerTargets = append(observerTargets, podCIDRs...)
	}

	network := options.Network
	if network == "" {
		network, err = k8sManager.DerivedKindNetwork(ctx)
		if err != nil {
			return err
		}
	}

	observer := kaliruntime.NewKaliObserver(dockerManager)
	observerSpec := kaliruntime.KaliObserverSpec{
		Mode:                options.Mode,
		Image:               options.KaliImage,
		Network:             network,
		Interface:           "eth0",
		Duration:            options.Duration,
		Targets:             observerTargets,
		PCAPFilter:          options.PCAPFilter,
		NMapArgs:            options.NMapArgs,
		OutputFormat:        options.OutputFormat,
		RunAsRootIfNeeded:   options.RunAsRoot,
		AllowExternalTarget: options.AllowExternal,
	}

	observerHandle, err := observer.Start(ctx, observerSpec)
	if err != nil {
		return err
	}
	fmt.Println("status: kali observer started", observerHandle.ContainerID, "for pod", podName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	telemetryCh := make(chan error, 1)
	go func() {
		telemetryCh <- observer.StreamTelemetry(ctx, observerHandle.ContainerID, os.Stdout)
	}()

	observerWait := make(chan error, 1)
	go func() {
		_, err := observer.Wait(ctx, observerHandle.ContainerID)
		observerWait <- err
	}()

	select {
	case sig := <-sigCh:
		fmt.Println("status: signal received", sig)
		cancel()
	case err := <-telemetryCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case err := <-observerWait:
		if err != nil {
			fmt.Println("status: observer wait returned:", err)
		}
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cleanupCancel()
	cleanupMonitor(cleanupCtx, observer, observerHandle.ContainerID, dockerManager, "")
	return nil
}

func parseSurveillanceMode(value string) (kaliruntime.SurveillanceMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(kaliruntime.SurveillanceModePassive):
		return kaliruntime.SurveillanceModePassive, nil
	case string(kaliruntime.SurveillanceModeActive):
		return kaliruntime.SurveillanceModeActive, nil
	case string(kaliruntime.SurveillanceModeDual), "":
		return kaliruntime.SurveillanceModeDual, nil
	default:
		return "", fmt.Errorf("unsupported monitoring mode %q", value)
	}
}

type containerLifeCycler interface {
	Stop(context.Context, string, time.Duration) error
	Remove(context.Context, string, bool) error
}

func cleanupContainer(ctx context.Context, manager containerLifeCycler, containerID string) {
	if strings.TrimSpace(containerID) == "" {
		return
	}
	// Always use an independent cleanup context; we don't want shutdown path to
	// be blocked by a canceled or timed-out parent context.
	cleanupCtx := context.Background()
	if ctx != nil && ctx.Err() == nil {
		cleanupCtx = ctx
	}
	stopCtx, stopCancel := context.WithTimeout(cleanupCtx, 5*time.Second)
	defer stopCancel()
	_ = manager.Stop(stopCtx, containerID, 5*time.Second)
	_ = manager.Remove(stopCtx, containerID, true)
}

func cleanupMonitor(ctx context.Context, observer interface {
	Stop(context.Context, string, time.Duration) error
	Remove(context.Context, string, bool) error
}, observerID string, dockerManager containerruntime.ContainerManager, targetID string) {
	cleanupContainer(ctx, observer, observerID)
	if targetID != "" {
		cleanupContainer(ctx, dockerManager, targetID)
	}
}

func inferDockerSubnet(ctx context.Context, network string) (string, error) {
	if strings.TrimSpace(network) == "" {
		network = "bridge"
	}
	output, err := runCommand(ctx, "network", "inspect", network, "--format", "{{(index .IPAM.Config 0).Subnet}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func splitCommaValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}
		values = append(values, clean)
	}
	return values
}

func runCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), err
	}
	return strings.TrimSpace(string(output)), nil
}
