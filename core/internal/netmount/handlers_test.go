package netmount

import (
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/ipctest"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func call(t *testing.T, svc *Service, req ipc.Request) map[string]any {
	t.Helper()
	return ipctest.Call(t, func(w *ipc.ConnWriter) { svc.Handle(w, req) })
}

func mountedService(t *testing.T) *Service {
	t.Helper()
	svc := NewService(nil, t.TempDir())
	svc.available = true
	svc.reason = ""
	svc.mountables = mountables
	svc.mounts = []Mount{{ID: "ftp:host=example", Name: "probe on example", URI: "ftp://example/", Path: "/run/user/1000/gvfs/ftp:host=example", Scheme: "ftp", Kind: KindNetwork}}
	return svc
}

func TestHandleMountsListsWhatGvfsHolds(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 1, Method: "network.mounts"})

	result := out["result"].(map[string]any)
	assert.Equal(t, true, result["available"])
	assert.Equal(t, []any{"sftp", "smb", "ftp", "dav", "davs"}, result["schemes"])
	mounts := result["mounts"].([]any)
	require.Len(t, mounts, 1)
	assert.Equal(t, "ftp://example/", mounts[0].(map[string]any)["uri"])
}

func TestHandleMountsExplainsAMissingGvfs(t *testing.T) {
	svc := NewService(nil, t.TempDir())
	out := call(t, svc, ipc.Request{ID: 2, Method: "network.mounts"})

	result := out["result"].(map[string]any)
	assert.Equal(t, false, result["available"])
	assert.Equal(t, "gvfs is not running", result["reason"])
	assert.Equal(t, []any{}, result["schemes"])
}

func TestHandleConnectNeedsAUri(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 3, Method: "network.connect"})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleConnectReportsAMissingDaemon(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 4, Method: "network.connect", Params: map[string]any{"uri": "sftp://host/"}})
	assert.Equal(t, "NOTSUPPORTED", out["code"])
	assert.Contains(t, out["error"], "gvfs is not running")
}

func TestHandleDisconnectRejectsUnknownMounts(t *testing.T) {
	svc := mountedService(t)
	svc.conn = nil

	out := call(t, svc, ipc.Request{ID: 5, Method: "network.disconnect", Params: map[string]any{"id": "sftp://nope/"}})
	assert.Equal(t, "NOTSUPPORTED", out["code"])

	out = call(t, svc, ipc.Request{ID: 6, Method: "network.disconnect"})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleAnswerRejectsStaleQuestions(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 7, Method: "network.answer", Params: map[string]any{"id": "/dev/duxlabs/files/mountop/1/1"}})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleRecentRoundTripsThroughTheStore(t *testing.T) {
	svc := mountedService(t)
	svc.servers.remember(Server{URI: "sftp://one/"})

	out := call(t, svc, ipc.Request{ID: 8, Method: "network.recent"})
	assert.Len(t, out["result"].(map[string]any)["servers"], 1)

	out = call(t, svc, ipc.Request{ID: 9, Method: "network.forget", Params: map[string]any{"uri": "sftp://one/"}})
	assert.Empty(t, out["result"].(map[string]any)["servers"])
}

func TestHandleMountPortableNeedsBothIdentifiers(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 10, Method: "network.mountPortable", Params: map[string]any{"id": "volume"}})
	assert.Equal(t, "EINVAL", out["code"])
}

func TestHandleRejectsUnknownMethods(t *testing.T) {
	out := call(t, mountedService(t), ipc.Request{ID: 11, Method: "network.nope"})
	assert.Equal(t, "NOTSUPPORTED", out["code"])
}

func TestConnectReusesAMountThatCoversTheAddress(t *testing.T) {
	svc := mountedService(t)

	for _, uri := range []string{"ftp://example/", "ftp://example/pub/music"} {
		spec, err := ParseURI(uri, svc.mountableList())
		require.NoError(t, err)
		mount, found := svc.mountedAt(spec)
		require.True(t, found, uri)
		assert.Equal(t, "ftp:host=example", mount.ID)
	}

	spec, err := ParseURI("ftp://example.org/", svc.mountableList())
	require.NoError(t, err)
	_, found := svc.mountedAt(spec)
	assert.False(t, found)
}
