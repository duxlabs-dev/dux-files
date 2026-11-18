package helper

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	keptCaps    = 1<<unix.CAP_CHOWN | 1<<unix.CAP_DAC_OVERRIDE | 1<<unix.CAP_DAC_READ_SEARCH | 1<<unix.CAP_FOWNER
	boundedEnv  = "DFILES_HELPER_BOUNDED"
	capSetWidth = 64
)

func confine() error {
	if os.Getenv(boundedEnv) == "" {
		return execBounded()
	}
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("dumpable: %w", err)
	}
	return verifyConfined()
}

func execBounded() error {
	runtime.LockOSThread()
	if err := dropBoundingSet(); err != nil {
		return fmt.Errorf("capbset: %w", err)
	}
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{{Effective: keptCaps, Permitted: keptCaps}}
	if err := unix.Capset(&header, &data[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %w", err)
	}
	err := syscall.Exec("/proc/self/exe", os.Args, append(os.Environ(), boundedEnv+"=1"))
	return fmt.Errorf("exec: %w", err)
}

func dropBoundingSet() error {
	for capability := range capSetWidth {
		if keptCaps&(1<<capability) != 0 {
			continue
		}
		err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(capability), 0, 0, 0)
		pastLastCap := errors.Is(err, unix.EINVAL)
		if pastLastCap {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyConfined() error {
	caps := fmt.Sprintf("%016x", uint64(keptCaps))
	want := map[string]string{
		"Uid":        "0\t0\t0\t0",
		"CapPrm":     caps,
		"CapEff":     caps,
		"CapBnd":     caps,
		"NoNewPrivs": "1",
	}
	threads, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return err
	}
	for _, thread := range threads {
		got, err := threadStatus(thread.Name())
		if err != nil {
			return err
		}
		for key, value := range want {
			if got[key] != value {
				return fmt.Errorf("privilege drop did not take effect on thread %s (%s %q, want %q)", thread.Name(), key, got[key], value)
			}
		}
	}
	return nil
}

func threadStatus(tid string) (map[string]string, error) {
	raw, err := os.ReadFile("/proc/self/task/" + tid + "/status")
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		status[key] = strings.TrimSpace(value)
	}
	return status, nil
}
