package netmount

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServersKeepTheMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	store := newServerStore(dir)

	store.remember(Server{URI: "sftp://one/"})
	store.remember(Server{URI: "sftp://two/"})

	list := store.list()
	require.Len(t, list, 2)
	assert.Equal(t, "sftp://two/", list[0].URI)
	assert.Greater(t, list[0].LastUsed, int64(0))
}

func TestServersDeduplicateOnReconnect(t *testing.T) {
	store := newServerStore(t.TempDir())
	store.remember(Server{URI: "sftp://one/"})
	store.remember(Server{URI: "sftp://two/"})
	store.remember(Server{URI: "sftp://one/", Name: "One"})

	list := store.list()
	require.Len(t, list, 2)
	assert.Equal(t, "sftp://one/", list[0].URI)
	assert.Equal(t, "One", list[0].Name)
}

func TestServersAreCapped(t *testing.T) {
	store := newServerStore(t.TempDir())
	for i := 0; i < recentServers+5; i++ {
		store.remember(Server{URI: "sftp://host" + time.Duration(i).String() + "/"})
	}
	assert.Len(t, store.list(), recentServers)
}

func TestServersSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	newServerStore(dir).remember(Server{URI: "smb://server/share", Scheme: "smb"})

	reloaded := newServerStore(dir).list()
	require.Len(t, reloaded, 1)
	assert.Equal(t, "smb", reloaded[0].Scheme)
	assert.FileExists(t, filepath.Join(dir, "servers.json"))
}

func TestServersForgetDropsOne(t *testing.T) {
	store := newServerStore(t.TempDir())
	store.remember(Server{URI: "sftp://one/"})
	store.remember(Server{URI: "sftp://two/"})
	store.forget("sftp://one/")

	list := store.list()
	require.Len(t, list, 1)
	assert.Equal(t, "sftp://two/", list[0].URI)
}
