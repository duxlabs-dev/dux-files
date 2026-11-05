package mimeapps

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/portal"
	"github.com/godbus/dbus/v5"
)

const (
	portalBus    = "org.freedesktop.portal.Desktop"
	portalPath   = "/org/freedesktop/portal/desktop"
	portalOpen   = "org.freedesktop.portal.OpenURI"
	sandboxedMsg = "not available inside the sandbox"
)

func Sandboxed() bool { return portal.InFlatpak() }

func openViaPortal(ctx context.Context, paths []string, ask bool) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	obj := conn.Object(portalBus, portalPath)
	options := map[string]dbus.Variant{
		"ask":      dbus.MakeVariant(ask),
		"writable": dbus.MakeVariant(true),
	}

	for _, path := range paths {
		if err := openOneViaPortal(ctx, obj, path, options); err != nil {
			return err
		}
	}
	return nil
}

func openOneViaPortal(ctx context.Context, obj dbus.BusObject, path string, options map[string]dbus.Variant) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	method := portalOpen + ".OpenFile"
	if info.IsDir() {
		method = portalOpen + ".OpenDirectory"
	}

	var handle dbus.ObjectPath
	return obj.CallWithContext(ctx, method, 0, "", dbus.UnixFD(f.Fd()), options).Store(&handle)
}

func openURIViaPortal(ctx context.Context, uri string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	var handle dbus.ObjectPath
	return conn.Object(portalBus, portalPath).
		CallWithContext(ctx, portalOpen+".OpenURI", 0, "", uri, map[string]dbus.Variant{}).
		Store(&handle)
}

// Needs --talk-name=org.freedesktop.Flatpak in the manifest.
func hostSpawn(argv []string, dir string) error {
	if len(argv) == 0 {
		return errors.New("nothing to run")
	}
	if _, err := exec.LookPath("flatpak-spawn"); err != nil {
		return wire.Fail(files.CodeSandboxed, "flatpak-spawn %s", sandboxedMsg)
	}
	return spawn(append([]string{"flatpak-spawn", "--host", "--directory=" + dir}, argv...), dir, "", false)
}
