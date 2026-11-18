package ipctest

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/stretchr/testify/require"
)

func Call(t *testing.T, handle func(w *ipc.ConnWriter)) map[string]any {
	t.Helper()
	client, srv := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = srv.Close()
	})

	go handle(ipc.NewConnWriter(srv))

	line, err := bufio.NewReader(client).ReadBytes('\n')
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(line, &out))
	return out
}
