package portalsetup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	frontendUnit = "xdg-desktop-portal.service"
	stopWait     = 3 * time.Second
	stopTick     = 50 * time.Millisecond
)

func FrontendUnitKnown() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// Without systemd the bus starts the portal again on its next call.
func RestartFrontend(pid int) error {
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return stopFrontend(pid)
	}
	out, err := exec.Command(systemctl, "--user", "restart", frontendUnit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s: %w: %s", frontendUnit, err, out)
	}
	return nil
}

func stopFrontend(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = proc.Signal(syscall.SIGTERM)
	switch {
	case errors.Is(err, os.ErrProcessDone):
		return nil
	case err != nil:
		return fmt.Errorf("stop xdg-desktop-portal (pid %d): %w", pid, err)
	}
	deadline := time.Now().Add(stopWait)
	for time.Now().Before(deadline) {
		if errors.Is(proc.Signal(syscall.Signal(0)), os.ErrProcessDone) {
			return nil
		}
		time.Sleep(stopTick)
	}
	return fmt.Errorf("xdg-desktop-portal (pid %d) did not stop", pid)
}
