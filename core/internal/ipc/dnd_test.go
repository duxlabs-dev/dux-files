package ipc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/dbustest"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDndExtractValidatesItsInput(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"missing params", map[string]any{}, "unique bus name"},
		{"well known service", map[string]any{"service": "org.kde.ark.1", "path": "/DndExtract/1", "dest": dir}, "unique bus name"},
		{"wrong object path", map[string]any{"service": ":1.2", "path": "/Object", "dest": dir}, "object path"},
		{"relative dest", map[string]any{"service": ":1.2", "path": "/DndExtract/1", "dest": "out"}, "absolute dest"},
		{"missing dest", map[string]any{"service": ":1.2", "path": "/DndExtract/1", "dest": filepath.Join(dir, "gone")}, "not a directory"},
		{"dest is a file", map[string]any{"service": ":1.2", "path": "/DndExtract/1", "dest": filepath.Join(dir, "file")}, "not a directory"},
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0o644))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := routeAndRead(t, Request{ID: 1, Method: "files.dndExtract", Params: tc.params}, Deps{})
			message, _ := out["error"].(string)
			assert.Contains(t, message, tc.want)
			assert.Nil(t, out["result"])
		})
	}
}

func TestDndExtractCallsArksSessionObject(t *testing.T) {
	address := dbustest.Bus(t)
	serviceConn := dbustest.Connect(t, address)
	clientConn := dbustest.Connect(t, address)
	service := uniqueName(t, serviceConn)
	dest := t.TempDir()
	calls := make(chan string, 1)

	require.NoError(t, serviceConn.ExportMethodTable(map[string]any{
		"extractSelectedFilesTo": func(destination string) *dbus.Error {
			calls <- destination
			return nil
		},
	}, dbus.ObjectPath("/DndExtract/1"), arkDndInterface))

	restore := dndSessionBus
	dndSessionBus = func() (*dbus.Conn, error) { return clientConn, nil }
	t.Cleanup(func() { dndSessionBus = restore })

	out := routeAndRead(t, Request{ID: 2, Method: "files.dndExtract", Params: map[string]any{
		"service": service,
		"path":    "/DndExtract/1",
		"dest":    dest,
	}}, Deps{})

	assert.Equal(t, map[string]any{"ok": true}, out["result"])
	assert.Equal(t, dest, <-calls)
}

func TestDndExtractReportsArkFailure(t *testing.T) {
	address := dbustest.Bus(t)
	serviceConn := dbustest.Connect(t, address)
	service := uniqueName(t, serviceConn)
	dest := t.TempDir()

	require.NoError(t, serviceConn.ExportMethodTable(map[string]any{
		"extractSelectedFilesTo": func(string) *dbus.Error {
			return dbus.NewError("org.kde.ark.ReadOnly", []any{"archive is read only"})
		},
	}, dbus.ObjectPath("/DndExtract/2"), arkDndInterface))

	restore := dndSessionBus
	dndSessionBus = func() (*dbus.Conn, error) { return dbus.Connect(address) }
	t.Cleanup(func() { dndSessionBus = restore })

	out := routeAndRead(t, Request{ID: 3, Method: "files.dndExtract", Params: map[string]any{
		"service": service,
		"path":    "/DndExtract/2",
		"dest":    dest,
	}}, Deps{})

	message, _ := out["error"].(string)
	assert.Contains(t, message, "ark drag extraction failed")
	assert.Contains(t, message, "archive is read only")
	assert.Nil(t, out["result"])
}

func TestDndExtractRouteNeedsNoFileService(t *testing.T) {
	out := routeAndRead(t, Request{ID: 4, Method: "files.dndExtract", Params: map[string]any{}}, Deps{})
	message, _ := out["error"].(string)
	assert.False(t, strings.Contains(message, "file service unavailable"))
}

func uniqueName(t *testing.T, conn *dbus.Conn) string {
	t.Helper()
	for _, name := range conn.Names() {
		if strings.HasPrefix(name, ":") {
			return name
		}
	}
	t.Fatal("connection has no unique name")
	return ""
}
