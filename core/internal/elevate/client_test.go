package elevate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-go/elevate"
	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
	"github.com/duxlabs-dev/dux-go/files"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/helper"
)

const fakeEnv = "DFILES_ELEVATE_FAKE_HELPER"

// TestMain lets the test binary stand in for pkexec: invoked as
// "<binary> <exe> helper <op> ..." it runs the helper operation unprivileged.
func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == "" {
		os.Exit(m.Run())
	}
	args := os.Args[1:]
	if len(args) < 2 || args[1] != "helper" {
		fmt.Fprintln(os.Stderr, "bad fake invocation")
		os.Exit(127)
	}
	os.Exit(dhelper.Finish(os.Stderr, nil, helper.Run(args[2:], os.Stdin, os.Stdout)))
}

func fakeClient(t *testing.T) *Client {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	return &Client{helper: &elevate.Client{Pkexec: self, Exe: "dfiles", Env: []string{fakeEnv + "=1"}}}
}

func TestClientSendsPermissionBitsOnly(t *testing.T) {
	root := t.TempDir()
	client := fakeClient(t)

	dir := filepath.Join(root, "shared")
	require.NoError(t, client.Mkdir(dir, 0o2775))
	require.NoError(t, client.Chmod(dir, 0o755|os.ModeSetgid|os.ModeSticky, false))
	info, err := os.Lstat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	assert.Zero(t, info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky))
	assert.Equal(t, "755", octal(0o755|os.ModeSetuid))
}

func TestClientRoundTripsThroughTheHelper(t *testing.T) {
	root := t.TempDir()
	client := fakeClient(t)

	require.NoError(t, client.MkdirAll(filepath.Join(root, "a", "b"), 0o755))
	require.NoError(t, client.Mkdir(filepath.Join(root, "a", "c"), 0o700))

	out, err := client.Create(filepath.Join(root, "a", "f.txt"), 0o640)
	require.NoError(t, err)
	_, err = out.Write([]byte("hello "))
	require.NoError(t, err)
	_, err = out.Write(bytes.Repeat([]byte("w"), 100000))
	require.NoError(t, err)
	require.NoError(t, out.Close())
	require.NoError(t, out.Close(), "closing twice is harmless")

	info, err := client.Lstat(filepath.Join(root, "a", "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "f.txt", info.Name())
	assert.Equal(t, int64(100006), info.Size())
	assert.True(t, info.Mode().IsRegular())
	assert.False(t, info.ModTime().IsZero())

	in, err := client.Open(filepath.Join(root, "a", "f.txt"))
	require.NoError(t, err)
	body, err := io.ReadAll(in)
	require.NoError(t, err)
	require.NoError(t, in.Close())
	assert.Len(t, body, 100006)
	assert.Equal(t, "hello ", string(body[:6]))

	require.NoError(t, client.Symlink("f.txt", filepath.Join(root, "a", "l")))
	target, err := client.Readlink(filepath.Join(root, "a", "l"))
	require.NoError(t, err)
	assert.Equal(t, "f.txt", target)

	entries, err := client.ReadDir(filepath.Join(root, "a"))
	require.NoError(t, err)
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = entry.IsDir()
	}
	assert.Equal(t, map[string]bool{"b": true, "c": true, "f.txt": false, "l": false}, names)

	require.NoError(t, client.Rename(filepath.Join(root, "a", "f.txt"), filepath.Join(root, "a", "g.txt"), true))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "h.txt"), []byte("h"), 0o644))
	err = client.Rename(filepath.Join(root, "a", "g.txt"), filepath.Join(root, "a", "h.txt"), true)
	assert.Equal(t, files.CodeExists, files.CodeOf(err), "noreplace refuses an existing target")
	require.NoError(t, client.Rename(filepath.Join(root, "a", "g.txt"), filepath.Join(root, "a", "h.txt"), false))
	stamp := time.Unix(1700000000, 0)
	require.NoError(t, client.Chtimes(filepath.Join(root, "a", "h.txt"), stamp))
	stamped, err := os.Lstat(filepath.Join(root, "a", "h.txt"))
	require.NoError(t, err)
	assert.True(t, stamped.ModTime().Equal(stamp))
	require.NoError(t, client.Chmod(filepath.Join(root, "a"), 0o700, true))
	require.NoError(t, client.Lchown(filepath.Join(root, "a", "h.txt"), -1, -1, false))
	require.NoError(t, client.Remove(filepath.Join(root, "a", "l")))
	require.NoError(t, client.RemoveAll(filepath.Join(root, "a")))
	_, err = os.Lstat(filepath.Join(root, "a"))
	assert.True(t, os.IsNotExist(err))
}

