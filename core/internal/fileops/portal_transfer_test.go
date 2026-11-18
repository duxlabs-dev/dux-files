package fileops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

type fakePortalTransfer struct {
	key        string
	startErr   error
	addErr     error
	batches    [][]dbus.UnixFD
	stopped    []string
	addCalls   int
	startCalls int
	stopCalls  int
}

func (f *fakePortalTransfer) StartTransfer(context.Context) (string, error) {
	f.startCalls++
	return f.key, f.startErr
}

func (f *fakePortalTransfer) AddFiles(_ context.Context, _ string, fds []dbus.UnixFD) error {
	f.addCalls++
	f.batches = append(f.batches, append([]dbus.UnixFD(nil), fds...))
	return f.addErr
}

func (f *fakePortalTransfer) StopTransfer(_ context.Context, key string) {
	f.stopCalls++
	f.stopped = append(f.stopped, key)
}

func TestHandlePortalTransferRejectsInvalidParams(t *testing.T) {
	manager, _ := newHarness(t)

	for _, params := range []map[string]any{
		nil,
		{"paths": []string{}},
		{"paths": []string{"relative/file.txt"}},
	} {
		out := call(t, manager, ipc.Request{Method: "files.op.portalTransfer", Params: params})
		require.Contains(t, out, "error")
		require.Equal(t, "EINVAL", out["code"])
	}
}

func TestExportPortalTransferBatchesFiles(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 0, portalBatchSize+3)
	for i := range portalBatchSize + 3 {
		p := filepath.Join(dir, fmt.Sprintf("f-%d.txt", i))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
		paths = append(paths, p)
	}

	client := &fakePortalTransfer{key: "transfer-key"}
	key, err := exportPortalTransfer(context.Background(), client, paths)
	require.NoError(t, err)
	require.Equal(t, "transfer-key", key)
	require.Equal(t, 1, client.startCalls)
	require.Equal(t, 2, client.addCalls)
	require.Len(t, client.batches[0], portalBatchSize)
	require.Len(t, client.batches[1], 3)
}

func TestExportPortalTransferErrors(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(existing, []byte("x"), 0o644))

	t.Run("start error", func(t *testing.T) {
		client := &fakePortalTransfer{startErr: errors.New("boom")}
		key, err := exportPortalTransfer(context.Background(), client, []string{existing})
		require.Error(t, err)
		require.Empty(t, key)
		require.Zero(t, client.addCalls)
	})

	t.Run("missing file", func(t *testing.T) {
		client := &fakePortalTransfer{key: "k"}
		key, err := exportPortalTransfer(context.Background(), client, []string{filepath.Join(dir, "missing")})
		require.Error(t, err)
		require.Empty(t, key)
		require.Zero(t, client.addCalls)
		require.Equal(t, 1, client.stopCalls)
	})

	t.Run("add error", func(t *testing.T) {
		client := &fakePortalTransfer{key: "k", addErr: errors.New("add failed")}
		key, err := exportPortalTransfer(context.Background(), client, []string{existing})
		require.Error(t, err)
		require.Empty(t, key)
		require.Equal(t, 1, client.stopCalls)
	})
}

func TestManagerPortalTransferStopsPreviousKey(t *testing.T) {
	manager, _ := newHarness(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	client := &fakePortalTransfer{key: "first-key"}
	key, err := manager.exportPortalTransfer(context.Background(), client, []string{file})
	require.NoError(t, err)
	require.Equal(t, "first-key", key)
	require.Empty(t, client.stopped)

	client.key = "second-key"
	key, err = manager.exportPortalTransfer(context.Background(), client, []string{file})
	require.NoError(t, err)
	require.Equal(t, "second-key", key)
	require.Equal(t, []string{"first-key"}, client.stopped)
}
