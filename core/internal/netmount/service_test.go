package netmount

import (
	"testing"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/dbustest"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTracker struct {
	mounts []wireMount
}

func (f *fakeTracker) ListMountableInfo() ([]Mountable, *dbus.Error) { return mountables, nil }

func (f *fakeTracker) ListMounts() ([]wireMount, *dbus.Error) { return f.mounts, nil }

func (f *fakeTracker) LookupMount(spec wireSpec) (wireMount, *dbus.Error) {
	wanted := decodeSpec(spec)
	for _, mount := range f.mounts {
		if decodeSpec(mount.Spec).Items["type"] == wanted.Items["type"] {
			return mount, nil
		}
	}
	return wireMount{}, &dbus.Error{Name: unmappedGIOError + "16", Body: []any{"The specified location is not mounted"}}
}

func (f *fakeTracker) MountLocation(spec wireSpec, source wireSource) *dbus.Error {
	return &dbus.Error{Name: unmappedGIOError + "17", Body: []any{"Location is already mounted"}}
}

type fakeBrowseMount struct {
	conn    *dbus.Conn
	entries [][]wireAttribute
}

func (f *fakeBrowseMount) Enumerate(sender dbus.Sender, path []byte, objPath string, attributes string, flags uint32, uri string) *dbus.Error {
	go func() {
		listener := f.conn.Object(string(sender), dbus.ObjectPath(objPath))
		_ = listener.Call(EnumeratorIface+".GotInfo", 0, f.entries).Err
		_ = listener.Call(EnumeratorIface+".Done", 0).Err
	}()
	return nil
}

func gvfsEntry(name, comment, target string, fileType uint32) []wireAttribute {
	return []wireAttribute{
		{Name: "standard::name", Value: dbus.MakeVariant(byteStringOf(name))},
		{Name: "standard::display-name", Value: dbus.MakeVariant(name)},
		{Name: "standard::type", Value: dbus.MakeVariant(fileType)},
		{Name: "standard::target-uri", Value: dbus.MakeVariant(target)},
		{Name: "smb::comment", Value: dbus.MakeVariant(comment)},
	}
}

func trackedService(t *testing.T, tracker *fakeTracker) *Service {
	t.Helper()
	address, _ := trackedBus(t, tracker)

	svc := NewService(nil, t.TempDir())
	conn, err := dbus.Connect(address, dbus.WithIncomingInterceptor(svc.intercept))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	svc.conn = conn
	svc.mountables = mountables
	return svc
}

type snapshots chan Snapshot

func (p snapshots) Publish(_ string, data any) {
	if snapshot, ok := data.(Snapshot); ok {
		p <- snapshot
	}
}

func (p snapshots) next(t *testing.T) Snapshot {
	t.Helper()
	select {
	case snapshot := <-p:
		return snapshot
	case <-time.After(5 * time.Second):
		t.Fatal("no mounts snapshot published")
		return Snapshot{}
	}
}

func trackedBus(t *testing.T, tracker *fakeTracker) (string, *dbus.Conn) {
	t.Helper()
	address := dbustest.Bus(t)
	return address, ownTracker(t, address, tracker)
}

func ownTracker(t *testing.T, address string, tracker *fakeTracker) *dbus.Conn {
	t.Helper()
	daemon := dbustest.Connect(t, address)
	require.NoError(t, daemon.Export(tracker, TrackerPath, TrackerIface))
	for i, mount := range tracker.mounts {
		if mount.UserVisible {
			continue
		}
		tracker.mounts[i].DBusID = daemon.Names()[0]
		browse := &fakeBrowseMount{conn: daemon, entries: [][]wireAttribute{
			gvfsEntry("media", "Music and video", "smb://example/media", gioFileTypeMountable),
			gvfsEntry("Back up", "", "smb://wrong-host/Back%20up", gioFileTypeMountable),
			gvfsEntry("WORKGROUP", "", "smb://WORKGROUP/", 5),
		}}
		require.NoError(t, daemon.Export(browse, mount.ObjectPath, MountIface))
	}
	reply, err := daemon.RequestName(VfsBus, dbus.NameFlagDoNotQueue)
	require.NoError(t, err)
	require.Equal(t, dbus.RequestNameReplyPrimaryOwner, reply)
	return daemon
}

func gvfsMount(uri string, visible bool) wireMount {
	mount := fuselessMount(uri, visible)
	if visible {
		mount.FuseMountpoint = byteStringOf("/run/user/1000/gvfs/ftp:host=example")
	}
	return mount
}

func fuselessMount(uri string, visible bool) wireMount {
	spec, err := ParseURI(uri, mountables)
	if err != nil {
		panic(err)
	}
	return wireMount{
		DBusID:      ":1.9",
		ObjectPath:  "/org/gtk/vfs/mount/1",
		DisplayName: "probe",
		StableName:  "ftp:host=example",
		Icon:        ". GThemedIcon folder-remote folder",
		UserVisible: visible,
		Spec:        encodeSpec(spec),
	}
}

func TestConnectReusesAMountGvfsAlreadyHolds(t *testing.T) {
	tracker := &fakeTracker{mounts: []wireMount{gvfsMount("ftp://example/", true)}}
	svc := trackedService(t, tracker)

	connected, err := svc.Connect("ftp://example/pub", "")
	require.NoError(t, err)
	assert.Equal(t, "ftp:host=example", connected.Mount.ID)
	assert.Equal(t, "/run/user/1000/gvfs/ftp:host=example/pub", connected.Location)
}

func TestConnectListsTheSharesOfABareSMBServer(t *testing.T) {
	tracker := &fakeTracker{mounts: []wireMount{gvfsMount("smb://example/", false)}}
	svc := trackedService(t, tracker)

	connected, err := svc.Connect("smb://example", "")
	require.NoError(t, err)
	assert.Nil(t, connected.Mount)
	assert.Equal(t, "smb://example/", connected.Server)
	assert.Equal(t, []Share{
		{Name: "Back up", URI: "smb://example/Back%20up"},
		{Name: "media", URI: "smb://example/media", Comment: "Music and video"},
	}, connected.Shares)
}

func TestDaemonRestartDropsStaleMountsAndReloads(t *testing.T) {
	address, daemon := trackedBus(t, &fakeTracker{mounts: []wireMount{fuselessMount("ftp://example/", true)}})
	published := make(snapshots, 8)
	svc := NewService(published, t.TempDir())
	conn, err := dbus.Connect(address, dbus.WithIncomingInterceptor(svc.intercept))
	require.NoError(t, err)
	svc.run(conn)
	t.Cleanup(svc.Close)

	before := svc.Snapshot()
	require.True(t, before.Available)
	require.Len(t, before.Mounts, 1)
	assert.Empty(t, before.Mounts[0].Path)

	require.NoError(t, daemon.Close())
	gone := published.next(t)
	assert.False(t, gone.Available)
	assert.Equal(t, "gvfs is not running", gone.Reason)
	assert.Empty(t, gone.Mounts)
	assert.Empty(t, gone.Schemes)

	ownTracker(t, address, &fakeTracker{mounts: []wireMount{gvfsMount("ftp://example/", true)}})
	back := published.next(t)
	assert.True(t, back.Available)
	require.Len(t, back.Mounts, 1)
	assert.Equal(t, "/run/user/1000/gvfs/ftp:host=example", back.Mounts[0].Path)

	connected, err := svc.Connect("ftp://example/pub", "")
	require.NoError(t, err)
	assert.Equal(t, "/run/user/1000/gvfs/ftp:host=example/pub", connected.Location)
}
