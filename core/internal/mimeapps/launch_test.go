package mimeapps

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func expand(t *testing.T, execLine string, entry *Entry, paths ...string) []string {
	t.Helper()
	tokens, err := splitExec(execLine)
	require.NoError(t, err)
	if entry == nil {
		entry = &Entry{Name: "Editor", Icon: "text-editor", Path: "/apps/editor.desktop"}
	}
	return expandExec(tokens, entry, paths)
}

func TestExecFieldCodes(t *testing.T) {
	assert.Equal(t, []string{"gedit", "/tmp/a"}, expand(t, "gedit %f", nil, "/tmp/a"))
	assert.Equal(t, []string{"gedit", "/tmp/a", "/tmp/b"}, expand(t, "gedit %F", nil, "/tmp/a", "/tmp/b"))
	assert.Equal(t, []string{"open", "file:///tmp/a"}, expand(t, "open %u", nil, "/tmp/a"))
	assert.Equal(t, []string{"open", "file:///tmp/a", "file:///tmp/b"}, expand(t, "open %U", nil, "/tmp/a", "/tmp/b"))
	assert.Equal(t, []string{"app", "--icon", "text-editor"}, expand(t, "app %i", nil))
	assert.Equal(t, []string{"app", "Editor", "/apps/editor.desktop"}, expand(t, "app %c %k", nil))
	assert.Equal(t, []string{"app"}, expand(t, "app %d %D %n %N %v %m", nil), "deprecated codes are dropped")
	assert.Equal(t, []string{"app", "/tmp/a"}, expand(t, "app", nil, "/tmp/a"), "without a field code the paths are appended")
	assert.Equal(t, []string{"app", "100%"}, expand(t, "app 100%%", nil))
	assert.Equal(t, []string{"app", "--file=/tmp/a"}, expand(t, "app --file=%f", nil, "/tmp/a"))
	assert.Equal(t, []string{"app"}, expand(t, "app %i", &Entry{}), "an entry with no icon drops the flag as well")
}

func TestExecQuoting(t *testing.T) {
	assert.Equal(t, []string{"sh", "-c", "echo hi", "/tmp/a"}, expand(t, `sh -c "echo hi" %f`, nil, "/tmp/a"))
	assert.Equal(t, []string{"app", `quo"ted`}, expand(t, `app "quo\"ted"`, nil))

	_, err := splitExec(`app "unterminated`)
	require.Error(t, err)
}

func TestSingularExecRunsOncePerPath(t *testing.T) {
	single, err := splitExec("gedit %f")
	require.NoError(t, err)
	assert.True(t, singularExec(single))

	plural, err := splitExec("gedit %F")
	require.NoError(t, err)
	assert.False(t, singularExec(plural))
}

func TestPathURI(t *testing.T) {
	assert.Equal(t, "file:///tmp/a%20b", PathURI("/tmp/a b"))
	assert.Equal(t, "https://example.invalid", PathURI("https://example.invalid"))
}

func TestLaunchWritesThroughTheChosenApp(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "launched")
	target := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	writeFile(t, filepath.Join(root, "xdg", "data", "applications", "recorder.desktop"), []byte(
		"[Desktop Entry]\nType=Application\nName=Recorder\nExec=sh -c \"echo \\\"$1\\\" > "+marker+"\" sh %f\nMimeType=text/plain;\n"))

	require.NoError(t, svc.OpenWith(context.Background(), "recorder.desktop", []string{target}, LaunchOptions{}))
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(data)) == target
	}, waitFor, waitTick, "the launched process received the file path")
}

func TestOpenUsesTheDefaultApp(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "opened")
	target := writeFile(t, filepath.Join(root, "files", "shot.png"), []byte("\x89PNG\r\n\x1a\n"))
	writeFile(t, filepath.Join(root, "xdg", "data", "applications", "viewer.desktop"), []byte(
		"[Desktop Entry]\nType=Application\nName=Viewer\nExec=sh -c \"echo \\\"$1\\\" > "+marker+"\" sh %f\nMimeType=image/png;\n"))

	require.NoError(t, svc.Open(context.Background(), []string{target}, LaunchOptions{}))
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(data)) == target
	}, waitFor, waitTick)
}