func TestClientSurfacesHelperErrors(t *testing.T) {
	root := t.TempDir()
	client := fakeClient(t)

	_, err := client.Lstat(filepath.Join(root, "missing"))
	assert.Equal(t, files.CodeNotFound, files.CodeOf(err))

	_, err = client.Open(filepath.Join(root, "missing"))
	assert.Equal(t, files.CodeNotFound, files.CodeOf(err), "a refused open is reported by Open itself")

	empty := filepath.Join(root, "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	in, err := client.Open(empty)
	require.NoError(t, err)
	body, err := io.ReadAll(in)
	require.NoError(t, err)
	assert.Empty(t, body)
	require.NoError(t, in.Close())

	existing := filepath.Join(root, "exists")
	require.NoError(t, os.WriteFile(existing, []byte("old"), 0o644))
	_, err = client.Create(existing, 0o644)
	assert.Equal(t, files.CodeExists, files.CodeOf(err), "a refused create is reported by Create itself")
	body, _ = os.ReadFile(existing)
	assert.Equal(t, "old", string(body))

	aborted := filepath.Join(root, "aborted")
	out, err := client.Create(aborted, 0o644)
	require.NoError(t, err)
	_, err = out.Write([]byte("partial"))
	require.NoError(t, err)
	require.NoError(t, out.Abort())
	_, statErr := os.Lstat(aborted)
	assert.True(t, os.IsNotExist(statErr), "an aborted write leaves nothing behind")

	err = client.Mkdir("relative/dir", 0o755)
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
}

func TestClientCanonicalisesSymlinkedParents(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(real, "f"), []byte("f"), 0o644))
	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(real, link))
	client := fakeClient(t)

	info, err := client.Lstat(filepath.Join(link, "f"))
	require.NoError(t, err)
	assert.Equal(t, "f", info.Name())

	require.NoError(t, client.Mkdir(filepath.Join(link, "sub"), 0o755))
	_, err = os.Lstat(filepath.Join(real, "sub"))
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(real, "f"), canonical(filepath.Join(link, "f")))
	assert.Equal(t, "/nonexistent/x/y", canonical("/nonexistent/x/y"), "an unresolvable parent is passed through")
}

func TestAvailableExplainsWhatIsMissing(t *testing.T) {
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("FLATPAK_ID", "com.example.Sandbox")
	client := &Client{helper: &elevate.Client{Policy: PolicyName, Pkexec: "/bin/true", Exe: "/bin/true"}}
	assert.Equal(t, elevate.ReasonSandboxed, reason(t, client.Available()))

	t.Setenv("FLATPAK_ID", "")
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		t.Skip("running inside flatpak")
	}
	assert.Equal(t, elevate.ReasonNoPolicy, reason(t, client.Available()))

	missing := &Client{helper: &elevate.Client{Policy: PolicyName, Pkexec: "/nonexistent/pkexec", Exe: "/bin/true"}}
	assert.Equal(t, elevate.ReasonNoPkexec, reason(t, missing.Available()))
}

func reason(t *testing.T, err error) string {
	t.Helper()
	var unavailable *fileops.Unavailable
	require.ErrorAs(t, err, &unavailable)
	return unavailable.Reason
}
