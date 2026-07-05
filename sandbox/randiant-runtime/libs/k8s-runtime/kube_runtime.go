package k8sruntime

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	containerruntime "github.com/radiant/container-runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeManager interface {
	EnsureKindCluster(ctx context.Context) error
	Apply(ctx context.Context, vars map[string]string) error
	WaitForReady(ctx context.Context, namespace, selector string) (string, error)
	StreamPodLogs(ctx context.Context, namespace, podName string, destination io.Writer) error
	DerivedKindNetwork(ctx context.Context) (string, error)
	GetPodCIDRs(ctx context.Context, namespace, selector string) ([]string, error)
	Destroy(ctx context.Context) error
}

type KubernetesManagerConfig struct {
	Runner       OpenTofuRunner
	Docker       containerruntime.ContainerManager
	ClusterName  string
	Namespace    string
	ReadyTimeout time.Duration
	PollInterval time.Duration
}

type KubernetesManager struct {
	runner       OpenTofuRunner
	docker       containerruntime.ContainerManager
	clusterName  string
	namespace    string
	readyTimeout time.Duration
	pollInterval time.Duration
	kubeConfig   string
	createdByMe  bool
	appliedVars  map[string]string
}

func NewKubernetesManager(cfg KubernetesManagerConfig) *KubernetesManager {
	clusterName := cfg.ClusterName
	if strings.TrimSpace(clusterName) == "" {
		clusterName = "orchestrator-kind"
	}
	namespace := cfg.Namespace
	if strings.TrimSpace(namespace) == "" {
		namespace = "default"
	}
	timeout := cfg.ReadyTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = 2 * time.Second
	}

	return &KubernetesManager{
		runner:       cfg.Runner,
		docker:       cfg.Docker,
		clusterName:  clusterName,
		namespace:    namespace,
		readyTimeout: timeout,
		pollInterval: poll,
	}
}

func FromDockerManager(docker containerruntime.ContainerManager, runner OpenTofuRunner) *KubernetesManager {
	return NewKubernetesManager(KubernetesManagerConfig{
		Docker:  docker,
		Runner:  runner,
	})
}

func (m *KubernetesManager) EnsureKindCluster(ctx context.Context) error {
	if m.docker != nil {
		handle, err := containerruntime.RunToyEchoContainer(ctx, m.docker, "k8s manager preflight")
		if err == nil {
			_, _ = m.docker.Wait(ctx, handle.ID)
			_ = m.docker.Remove(ctx, handle.ID, true)
		}
	}

	if _, err := runKindCommand(ctx, "version"); err != nil {
		return fmt.Errorf("kind binary check failed: %w", err)
	}

	clusters, err := runKindCommand(ctx, "get", "clusters")
	if err != nil {
		return fmt.Errorf("query kind clusters: %w", err)
	}

	names := map[string]struct{}{}
	for _, name := range strings.Fields(clusters) {
		names[name] = struct{}{}
	}
	if _, ok := names[m.clusterName]; !ok {
		if _, err := runKindCommand(ctx, "create", "cluster", "--name", m.clusterName, "--wait", "30s"); err != nil {
			return err
		}
		m.createdByMe = true
	}

	cfgPath, err := runKindCommand(ctx, "get", "kubeconfig", "--name", m.clusterName)
	if err != nil {
		return fmt.Errorf("resolve kubeconfig: %w", err)
	}
	if strings.TrimSpace(cfgPath) == "" {
		return fmt.Errorf("kind did not return a kubeconfig path")
	}

	m.kubeConfig = strings.TrimSpace(cfgPath)
	if tofu, ok := m.runner.(*ExecTofuRunner); ok {
		tofu.KubeConfigPath = m.kubeConfig
	}
	return nil
}

func (m *KubernetesManager) Apply(ctx context.Context, vars map[string]string) error {
	if m.runner == nil {
		return fmt.Errorf("tofu runner is not configured")
	}
	if m.kubeConfig == "" {
		return fmt.Errorf("kubeconfig is unknown, call EnsureKindCluster first")
	}

	if vars == nil {
		vars = map[string]string{}
	}
	if _, ok := vars["namespace"]; !ok {
		vars["namespace"] = m.namespace
	}
	applied := mergeVars(vars)
	applied["kubeconfig_path"] = m.kubeConfig

	if err := m.runner.Init(ctx, applied); err != nil {
		return err
	}
	if err := m.runner.Apply(ctx, applied); err != nil {
		return err
	}
	m.appliedVars = applied
	return nil
}

