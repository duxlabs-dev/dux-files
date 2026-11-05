package mimeapps

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/testfs"

	"github.com/duxlabs-dev/dux-files/core/internal/ipctest"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func call(t *testing.T, svc *Service, req ipc.Request) map[string]any {
	t.Helper()
	return ipctest.Call(t, func(w *ipc.ConnWriter) { svc.Handle(context.Background(), w, req) })
}

func resultOf(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	require.NotContains(t, out, "error", "unexpected error response")
	result, ok := out["result"].(map[string]any)
	require.True(t, ok)
	return result
}

func TestHandleForMime(t *testing.T) {
	svc, _ := fixture(t)

	result := resultOf(t, call(t, svc, ipc.Request{ID: 1, Method: "files.apps.forMime", Params: map[string]any{"mime": "text/plain"}}))
	assert.Equal(t, "text/plain", result["mimeType"])
	assert.Equal(t, "editor.desktop", result["defaultId"])

	apps := result["apps"].([]any)
	require.NotEmpty(t, apps)
	first := apps[0].(map[string]any)
	assert.Equal(t, "editor.desktop", first["id"])
	assert.Equal(t, "Editor", first["name"])
	assert.Equal(t, "accessories-text-editor", first["icon"])
	assert.Equal(t, true, first["isDefault"])
}

func TestHandleForMimeTypesAPath(t *testing.T) {
	svc, root := fixture(t)
	note := testfs.WriteBytes(t, filepath.Join(root, "files", "note.txt"), []byte("hello"))

	result := resultOf(t, call(t, svc, ipc.Request{ID: 2, Method: "files.apps.forMime", Params: map[string]any{"path": note}}))
	assert.Equal(t, "text/plain", result["mimeType"])

	out := call(t, svc, ipc.Request{ID: 3, Method: "files.apps.forMime", Params: map[string]any{}})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleAppsAll(t *testing.T) {
	svc, _ := fixture(t)

	result := resultOf(t, call(t, svc, ipc.Request{ID: 4, Method: "files.apps.all"}))
	apps := result["apps"].([]any)
	assert.Len(t, apps, 4)
	assert.Equal(t, "Editor", apps[0].(map[string]any)["name"])
}

func TestHandleSetDefault(t *testing.T) {
	svc, _ := fixture(t)

	result := resultOf(t, call(t, svc, ipc.Request{ID: 5, Method: "files.apps.setDefault",
		Params: map[string]any{"mime": "image/png", "desktopId": "system-viewer.desktop"}}))
	assert.Equal(t, "system-viewer.desktop", result["desktopId"])
	assert.Equal(t, "system-viewer.desktop", svc.Default("image/png"))

	out := call(t, svc, ipc.Request{ID: 6, Method: "files.apps.setDefault", Params: map[string]any{"mime": "image/png"}})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleErrorsCarryCodes(t *testing.T) {
	svc, root := fixture(t)

	out := call(t, svc, ipc.Request{ID: 7, Method: "files.open", Params: map[string]any{}})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 8, Method: "files.open",
		Params: map[string]any{"paths": []any{filepath.Join(root, "gone")}}})
	assert.Equal(t, "ENOENT", out["code"])

	out = call(t, svc, ipc.Request{ID: 9, Method: "files.openWith", Params: map[string]any{"paths": []any{root}}})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 10, Method: "files.run", Params: map[string]any{}})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 11, Method: "files.openTerminal", Params: map[string]any{}})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 12, Method: "files.nope"})
	assert.Equal(t, "NOTSUPPORTED", out["code"])
}
