package dbustest

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

const busConfig = `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:dir=%[1]s</listen>
  <servicedir>%[1]s/services</servicedir>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow receive_sender="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>`

func Bus(t *testing.T) string {
	address, _ := ActivatingBus(t)
	return address
}

// ActivatingBus also hands back the directory where a .service file makes a name activatable.
func ActivatingBus(t *testing.T) (address, serviceDir string) {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	switch {
	case err != nil && os.Getenv("CI") != "":
		t.Fatal("dbus-daemon not installed")
	case err != nil:
		t.Skip("dbus-daemon not installed")
	}
	dir := t.TempDir()
	serviceDir = filepath.Join(dir, "services")
	require.NoError(t, os.Mkdir(serviceDir, 0o700))
	config := filepath.Join(dir, "bus.conf")
	require.NoError(t, os.WriteFile(config, []byte(fmt.Sprintf(busConfig, dir)), 0o600))

	cmd := exec.Command(bin, "--config-file="+config, "--nofork", "--nopidfile", "--print-address")
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	return strings.TrimSpace(line), serviceDir
}

func Connect(t *testing.T, address string) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(address)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
