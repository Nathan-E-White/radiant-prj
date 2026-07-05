package surveilanceruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	containerruntime "github.com/radiant/container-runtime"
)

const (
	defaultKaliObservationDuration = 30 * time.Second
	kaliObserverImageEnvVar       = "RADIANT_KALI_OBSERVER_IMAGE"
	defaultKaliObserverImage      = "kali-observer:local"
	passiveModeLabel              = "[tcpdump]"
	activeModeLabel               = "[nmap]"
)

type SurveillanceMode string

const (
	SurveillanceModePassive SurveillanceMode = "passive"
	SurveillanceModeActive  SurveillanceMode = "active"
	SurveillanceModeDual    SurveillanceMode = "dual"
)

type KaliObserverSpec struct {
	Mode                SurveillanceMode
	Image               string
	Network             string
	Interface           string
	Duration            time.Duration
	Targets             []string
	PCAPFilter          string
	NMapArgs            []string
	OutputFormat        string
	RunAsRootIfNeeded   bool
	AllowExternalTarget bool
}

type ObserverHandle struct {
	ContainerID string
}

type ObservationState struct {
	ExitCode  int64
	Error     string
	OOMKilled bool
}

type KaliObserver interface {
	Start(ctx context.Context, spec KaliObserverSpec) (ObserverHandle, error)
	StreamTelemetry(ctx context.Context, id string, destination io.Writer) error
	Stop(ctx context.Context, id string, timeout time.Duration) error
	Remove(ctx context.Context, id string, force bool) error
	Wait(ctx context.Context, id string) (ObservationState, error)
}

type KaliObserverManager struct {
	docker containerruntime.ContainerManager
}

func NewKaliObserver(docker containerruntime.ContainerManager) *KaliObserverManager {
	return &KaliObserverManager{docker: docker}
}

func GetDefaultKaliImage() string {
	if image := strings.TrimSpace(os.Getenv(kaliObserverImageEnvVar)); image != "" {
		return image
	}
	return defaultKaliObserverImage
}

func (m *KaliObserverManager) Start(ctx context.Context, spec KaliObserverSpec) (ObserverHandle, error) {
	if m == nil || m.docker == nil {
		return ObserverHandle{}, errors.New("kali observer has no container runtime")
	}

	normalized, err := normalizeKaliSpec(spec)
	if err != nil {
		return ObserverHandle{}, err
	}

	command, err := buildObservationCommand(normalized)
	if err != nil {
		return ObserverHandle{}, err
	}

	containerSpec := containerruntime.ContainerSpec{
		Image:              normalized.Image,
		Cmd:                []string{"/bin/sh", "-c", command},
		NetworkMode:        normalized.Network,
		RemoveOnExit:       true,
		HealthProbeTimeout: 10 * time.Second,
		Labels: map[string]string{
			"project": "orchestrator",
			"type":    "kali-observer",
			"mode":    string(normalized.Mode),
		},
	}

	if normalized.Mode != SurveillanceModeActive {
		containerSpec.Capabilities = []string{"NET_RAW", "NET_ADMIN"}
		containerSpec.User = "root"
	}
	if normalized.RunAsRootIfNeeded && normalized.Mode == SurveillanceModeActive {
		containerSpec.User = "root"
	}

	handle, err := m.docker.Start(ctx, containerSpec)
	if err != nil {
		return ObserverHandle{}, err
	}
	return ObserverHandle{ContainerID: handle.ID}, nil
}

func (m *KaliObserverManager) StreamTelemetry(ctx context.Context, id string, destination io.Writer) error {
	if m == nil || m.docker == nil {
		return errors.New("kali observer has no container runtime")
	}
	return m.docker.StreamLogs(ctx, id, destination)
}

func (m *KaliObserverManager) Stop(ctx context.Context, id string, timeout time.Duration) error {
	if m == nil || m.docker == nil {
		return errors.New("kali observer has no container runtime")
	}
	return m.docker.Stop(ctx, id, timeout)
}

func (m *KaliObserverManager) Remove(ctx context.Context, id string, force bool) error {
	if m == nil || m.docker == nil {
		return errors.New("kali observer has no container runtime")
	}
	return m.docker.Remove(ctx, id, force)
}

func (m *KaliObserverManager) Wait(ctx context.Context, id string) (ObservationState, error) {
	if m == nil || m.docker == nil {
		return ObservationState{}, errors.New("kali observer has no container runtime")
	}
	state, err := m.docker.Wait(ctx, id)
	if err != nil {
		return ObservationState{}, err
	}
	return ObservationState{
		ExitCode:  state.ExitCode,
		Error:     state.Error,
		OOMKilled: state.OOMKilled,
	}, nil
}