func (m *KubernetesManager) WaitForReady(ctx context.Context, namespace, selector string) (string, error) {
	if strings.TrimSpace(namespace) == "" {
		namespace = m.namespace
	}
	if strings.TrimSpace(selector) == "" {
		return "", fmt.Errorf("selector is required")
	}

	clientset, err := m.client()
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(m.readyTimeout)
	for {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return "", err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodFailed {
				return "", fmt.Errorf("pod %s failed", pod.Name)
			}
			if pod.Status.Phase == corev1.PodSucceeded {
				return pod.Name, nil
			}
			if pod.Status.Phase == corev1.PodRunning && isPodReady(&pod) {
				return pod.Name, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timeout waiting for selector %q in namespace %q", selector, namespace)
		}
		select {
		case <-time.After(m.pollInterval):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (m *KubernetesManager) StreamPodLogs(ctx context.Context, namespace, podName string, destination io.Writer) error {
	if strings.TrimSpace(namespace) == "" {
		namespace = m.namespace
	}
	if strings.TrimSpace(podName) == "" {
		return fmt.Errorf("pod name is required")
	}

	clientset, err := m.client()
	if err != nil {
		return err
	}

	logs := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
	stream, err := logs.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = io.Copy(destination, stream)
	return err
}

func (m *KubernetesManager) DerivedKindNetwork(ctx context.Context) (string, error) {
	networkName := "kind"
	if strings.TrimSpace(m.clusterName) == "" {
		networkName = "kind"
	}

	if _, err := runCommand(ctx, "docker", "network", "inspect", networkName); err == nil {
		return networkName, nil
	}

	return "", fmt.Errorf("unable to find kind docker network for cluster %q", m.clusterName)
}

func (m *KubernetesManager) GetPodCIDRs(ctx context.Context, namespace, selector string) ([]string, error) {
	if strings.TrimSpace(namespace) == "" {
		namespace = m.namespace
	}
	clientset, err := m.client()
	if err != nil {
		return nil, err
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	for _, pod := range pods.Items {
		podIP := strings.TrimSpace(pod.Status.PodIP)
		if podIP == "" {
			continue
		}
		parsed := net.ParseIP(podIP).To4()
		if parsed == nil {
			continue
		}
		cidr := fmt.Sprintf("%d.%d.%d.0/24", parsed[0], parsed[1], parsed[2])
		seen[cidr] = struct{}{}
	}

	if len(seen) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(seen))
	for cidr := range seen {
		result = append(result, cidr)
	}
	sort.Strings(result)
	return result, nil
}

func (m *KubernetesManager) Destroy(ctx context.Context) error {
	var result error
	vars := copyVars(m.appliedVars)
	if vars == nil {
		vars = map[string]string{}
	}
	if m.kubeConfig != "" {
		vars["kubeconfig_path"] = m.kubeConfig
	}
	if m.runner != nil {
		if err := m.runner.Destroy(ctx, vars); err != nil {
			result = err
		}
	}
	if m.createdByMe {
		if err := runKindCommand(ctx, "delete", "cluster", "--name", m.clusterName); err != nil && result == nil {
			result = err
		}
		m.createdByMe = false
	}
	return result
}

func (m *KubernetesManager) client() (*kubernetes.Clientset, error) {
	if strings.TrimSpace(m.kubeConfig) == "" {
		return nil, fmt.Errorf("kubeconfig is required, call EnsureKindCluster first")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", m.kubeConfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func runKindCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return strings.TrimSpace(string(out)), nil
}

func runCommand(ctx context.Context, binary string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return strings.TrimSpace(string(out)), nil
}

func mergeVars(vars map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range vars {
		result[k] = v
	}
	if _, ok := result["namespace"]; !ok {
		result["namespace"] = "default"
	}
	if _, ok := result["app_label"]; !ok {
		result["app_label"] = "orchestrator-alpine"
	}
	if _, ok := result["container_image"]; !ok {
		result["container_image"] = "alpine:latest"
	}
	if _, ok := result["pod_name"]; !ok {
		result["pod_name"] = "orchestrator-alpine-echo"
	}
	return result
}

func copyVars(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isPodReady(pod *corev1.Pod) bool {
	if pod == nil || len(pod.Status.ContainerStatuses) == 0 {
		return false
	}
	for _, st := range pod.Status.ContainerStatuses {
		if !st.Ready {
			return false
		}
	}
	return true
}
