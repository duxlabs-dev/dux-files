package devices

import (
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/ipctest"

	"github.com/duxlabs-dev/dux-go/ipc"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedCall struct {
	Path   dbus.ObjectPath
	Method string
	Args   []any
}

type fakeBus struct {
	calls    []recordedCall
	body     []any
	err      error
	failFrom int
}

type fakeObject struct {
	dbus.BusObject
	bus  *fakeBus
	path dbus.ObjectPath
}

func (b *fakeBus) Object(_ string, path dbus.ObjectPath) dbus.BusObject {
	return &fakeObject{bus: b, path: path}
}

func (o *fakeObject) Call(method string, _ dbus.Flags, args ...any) *dbus.Call {
	o.bus.calls = append(o.bus.calls, recordedCall{Path: o.path, Method: method, Args: args})
	if len(o.bus.calls) <= o.bus.failFrom {
		return &dbus.Call{Body: o.bus.body}
	}
	return &dbus.Call{Err: o.bus.err, Body: o.bus.body}
}

func testService(t *testing.T, bus *fakeBus) *Service {
	t.Helper()
	svc := NewService(nil, testUID)
	svc.objects = table()
	svc.available = true
	svc.reason = ""
	if bus != nil {
		svc.conn = bus
	}
	return svc
}

func call(t *testing.T, svc *Service, req ipc.Request) map[string]any {
	t.Helper()
	return ipctest.Call(t, func(w *ipc.ConnWriter) { svc.Handle(w, req) })
}

func TestHandleListReturnsTheSnapshot(t *testing.T) {
	out := call(t, testService(t, nil), ipc.Request{ID: 3, Method: "devices.list"})

	result := out["result"].(map[string]any)
	assert.Equal(t, true, result["available"])
	assert.Len(t, result["devices"], 3)
}

func TestHandleMountCallsUdisksAndReturnsTheMountPoint(t *testing.T) {
	bus := &fakeBus{body: []any{"/run/media/brandon/PHOTOS"}}
	out := call(t, testService(t, bus), ipc.Request{ID: 4, Method: "devices.mount", Params: map[string]any{"id": "/block/stick"}})

	result := out["result"].(map[string]any)
	assert.Equal(t, "/run/media/brandon/PHOTOS", result["mountPoint"])
	require.Len(t, bus.calls, 1)
	assert.Equal(t, dbus.ObjectPath("/block/stick"), bus.calls[0].Path)
	assert.Equal(t, FsIface+".Mount", bus.calls[0].Method)
}

func TestHandleUnmountPassesForceOnlyWhenAsked(t *testing.T) {
	bus := &fakeBus{}
	svc := testService(t, bus)
	svc.objects["/block/stick"][FsIface] = map[string]dbus.Variant{"MountPoints": mountPoints("/run/media/brandon/PHOTOS")}

	call(t, svc, ipc.Request{ID: 5, Method: "devices.unmount", Params: map[string]any{"id": "/block/stick"}})
	require.Len(t, bus.calls, 1)
	assert.Equal(t, map[string]dbus.Variant{}, bus.calls[0].Args[0])

	call(t, svc, ipc.Request{ID: 6, Method: "devices.unmount", Params: map[string]any{"id": "/block/stick", "force": true}})
	require.Len(t, bus.calls, 2)
	assert.Equal(t, map[string]dbus.Variant{"force": dbus.MakeVariant(true)}, bus.calls[1].Args[0])
}

func TestHandleEjectUnmountsFirst(t *testing.T) {
	bus := &fakeBus{}
	svc := testService(t, bus)
	svc.objects["/block/stick"][FsIface] = map[string]dbus.Variant{"MountPoints": mountPoints("/run/media/brandon/PHOTOS")}

	out := call(t, svc, ipc.Request{ID: 7, Method: "devices.eject", Params: map[string]any{"id": "/block/stick"}})
	assert.Equal(t, true, out["result"].(map[string]any)["ejected"])

	require.Len(t, bus.calls, 2)
	assert.Equal(t, FsIface+".Unmount", bus.calls[0].Method)
	assert.Equal(t, DriveIface+".Eject", bus.calls[1].Method)
	assert.Equal(t, dbus.ObjectPath("/drives/stick"), bus.calls[1].Path)
}

func TestHandleUnlockRefusesPlainDevices(t *testing.T) {
	out := call(t, testService(t, &fakeBus{}), ipc.Request{ID: 8, Method: "devices.unlock", Params: map[string]any{"id": "/block/stick", "passphrase": "hunter2"}})

	assert.Equal(t, "EINVAL", out["code"])
	assert.Contains(t, out["error"], "not encrypted")
}

func TestHandleUnlockPassesThePassphraseAndMountsOnRequest(t *testing.T) {
	bus := &fakeBus{body: []any{dbus.ObjectPath("/block/clear")}}
	svc := testService(t, bus)
	svc.objects["/block/luks"] = map[string]map[string]dbus.Variant{
		BlockIface:  props(map[string]any{"Device": append([]byte("/dev/sdc1"), 0), "IdLabel": "VAULT", "Size": uint64(1 << 35)}),
		CryptoIface: props(map[string]any{}),
	}

	out := call(t, svc, ipc.Request{ID: 9, Method: "devices.unlock", Params: map[string]any{"id": "/block/luks", "passphrase": "hunter2", "mount": true}})
	result := out["result"].(map[string]any)
	assert.Equal(t, "/block/clear", result["cleartext"])

	require.Len(t, bus.calls, 2)
	assert.Equal(t, CryptoIface+".Unlock", bus.calls[0].Method)
	assert.Equal(t, "hunter2", bus.calls[0].Args[0])
	assert.Equal(t, dbus.ObjectPath("/block/clear"), bus.calls[1].Path)
	assert.Equal(t, FsIface+".Mount", bus.calls[1].Method)
}

func TestHandleReportsUdisksErrors(t *testing.T) {
	bus := &fakeBus{err: dbus.Error{Name: BusName + ".Error.DeviceBusy", Body: []any{"target is busy"}}}
	svc := testService(t, bus)
	svc.objects["/block/stick"][FsIface] = map[string]dbus.Variant{"MountPoints": mountPoints("/run/media/brandon/PHOTOS")}

	out := call(t, svc, ipc.Request{ID: 10, Method: "devices.unmount", Params: map[string]any{"id": "/block/stick"}})
	assert.Equal(t, "EBUSY", out["code"])
	assert.Contains(t, out["error"], "target is busy")
}

func TestHandleRejectsMissingParams(t *testing.T) {
	svc := testService(t, nil)

	out := call(t, svc, ipc.Request{ID: 11, Method: "devices.mount"})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 12, Method: "devices.unlock", Params: map[string]any{"id": "/block/stick"}})
	assert.Equal(t, "EINVAL", out["code"])

	out = call(t, svc, ipc.Request{ID: 13, Method: "devices.nope"})
	assert.Equal(t, "NOTSUPPORTED", out["code"])
}