func normalizeKaliSpec(spec KaliObserverSpec) (KaliObserverSpec, error) {
	normalized := spec
	if normalized.Mode == "" {
		normalized.Mode = SurveillanceModeDual
	}
	if normalized.Mode != SurveillanceModePassive &&
		normalized.Mode != SurveillanceModeActive &&
		normalized.Mode != SurveillanceModeDual {
		return KaliObserverSpec{}, fmt.Errorf("unknown surveillance mode %q", spec.Mode)
	}
	if strings.TrimSpace(normalized.Image) == "" {
		normalized.Image = GetDefaultKaliImage()
	}
	if strings.TrimSpace(normalized.Interface) == "" {
		normalized.Interface = "any"
	}
	if normalized.Duration <= 0 {
		normalized.Duration = defaultKaliObservationDuration
}
	if strings.TrimSpace(normalized.OutputFormat) == "" {
		normalized.OutputFormat = "text"
	}
	if strings.TrimSpace(normalized.Network) == "" {
		normalized.Network = "bridge"
	}

	filteredTargets := make([]string, 0, len(normalized.Targets))
	for _, target := range normalized.Targets {
		clean := strings.TrimSpace(target)
		if clean == "" {
			continue
		}
		if normalized.Mode != SurveillanceModePassive && !normalized.AllowExternalTarget {
			if err := ensureLocalTarget(clean); err != nil {
				return KaliObserverSpec{}, err
			}
		}
		filteredTargets = append(filteredTargets, clean)
	}
	normalized.Targets = filteredTargets

	if normalized.Mode == SurveillanceModeActive && len(normalized.Targets) == 0 {
		return KaliObserverSpec{}, errors.New("active mode requires at least one target")
	}
	if normalized.Mode == SurveillanceModeDual && len(normalized.Targets) == 0 {
		return KaliObserverSpec{}, errors.New("dual mode requires at least one target for active probe")
	}

	if len(normalized.NMapArgs) > 0 {
		normalized.NMapArgs = append([]string(nil), normalized.NMapArgs...)
	}

	return normalized, nil
}

func buildObservationCommand(spec KaliObserverSpec) (string, error) {
	switch spec.Mode {
	case SurveillanceModePassive:
		return buildPassiveCommand(spec), nil
	case SurveillanceModeActive:
		return buildActiveCommand(spec)
	case SurveillanceModeDual:
		passiveCmd := buildPassiveCommand(spec)
		activeCmd, err := buildActiveCommand(spec)
		if err != nil {
			return "", err
		}
		return passiveCmd + " &\n" + activeCmd + "\nwait", nil
	default:
		return "", fmt.Errorf("unsupported surveillance mode %q", spec.Mode)
	}
}

func buildPassiveCommand(spec KaliObserverSpec) string {
	timeoutPrefix := formatTimeout(spec.Duration)
	filterArg := ""
	if strings.TrimSpace(spec.PCAPFilter) != "" {
		filterArg = " " + shellQuote(strings.TrimSpace(spec.PCAPFilter))
	}
	core := fmt.Sprintf("%stcpdump -i %s -nn -l -v %s", timeoutPrefix, shellQuote(spec.Interface), filterArg)
	return prefixLogOutput(core, passiveModeLabel)
}

func buildActiveCommand(spec KaliObserverSpec) (string, error) {
	if len(spec.Targets) == 0 {
		return "", errors.New("active command requires at least one target")
	}
	args := make([]string, 0, len(spec.NMapArgs)+6+len(spec.Targets))
	args = append(args, "-Pn", "-sV")
	args = append(args, outputArg(spec.OutputFormat)...)
	args = append(args, spec.NMapArgs...)
	args = append(args, spec.Targets...)

	for i := range args {
		args[i] = shellQuote(args[i])
	}

	timeoutPrefix := formatTimeout(spec.Duration)
	core := fmt.Sprintf("%snmap %s", timeoutPrefix, strings.Join(args, " "))
	return prefixLogOutput(core, activeModeLabel), nil
}

func outputArg(format string) []string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return []string{"-oX", "-"}
	case "grepable":
		return []string{"-oG", "-"}
	default:
		return []string{"-oN", "-"}
	}
}

func formatTimeout(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}
	return "timeout " + duration.String() + " "
}

func prefixLogOutput(command, label string) string {
	return fmt.Sprintf("(%s) 2>&1 | sed -e 's/^/%s /'", command, label)
}

func ensureLocalTarget(target string) error {
	if target == "" {
		return errors.New("empty target")
	}
	if strings.EqualFold(target, "localhost") || target == "127.0.0.1" || target == "::1" {
		return nil
	}
	if target == "localhost.localdomain" {
		return errors.New("non-local hostnames are blocked by default")
	}

	if ip := net.ParseIP(target); ip != nil {
		if ip.IsLoopback() || isPrivateIPv4(ip.To4()) {
			return nil
		}
		return fmt.Errorf("target %q is external", target)
	}

	_, network, err := net.ParseCIDR(target)
	if err == nil {
		ip := network.IP.To4()
		if isPrivateIPv4(ip) {
			return nil
		}
		return fmt.Errorf("target %q is external", target)
	}

	return errors.New("unsupported target format; use IP, CIDR, or localhost")
}

func isPrivateIPv4(ip net.IP) bool {
	if ip == nil {
		return false
	}
	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	case ip[0] == 127:
		return true
	default:
		return false
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
