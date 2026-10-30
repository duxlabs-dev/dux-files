package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-go/trash"
)

func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"runtime", "data", "cache", "config", "home"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o700))
	}
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("DFILES_SOCKET", "")
	t.Setenv("FLATPAK_ID", "")
	return root
}

func TestParseParams(t *testing.T) {
	params, err := parseParams([]string{"path=/a", "paths=/x", "paths=/y", "select=[\"/z\"]", "name=[draft", "extra=/one", "extra=[\"/two\"]"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"path":   "/a",
		"paths":  []any{"/x", "/y"},
		"select": []any{"/z"},
		"name":   "[draft",
		"extra":  []any{"/one", "/two"},
	}, params)

	_, err = parseParams([]string{"novalue"})
	assert.Error(t, err)
}

func TestTrashRoundTripWithoutADaemon(t *testing.T) {
	root := isolate(t)
	target := filepath.Join(root, "home", "notes.txt")
	require.NoError(t, os.WriteFile(target, []byte("first"), 0o644))

	var put jobResult
	require.NoError(t, withSession(fileServices, func(s *session) error {
		var err error
		put, err = runJobUnattended(s, "files.op.trash", map[string]any{"paths": []string{target}}, "skip")
		return err
	}))
	require.NoError(t, put.err())
	assert.True(t, put.succeeded(target))
	assert.NoFileExists(t, target)

	entries, err := trashEntries()
	require.NoError(t, err)
	entries = inTrashHome(entries, root)
	require.Len(t, entries, 1)
	assert.Equal(t, target, entries[0].OriginalPath)

	require.NoError(t, os.WriteFile(target, []byte("second"), 0o644))
	byDir, order, err := trashDirsFor([]string{entries[0].Name})
	require.NoError(t, err)
	require.Len(t, order, 1)

	var restored jobResult
	require.NoError(t, withSession(fileServices, func(s *session) error {
		restored, err = runJobUnattended(s, "files.op.restore", map[string]any{"names": byDir[order[0]], "trashDir": order[0]}, "keepBoth")
		return err
	}))
	require.NoError(t, restored.err())

	current, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "second", string(current), "keep both leaves the newer file in place")
	kept, err := os.ReadFile(filepath.Join(root, "home", "notes (copy).txt"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(kept))

	left, err := trash.List()
	require.NoError(t, err)
	assert.Empty(t, inTrashHome(left, root))
}

func inTrashHome(entries []trash.Entry, root string) []trash.Entry {
	home := filepath.Join(root, "data", "Trash")
	var out []trash.Entry
	for _, entry := range entries {
		if entry.TrashDir == home {
			out = append(out, entry)
		}
	}
	return out
}

func TestTrashFailuresAreReportedPerItem(t *testing.T) {
	root := isolate(t)
	kept := filepath.Join(root, "home", "kept.txt")
	require.NoError(t, os.WriteFile(kept, nil, 0o644))
	missing := filepath.Join(root, "home", "missing.txt")

	var result jobResult
	require.NoError(t, withSession(fileServices, func(s *session) error {
		var err error
		result, err = runJobUnattended(s, "files.op.trash", map[string]any{"paths": []string{kept, missing}}, "skip")
		return err
	}))
	assert.True(t, result.succeeded(kept))
	assert.False(t, result.succeeded(missing))
	assert.ErrorContains(t, result.err(), "failed for 1 item")
	assert.NoFileExists(t, kept)
}

func TestRestoringAnUnknownNameFails(t *testing.T) {
	isolate(t)
	_, _, err := trashDirsFor([]string{"nothing-here.txt"})
	assert.ErrorContains(t, err, "not in the trash")
}

func TestSearchStreamsWithoutADaemon(t *testing.T) {
	root := isolate(t)
	tree := filepath.Join(root, "home", "tree")
	for _, dir := range []string{"a", "a/b", ".hidden"} {
		require.NoError(t, os.MkdirAll(filepath.Join(tree, dir), 0o755))
	}
	for _, file := range []string{"report.txt", "a/report-2.txt", "a/b/other.txt", ".hidden/report.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(tree, file), nil, 0o644))
	}

	var paths []string
	var done searchEvent
	require.NoError(t, withSession(searchServices, func(s *session) error {
		var err error
		done, err = runSearch(s, map[string]any{"root": tree, "query": "REPORT"}, func(raw json.RawMessage) {
			var entry struct {
				Path string `json:"path"`
			}
			require.NoError(t, json.Unmarshal(raw, &entry))
			paths = append(paths, entry.Path)
		})
		return err
	}))

	assert.ElementsMatch(t, []string{filepath.Join(tree, "report.txt"), filepath.Join(tree, "a", "report-2.txt")}, paths)
	assert.Equal(t, 2, done.Count)
	assert.False(t, done.Truncated)
}

func TestFindDevice(t *testing.T) {
	devices := []device{
		{ID: "/org/freedesktop/UDisks2/block_devices/nvme0n1p2", Device: "/dev/nvme0n1p2", Kind: "system", MountPoint: "/"},
		{ID: "/org/freedesktop/UDisks2/block_devices/sdb1", Device: "/dev/sdb1", Label: "STICK", UUID: "1234-ABCD"},
	}
	for _, target := range []string{"/dev/sdb1", "STICK", "1234-ABCD", "/org/freedesktop/UDisks2/block_devices/sdb1"} {
		dev, err := findDevice(devices, target)
		require.NoError(t, err, target)
		assert.Equal(t, "/dev/sdb1", dev.Device, target)
	}
	dev, err := findDevice(devices, "/")
	require.NoError(t, err)
	assert.Equal(t, "system", dev.Kind)

	_, err = findDevice(devices, "")
	assert.Error(t, err)
	_, err = findDevice(devices, "/dev/sdc1")
	assert.Error(t, err)
}

func TestUIEnvSetsIconTheme(t *testing.T) {
	root := isolate(t)
	config := filepath.Join(root, "config")
	require.NoError(t, os.MkdirAll(config, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(config, "kdeglobals"), []byte("[Icons]\nTheme=WhiteSur-dark\n"), 0o644))

	t.Setenv("QS_ICON_THEME", "")
	env := uiEnv("")
	assert.Contains(t, env, "QS_ICON_THEME=WhiteSur-dark")

	t.Setenv("QS_ICON_THEME", "CustomTheme")
	env = uiEnv("")
	assert.NotContains(t, env, "QS_ICON_THEME=WhiteSur-dark")
}
