package mimeapps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addApp(t *testing.T, root, id, mimes string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "xdg", "data", "applications", id), []byte(
		"[Desktop Entry]\nType=Application\nName="+id+"\nExec=sh -c \"true\" %f\nMimeType="+mimes+"\n"))
}

func TestRecordUsePrependsAndKeepsTheRestOfTheFile(t *testing.T) {
	svc, root := fixture(t)
	path := filepath.Join(root, "xdg", "config", "mimeapps.list")

	require.NoError(t, svc.RecordUse("text/markdown", "viewer.desktop"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "[Default Applications]\ntext/plain=ghost.desktop;editor.desktop;\nimage/png=viewer.desktop\n\n[Added Associations]\ntext/markdown=viewer.desktop;editor.desktop;\n\n[Removed Associations]\nimage/png=system-viewer.desktop;\n", string(data))

	require.NoError(t, svc.RecordUse("text/markdown", "viewer.desktop"))
	require.NoError(t, svc.RecordUse("image/x-png", "system-viewer.desktop"))
	assert.Equal(t, []string{"viewer.desktop", "editor.desktop"}, idsOf(svc.RecentFor("text/markdown")), "a repeat does not duplicate")
	assert.Equal(t, []string{"system-viewer.desktop"}, idsOf(svc.RecentFor("image/png")), "aliases fold onto the canonical type")

	require.Error(t, svc.RecordUse("text/plain", "nope.desktop"))
}

func TestRecordUseCreatesTheGroup(t *testing.T) {
	svc, root := fixture(t)
	path := filepath.Join(root, "xdg", "config", "mimeapps.list")
	require.NoError(t, os.WriteFile(path, []byte("[Default Applications]\ntext/plain=editor.desktop\n"), 0o644))

	require.NoError(t, svc.RecordUse("text/plain", "viewer.desktop"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "[Default Applications]\ntext/plain=editor.desktop\n\n[Added Associations]\ntext/plain=viewer.desktop;\n", string(data))
}

func TestRecordUseDoesNotOverrideAnExplicitDefault(t *testing.T) {
	svc, _ := fixture(t)
	assert.Equal(t, "system-viewer.desktop", svc.Default("image/jpeg"))

	require.NoError(t, svc.RecordUse("image/jpeg", "viewer.desktop"))
	assert.Equal(t, "system-viewer.desktop", svc.Default("image/jpeg"), "a default set in a lower-precedence file still beats the user's added associations")
	assert.Equal(t, []string{"system-viewer.desktop", "viewer.desktop"}, idsOf(svc.Associations("image/jpeg")))
	assert.Equal(t, []string{"viewer.desktop"}, idsOf(svc.RecentFor("image/jpeg")))
}

func TestSectionsHideNoDisplayUnlessDefault(t *testing.T) {
	svc, root := fixture(t)
	assert.NotContains(t, idsOf(svc.Sections([]string{"text/markdown"}).Recommended), "background.desktop")

	path := filepath.Join(root, "xdg", "config", "mimeapps.list")
	require.NoError(t, os.WriteFile(path, []byte("[Default Applications]\ntext/markdown=background.desktop\n"), 0o644))
	sections := svc.Sections([]string{"text/markdown"})
	assert.Equal(t, "background.desktop", sections.DefaultID)
	assert.Contains(t, idsOf(sections.Recommended), "background.desktop", "the configured default stays visible")
}

func TestSectionsHideSelfEvenAsDefault(t *testing.T) {
	svc, root := fixture(t)
	addApp(t, root, selfDesktopID, "inode/directory;")
	addApp(t, root, "other-fm.desktop", "inode/directory;")
	require.NoError(t, svc.SetDefault("inode/directory", selfDesktopID))

	sections := svc.Sections([]string{"inode/directory"})
	assert.Equal(t, "", sections.DefaultID)
	assert.Equal(t, []string{"other-fm.desktop"}, idsOf(sections.Recommended))
	assert.NotContains(t, idsOf(sections.Other), selfDesktopID)
}

func TestSectionsSingleType(t *testing.T) {
	svc, root := fixture(t)
	addApp(t, root, selfDesktopID, "inode/directory;text/plain;")
	addApp(t, root, "notes.desktop", "text/plain;")
	require.NoError(t, svc.RecordUse("text/plain", "viewer.desktop"))
	require.NoError(t, svc.RecordUse("text/plain", "notes.desktop"))
	require.NoError(t, svc.RecordUse("text/plain", "editor.desktop"))

	sections := svc.Sections([]string{"text/plain"})
	assert.Equal(t, []string{"text/plain"}, sections.MimeTypes)
	assert.Equal(t, "editor.desktop", sections.DefaultID)
	assert.Equal(t, []string{"notes.desktop", "viewer.desktop"}, idsOf(sections.Recent), "the default stays out of recent")
	assert.Equal(t, "editor.desktop", idsOf(sections.Recommended)[0], "the default leads recommended")
	assert.NotContains(t, idsOf(sections.Recommended), "notes.desktop")
	assert.Equal(t, []string{"system-viewer.desktop", "vendor-terminalapp.desktop"}, idsOf(sections.Other))
	for _, group := range [][]*Entry{sections.Recent, sections.Recommended, sections.Other} {
		assert.NotContains(t, idsOf(group), selfDesktopID)
	}
}

func TestSectionsCapsRecentAtFive(t *testing.T) {
	svc, root := fixture(t)
	for _, id := range []string{"a.desktop", "b.desktop", "c.desktop", "d.desktop", "e.desktop", "f.desktop"} {
		addApp(t, root, id, "text/x-csrc;")
		require.NoError(t, svc.RecordUse("text/x-csrc", id))
	}

	sections := svc.Sections([]string{"text/x-csrc"})
	assert.Equal(t, []string{"e.desktop", "d.desktop", "c.desktop", "b.desktop", "a.desktop"}, idsOf(sections.Recent), "with no explicit default the last used app is the default, as in gio")
	assert.Equal(t, "f.desktop", sections.DefaultID)
	assert.Equal(t, "f.desktop", idsOf(sections.Recommended)[0])
	assert.Equal(t, []string{"vendor-terminalapp.desktop", "editor.desktop"}, idsOf(sections.Recommended)[1:], "the text/plain parent contributes its handler")
}

func TestSectionsMixedTypesIntersect(t *testing.T) {
	svc, root := fixture(t)
	addApp(t, root, "both.desktop", "text/plain;image/png;")
	require.NoError(t, svc.RecordUse("text/plain", "both.desktop"))
	require.NoError(t, svc.RecordUse("image/png", "both.desktop"))
	require.NoError(t, svc.RecordUse("text/plain", "editor.desktop"))

	sections := svc.Sections([]string{"text/plain", "image/x-png", "text/plain"})
	assert.Equal(t, []string{"text/plain", "image/png"}, sections.MimeTypes)
	assert.Equal(t, "", sections.DefaultID, "a mixed selection has no default to set")
	assert.Equal(t, []string{"both.desktop"}, idsOf(sections.Recent), "editor was used for text only")
	assert.Equal(t, []string{}, idsOf(sections.Recommended), "no other app handles both types")
	assert.Contains(t, idsOf(sections.Other), "editor.desktop")
	assert.Contains(t, idsOf(sections.Other), "viewer.desktop")
}

func TestSectionsHandlerTypesPaths(t *testing.T) {
	svc, root := fixture(t)
	note := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))
	shot := writeFile(t, filepath.Join(root, "files", "shot.png"), []byte("\x89PNG\r\n\x1a\n"))

	result := resultOf(t, call(t, svc, ipc.Request{ID: 1, Method: "files.apps.sections", Params: map[string]any{"paths": []any{note, shot}}}))
	assert.Equal(t, []any{"text/plain", "image/png"}, result["mimeTypes"])
	assert.Equal(t, true, result["mixed"])
	assert.Equal(t, []any{}, result["recommended"])

	single := resultOf(t, call(t, svc, ipc.Request{ID: 2, Method: "files.apps.sections", Params: map[string]any{"mimes": []any{"text/plain"}}}))
	assert.Equal(t, "editor.desktop", single["defaultId"])
	assert.Equal(t, false, single["mixed"])
	recommended := single["recommended"].([]any)
	assert.Equal(t, true, recommended[0].(map[string]any)["isDefault"])

	out := call(t, svc, ipc.Request{ID: 3, Method: "files.apps.sections", Params: map[string]any{}})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestOpenWithHandlerRecordsTheChoice(t *testing.T) {
	svc, root := fixture(t)
	note := writeFile(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))

	result := resultOf(t, call(t, svc, ipc.Request{ID: 1, Method: "files.openWith", Params: map[string]any{"paths": []any{note}, "desktopId": "viewer.desktop"}}))
	assert.Equal(t, true, result["recorded"])
	assert.Equal(t, []string{"viewer.desktop"}, idsOf(svc.RecentFor("text/plain")))

	resultOf(t, call(t, svc, ipc.Request{ID: 2, Method: "files.openWith", Params: map[string]any{"paths": []any{note}, "desktopId": "editor.desktop", "record": false}}))
	assert.Equal(t, []string{"viewer.desktop"}, idsOf(svc.RecentFor("text/plain")), "record=false leaves the list alone")
}