func TestHandleReportsAMissingDaemon(t *testing.T) {
	out := call(t, testService(t, nil), ipc.Request{ID: 14, Method: "devices.mount", Params: map[string]any{"id": "/block/stick"}})
	assert.Equal(t, "NOTSUPPORTED", out["code"])
	assert.Contains(t, out["error"], "udisks2 is not running")
}

func TestHandleUsageAnswersForMountedDevices(t *testing.T) {
	out := call(t, testService(t, nil), ipc.Request{ID: 15, Method: "devices.usage", Params: map[string]any{"paths": []any{t.TempDir()}}})

	usage := out["result"].(map[string]any)["usage"].([]any)
	require.Len(t, usage, 1)
	assert.Greater(t, usage[0].(map[string]any)["total"], float64(0))
}

func TestHandleUnlockReportsAMountFailureWithoutLosingTheUnlock(t *testing.T) {
	bus := &fakeBus{body: []any{dbus.ObjectPath("/block/clear")}}
	svc := testService(t, bus)
	svc.objects["/block/luks"] = map[string]map[string]dbus.Variant{
		BlockIface:  props(map[string]any{"Device": append([]byte("/dev/sdc1"), 0), "IdLabel": "VAULT", "Size": uint64(1 << 35)}),
		CryptoIface: props(map[string]any{}),
	}

	bus.err = dbus.Error{Name: "org.freedesktop.DBus.Error.UnknownMethod", Body: []any{"no such interface"}}
	bus.failFrom = 1

	out := call(t, svc, ipc.Request{ID: 16, Method: "devices.unlock", Params: map[string]any{"id": "/block/luks", "passphrase": "hunter2", "mount": true}})

	result := out["result"].(map[string]any)
	assert.Equal(t, "NOTSUPPORTED", result["mountCode"])
	assert.NotContains(t, result, "mountPoint")
}

func TestHandleRefusesToDetachProtectedMounts(t *testing.T) {
	for _, method := range []string{"devices.unmount", "devices.eject", "devices.powerOff", "devices.lock"} {
		for _, point := range []string{"/", "/home", "/boot"} {
			t.Run(method+point, func(t *testing.T) {
				bus := &fakeBus{}
				svc := testService(t, bus)
				svc.objects["/block/stick"][FsIface]["MountPoints"] = mountPoints(point)
				out := call(t, svc, ipc.Request{ID: 17, Method: method, Params: map[string]any{"id": "/block/stick", "force": true}})
				assert.Equal(t, "EPERM", out["code"])
				assert.Empty(t, bus.calls)
			})
		}
	}
}

func TestHandleRefusesToEjectDrivesWithProtectedIgnoredPartitions(t *testing.T) {
	for _, method := range []string{"devices.eject", "devices.powerOff"} {
		t.Run(method, func(t *testing.T) {
			bus := &fakeBus{}
			svc := testService(t, bus)
			svc.objects["/block/efi"][BlockIface]["Drive"] = dbus.MakeVariant(dbus.ObjectPath("/drives/stick"))
			out := call(t, svc, ipc.Request{ID: 18, Method: method, Params: map[string]any{"id": "/block/stick"}})
			assert.Equal(t, "EPERM", out["code"])
			assert.Empty(t, bus.calls)
		})
	}
}

func TestHandleRefusesToLockEncryptedHome(t *testing.T) {
	bus := &fakeBus{}
	svc := testService(t, bus)
	svc.objects["/block/stick"][CryptoIface] = props(map[string]any{})
	svc.objects["/block/clear"] = map[string]map[string]dbus.Variant{
		BlockIface: props(map[string]any{"CryptoBackingDevice": dbus.ObjectPath("/block/stick")}),
		FsIface:    {"MountPoints": mountPoints("/home")},
	}
	out := call(t, svc, ipc.Request{ID: 19, Method: "devices.lock", Params: map[string]any{"id": "/block/stick"}})
	assert.Equal(t, "EPERM", out["code"])
	assert.Empty(t, bus.calls)
}
