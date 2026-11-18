package filemanager1

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/dbustest"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (r *recorder) record(method string, uris []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, method+" "+strings.Join(uris, " "))
	return r.err
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *recorder) ShowFolders(uris []string) error { return r.record("folders", uris) }

func (r *recorder) ShowItems(uris []string) error { return r.record("items", uris) }

func (r *recorder) ShowItemProperties(uris []string) error { return r.record("properties", uris) }

func start(t *testing.T, address string, handler Handler) *Service {
	t.Helper()
	conn, err := dbus.Connect(address)
	require.NoError(t, err)
	svc, err := New(conn, handler)
	require.NoError(t, err)
	t.Cleanup(svc.Close)
	return svc
}

func TestIntrospectionMatchesTheInterface(t *testing.T) {
	address := dbustest.Bus(t)
	svc := start(t, address, &recorder{})
	require.True(t, svc.SetEnabled(true).Owned)

	node, err := introspect.Call(dbustest.Connect(t, address).Object(Name, Path))
	require.NoError(t, err)

	signatures := map[string]string{}
	for _, iface := range node.Interfaces {
		if iface.Name != Interface {
			continue
		}
		for _, method := range iface.Methods {
			var args []string
			for _, arg := range method.Args {
				args = append(args, arg.Direction+":"+arg.Type)
			}
			signatures[method.Name] = strings.Join(args, ",")
		}
	}
	assert.Equal(t, map[string]string{
		"ShowFolders":        "in:as,in:s",
		"ShowItems":          "in:as,in:s",
		"ShowItemProperties": "in:as,in:s",
	}, signatures)
}

func TestMethodsReachTheHandler(t *testing.T) {
	address := dbustest.Bus(t)
	rec := &recorder{}
	svc := start(t, address, rec)
	require.True(t, svc.SetEnabled(true).Owned)

	obj := dbustest.Connect(t, address).Object(Name, Path)
	uris := []string{"file:///home/me/a.txt", "file:///home/me/b%20c.txt"}
	require.NoError(t, obj.Call(Interface+".ShowFolders", 0, uris[:1], "").Err)
	require.NoError(t, obj.Call(Interface+".ShowItems", 0, uris, "startup-token").Err)
	require.NoError(t, obj.Call(Interface+".ShowItemProperties", 0, uris[1:], "").Err)

	assert.Equal(t, []string{
		"folders file:///home/me/a.txt",
		"items file:///home/me/a.txt file:///home/me/b%20c.txt",
		"properties file:///home/me/b%20c.txt",
	}, rec.seen())
}

func TestHandlerErrorsReachTheCaller(t *testing.T) {
	address := dbustest.Bus(t)
	svc := start(t, address, &recorder{err: errors.New("/nope does not exist")})
	require.True(t, svc.SetEnabled(true).Owned)

	err := dbustest.Connect(t, address).Object(Name, Path).Call(Interface+".ShowItems", 0, []string{"file:///nope"}, "").Err
	var dbusErr dbus.Error
	require.ErrorAs(t, err, &dbusErr)
	assert.Equal(t, "org.freedesktop.DBus.Error.Failed", dbusErr.Name)
	assert.Contains(t, dbusErr.Error(), "/nope does not exist")
}

func TestAnotherOwnerIsNotReplaced(t *testing.T) {
	address := dbustest.Bus(t)
	other := dbustest.Connect(t, address)
	reply, err := other.RequestName(Name, 0)
	require.NoError(t, err)
	require.Equal(t, dbus.RequestNameReplyPrimaryOwner, reply)

	svc := start(t, address, &recorder{})
	status := svc.SetEnabled(true)
	assert.True(t, status.Enabled)
	assert.False(t, status.Owned)
	assert.True(t, status.Queued)
	assert.NotEmpty(t, status.Owner)

	_, err = other.ReleaseName(Name)
	require.NoError(t, err)
	assert.True(t, svc.Status().Owned, "the queued request takes over once the holder lets go")
}

func TestDisablingReleasesTheName(t *testing.T) {
	address := dbustest.Bus(t)
	svc := start(t, address, &recorder{})
	require.True(t, svc.SetEnabled(true).Owned)

	status := svc.SetEnabled(false)
	assert.False(t, status.Enabled)
	assert.False(t, status.Owned)

	reply, err := dbustest.Connect(t, address).RequestName(Name, dbus.NameFlagDoNotQueue)
	require.NoError(t, err)
	assert.Equal(t, dbus.RequestNameReplyPrimaryOwner, reply)
}
