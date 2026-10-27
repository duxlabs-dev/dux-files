package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetIconThemeNotifiesRunningUI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "system"))
	icon := filepath.Join(root, "data", "icons", "Test", "scalable", "places", "folder.svg")
	require.NoError(t, os.MkdirAll(filepath.Dir(icon), 0755))
	require.NoError(t, os.WriteFile(icon, []byte("<svg/>"), 0644))
	bus := NewEventBus()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	require.NoError(t, client.SetReadDeadline(time.Now().Add(3*time.Second)))
	sub := bus.NewSubscriber(context.Background(), NewConnWriter(server))
	defer sub.Close()
	sub.Subscribe("iconTheme")
	events := make(chan []byte, 1)
	go func() {
		line, _ := bufio.NewReader(client).ReadBytes('\n')
		events <- line
	}()
	response := routeAndRead(t, Request{ID: 1, Method: "files.setIconTheme", Params: map[string]any{"name": "Test"}}, Deps{Bus: bus})
	result := response["result"].(map[string]any)
	require.Equal(t, "Test", result["effective"])
	require.NotContains(t, result, "paths")
	paths := routeAndRead(t, Request{ID: 2, Method: "files.iconThemePaths"}, Deps{})
	require.Equal(t, icon, paths["result"].(map[string]any)["folder"])
	line := <-events
	var event map[string]any
	require.NoError(t, json.Unmarshal(line, &event))
	require.Equal(t, "iconTheme", event["event"])
	require.Equal(t, "Test", event["data"].(map[string]any)["effective"])
}
