package wallpaper

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
)

const (
	ReasonNoDMS      = "NODMS"
	perMonitorErrSub = "Per-monitor mode enabled"
	probeTimeout     = 2 * time.Second
	callTimeout      = 10 * time.Second
)

type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type osRunner struct{}

func (osRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (osRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

type Service struct {
	sandboxed bool
	hostSpawn bool
	dmsBin    string
	runner    Runner
}

func New(caps sandbox.Capabilities) *Service {
	bin := os.Getenv("DMS_EXECUTABLE")
	if bin == "" {
		bin = "dms"
	}
	return NewWithRunner(caps, bin, osRunner{})
}

func NewWithRunner(caps sandbox.Capabilities, dmsBin string, runner Runner) *Service {
	if dmsBin == "" {
		dmsBin = "dms"
	}
	if runner == nil {
		runner = osRunner{}
	}
	return &Service{
		sandboxed: caps.Sandboxed,
		hostSpawn: caps.HostSpawn,
		dmsBin:    dmsBin,
		runner:    runner,
	}
}

func (s *Service) Feature(ctx context.Context) sandbox.Feature {
	if s.sandboxed && !s.hostSpawn {
		return sandbox.Feature{Reason: sandbox.ReasonNoHostSpawn}
	}
	if s.sandboxed {
		if _, err := s.runner.LookPath("flatpak-spawn"); err != nil {
			return sandbox.Feature{Reason: sandbox.ReasonNoHostSpawn}
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		defer cancel()
		if _, err := s.runner.Run(probeCtx, "flatpak-spawn", "--host", "sh", "-c", `command -v "$1" >/dev/null 2>&1`, "_", s.dmsBin); err != nil {
			return sandbox.Feature{Reason: ReasonNoDMS}
		}
		return sandbox.Feature{Available: true}
	}
	if _, err := s.runner.LookPath(s.dmsBin); err != nil {
		return sandbox.Feature{Reason: ReasonNoDMS}
	}
	return sandbox.Feature{Available: true}
}

func (s *Service) Set(ctx context.Context, path, screen string) error {
	if path == "" || !filepath.IsAbs(path) {
		return wire.Invalid("absolute path is required")
	}
	if s.sandboxed && !s.hostSpawn {
		return wire.Fail(files.CodeSandboxed, "flatpak-spawn not available inside the sandbox")
	}

	out, err := s.callDMS(ctx, "ipc", "call", "wallpaper", "set", path)
	if err == nil && !hasErrorOutput(out) {
		return nil
	}

	if screen != "" && strings.Contains(out, perMonitorErrSub) {
		out2, err2 := s.callDMS(ctx, "ipc", "call", "wallpaper", "setFor", screen, path)
		if err2 == nil && !hasErrorOutput(out2) {
			return nil
		}
		return formatCallError(out2, err2)
	}

	return formatCallError(out, err)
}

func (s *Service) callDMS(ctx context.Context, args ...string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	if !s.sandboxed {
		return s.runner.Run(callCtx, s.dmsBin, args...)
	}
	if _, err := s.runner.LookPath("flatpak-spawn"); err != nil {
		return "", wire.Fail(files.CodeSandboxed, "flatpak-spawn not available inside the sandbox")
	}
	hostArgs := make([]string, 0, len(args)+2)
	hostArgs = append(hostArgs, "--host", s.dmsBin)
	hostArgs = append(hostArgs, args...)
	return s.runner.Run(callCtx, "flatpak-spawn", hostArgs...)
}

func hasErrorOutput(out string) bool {
	return strings.HasPrefix(strings.TrimSpace(out), "ERROR:")
}

func formatCallError(out string, err error) error {
	var coded *files.Error
	if errors.As(err, &coded) {
		return coded
	}
	msg := strings.TrimSpace(out)
	msg = strings.TrimSpace(strings.TrimPrefix(msg, "ERROR:"))
	if msg != "" {
		return wire.Fail(files.CodeIO, "%s", msg)
	}
	if err != nil {
		return wire.Fail(files.CodeIO, "%v", err)
	}
	return wire.Fail(files.CodeIO, "failed to set wallpaper")
}
