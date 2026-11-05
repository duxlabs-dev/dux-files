package ipc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/ipctest"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routeAndRead(t *testing.T, req Request, deps Deps) map[string]any {
	t.Helper()
	return ipctest.Call(t, func(w *ConnWriter) { Route(context.Background(), w, req, deps) })
}

func TestRouteVersion(t *testing.T) {
	out := routeAndRead(t, Request{ID: 1, Method: "version"}, Deps{Version: "1.2.3"})

	result, ok := out["result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "1.2.3", result["version"])
	assert.Equal(t, float64(APIVersion), result["apiVersion"])
}

func TestRouteUnknownMethod(t *testing.T) {
	out := routeAndRead(t, Request{ID: 2, Method: "nope.nothing"}, Deps{})

	errMsg, ok := out["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errMsg, "unknown method: nope.nothing")
	assert.Nil(t, out["result"])
}

func TestRouteFilesList(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "x"), nil, 0o644))

	out := routeAndRead(t, Request{ID: 3, Method: "files.list", Params: map[string]any{"path": root}}, Deps{Files: files.NewService(nil, t.TempDir(), nil)})

	result, ok := out["result"].(map[string]any)
	require.True(t, ok)
	assert.Len(t, result["entries"], 1)
}

func TestRouteFilesWithoutService(t *testing.T) {
	out := routeAndRead(t, Request{ID: 4, Method: "files.list", Params: map[string]any{"path": t.TempDir()}}, Deps{})
	assert.Equal(t, "file service unavailable", out["error"])
}

func TestRegisteredFilesMethodsReachTheService(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	deps := Deps{Files: files.NewService(nil, t.TempDir(), nil)}
	t.Cleanup(deps.Files.Close)

	for _, spec := range Methods {
		if spec.Group != "files" {
			continue
		}
		out := routeAndRead(t, Request{ID: 5, Method: spec.Name}, deps)
		assert.NotEqual(t, string(files.CodeNotSupported), out["code"], spec.Name)
	}
}

func TestEveryRegisteredMethodIsRouted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	deps := Deps{Bus: NewEventBus()}
	serverHandled := map[string]bool{"ping": true, "subscribe": true, "unsubscribe": true}
	for _, spec := range Methods {
		if serverHandled[spec.Name] {
			continue
		}
		out := routeAndRead(t, Request{ID: 6, Method: spec.Name}, deps)
		message, _ := out["error"].(string)
		assert.False(t, strings.HasPrefix(message, "unknown"), "%s: %s", spec.Name, message)
	}
}