func TestOpenReportsMissingHandler(t *testing.T) {
	svc, root := fixture(t)

	target := writeFile(t, filepath.Join(root, "files", "disc.iso"), []byte("nothing handles this"))
	err := svc.Open(context.Background(), []string{target}, LaunchOptions{})
	require.Error(t, err)
	assert.Equal(t, files.CodeNotSupported, files.CodeOf(err))

	err = svc.Open(context.Background(), nil, LaunchOptions{})
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
}

func TestOpenGroupsPathsByTheirDefaultApp(t *testing.T) {
	svc, root := fixture(t)

	note := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	shot := writeFile(t, filepath.Join(root, "files", "shot.png"), []byte("\x89PNG\r\n\x1a\n"))
	readme := writeFile(t, filepath.Join(root, "files", "readme.md"), []byte("# hi"))
	disc := writeFile(t, filepath.Join(root, "files", "disc.iso"), []byte("nothing handles this"))
	photo := writeFile(t, filepath.Join(root, "files", "photo.jpg"), []byte("\xff\xd8\xff"))

	groups, unhandled, err := svc.groupByApp([]string{note, shot, readme, disc, photo})
	require.NoError(t, err)
	assert.Equal(t, []appGroup{
		{app: "editor.desktop", paths: []string{note, readme}, mimes: []string{"text/plain", "text/markdown"}},
		{app: "viewer.desktop", paths: []string{shot}, mimes: []string{"image/png"}},
		{app: "system-viewer.desktop", paths: []string{photo}, mimes: []string{"image/jpeg"}},
	}, groups, "two types with the same default app share one launch")
	assert.Equal(t, []string{"application/x-cd-image"}, unhandled)
}

func TestOpenStillLaunchesTheHandledPaths(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "opened")
	shot := writeFile(t, filepath.Join(root, "files", "shot.png"), []byte("\x89PNG\r\n\x1a\n"))
	disc := writeFile(t, filepath.Join(root, "files", "disc.iso"), []byte("nothing handles this"))
	writeFile(t, filepath.Join(root, "xdg", "data", "applications", "viewer.desktop"), []byte(
		"[Desktop Entry]\nType=Application\nName=Viewer\nExec=sh -c \"echo \\\"$1\\\" > "+marker+"\" sh %f\nMimeType=image/png;\n"))

	err := svc.Open(context.Background(), []string{disc, shot}, LaunchOptions{})
	require.Error(t, err)
	assert.Equal(t, files.CodeNotSupported, files.CodeOf(err))
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && strings.TrimSpace(string(data)) == shot
	}, waitFor, waitTick, "the png opened although the iso has no handler")
}

func TestOpenWithRejectsUnknownEntry(t *testing.T) {
	svc, root := fixture(t)

	target := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	err := svc.OpenWith(context.Background(), "nope.desktop", []string{target}, LaunchOptions{})
	require.Error(t, err)
	assert.Equal(t, files.CodeNotFound, files.CodeOf(err))
}

func TestRunRequiresTheExecuteBit(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "ran")
	script := writeFile(t, filepath.Join(root, "files", "go.sh"), []byte("#!/bin/sh\ntouch "+marker+"\n"))

	err := svc.Run(script, LaunchOptions{})
	require.Error(t, err)
	assert.Equal(t, files.CodeAccess, files.CodeOf(err))

	require.NoError(t, os.Chmod(script, 0o755))
	require.NoError(t, svc.Run(script, LaunchOptions{}))
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, waitFor, waitTick)

	assert.Equal(t, files.CodeInvalid, files.CodeOf(svc.Run(root, LaunchOptions{})))
	assert.Equal(t, files.CodeNotFound, files.CodeOf(svc.Run(filepath.Join(root, "gone"), LaunchOptions{})))
}

func TestRunLaunchesATrustedDesktopFile(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "desktop-ran")
	entry := writeFile(t, filepath.Join(root, "files", "launcher.desktop"), []byte(
		"[Desktop Entry]\nType=Application\nName=Launcher\nExec=touch "+marker+"\n"))

	err := svc.Run(entry, LaunchOptions{})
	require.Error(t, err)
	assert.Equal(t, files.CodeAccess, files.CodeOf(err), "an untrusted desktop file is never run")

	require.NoError(t, os.Chmod(entry, 0o755))
	require.NoError(t, svc.Run(entry, LaunchOptions{}))
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, waitFor, waitTick)
}

func TestDesktopDisplayNeedsTrust(t *testing.T) {
	svc, root := fixture(t)
	entry := writeFile(t, filepath.Join(root, "files", "launcher.desktop"), []byte(
		"[Desktop Entry]\nType=Application\nName=Launcher\nIcon=rocket\nExec=true\n"))

	_, _, ok := svc.Detector().DesktopDisplay(entry, false)
	assert.False(t, ok)

	name, icon, ok := svc.Detector().DesktopDisplay(entry, true)
	assert.True(t, ok)
	assert.Equal(t, "Launcher", name)
	assert.Equal(t, "rocket", icon)
}

func TestTerminalLookup(t *testing.T) {
	svc, _ := fixture(t)

	t.Setenv("TERMINAL", "dfiles-no-such-terminal")
	spec, err := svc.terminal("")
	if err == nil {
		assert.NotEqual(t, "dfiles-no-such-terminal", spec.command, "a terminal that is not installed is skipped")
	}

	t.Setenv("TERMINAL", "sh")
	spec, err = svc.terminal("")
	require.NoError(t, err)
	assert.Equal(t, "sh", spec.command)
	assert.Equal(t, []string{"-e"}, spec.exec, "an unknown terminal is assumed to take -e")

	spec, err = svc.terminal("/bin/sh")
	require.NoError(t, err)
	assert.Equal(t, "/bin/sh", spec.command)
}

func TestOpenTerminalInheritsTheDirectory(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "cwd")
	term := writeFile(t, filepath.Join(root, "bin", "faketerm"), []byte("#!/bin/sh\npwd > "+marker+"\n"))
	require.NoError(t, os.Chmod(term, 0o755))
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := filepath.Join(root, "files")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, svc.OpenTerminal(dir, "faketerm"))
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && string(data) == dir+"\n"
	}, waitFor, waitTick)
}

func TestFileURIRoundTrip(t *testing.T) {
	assert.Equal(t, "/tmp/a b", fileURIPath("file:///tmp/a%20b"))
	assert.Equal(t, "/tmp/a", fileURIPath("file://localhost/tmp/a"))
	assert.Empty(t, fileURIPath("https://example.invalid"))
}

func TestLaunchesLeaveASystemdUnitsCgroup(t *testing.T) {
	argv := []string{"gedit", "/tmp/a b"}
	found := func(name string) (string, error) {
		if name == "missing-app" {
			return "", os.ErrNotExist
		}
		return "/usr/bin/" + name, nil
	}
	missing := func(string) (string, error) { return "", os.ErrNotExist }
	unit := func(key string) string { return map[string]string{"INVOCATION_ID": "0123"}[key] }
	terminal := func(string) string { return "" }

	assert.Equal(t, []string{"/usr/bin/systemd-run", "--user", "--scope", "--quiet", "--collect", "--slice=app.slice", "--", "gedit", "/tmp/a b"}, inOwnScope(argv, unit, found))
	assert.Equal(t, argv, inOwnScope(argv, terminal, found), "a daemon started from a terminal already lives in that scope")
	assert.Equal(t, argv, inOwnScope(argv, unit, missing), "without systemd-run the launch goes ahead unwrapped")
	assert.Equal(t, []string{"missing-app"}, inOwnScope([]string{"missing-app"}, unit, found), "a missing binary still fails at exec instead of inside systemd-run")
}

func TestOpenDelegatesToTheOpener(t *testing.T) {
	svc, root := fixture(t)

	marker := filepath.Join(root, "opened")
	opener := writeFile(t, filepath.Join(root, "bin", "opener"), []byte("#!/bin/sh\necho \"$1 $2\" >> "+marker+"\n"))
	require.NoError(t, os.Chmod(opener, 0o755))
	note := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	disc := writeFile(t, filepath.Join(root, "files", "disc.iso"), []byte("nothing handles this"))

	require.NoError(t, svc.Open(context.Background(), []string{note, disc}, LaunchOptions{Opener: opener + " --quiet"}))
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(marker)
		if err != nil {
			return false
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		slices.Sort(lines)
		return slices.Equal(lines, []string{"--quiet " + disc, "--quiet " + note})
	}, waitFor, waitTick, "the opener ran once per path, unregistered types included")
}

func TestOpenReportsAMissingOpener(t *testing.T) {
	svc, root := fixture(t)
	t.Setenv("PATH", root)

	note := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	err := svc.Open(context.Background(), []string{note}, LaunchOptions{Opener: "no-such-opener"})
	require.Error(t, err)
	assert.Equal(t, files.CodeNotFound, files.CodeOf(err))
}
