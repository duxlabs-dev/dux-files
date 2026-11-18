package devices

import (
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
)

var noOptions = map[string]dbus.Variant{}

func (s *Service) Mount(id string) (string, error) {
	device, err := s.mountable(id)
	if err != nil {
		return "", err
	}
	if device.Mounted {
		return device.MountPoint, nil
	}

	target := mountTarget(device)
	var mountPoint string
	if err := s.call(target, FsIface+".Mount", noOptions).Store(&mountPoint); err != nil {
		return "", wrapDBus(err)
	}
	s.usage.forget(mountPoint)
	return mountPoint, nil
}

func (s *Service) Unmount(id string, force bool) error {
	device, err := s.mountable(id)
	if err != nil {
		return err
	}
	if !device.Mounted {
		return nil
	}

	if err := s.checkDetach(device, false); err != nil {
		return err
	}

	options := noOptions
	if force {
		options = map[string]dbus.Variant{"force": dbus.MakeVariant(true)}
	}
	if err := s.call(mountTarget(device), FsIface+".Unmount", options).Store(); err != nil {
		return wrapDBus(err)
	}
	s.usage.forget(device.MountPoint)
	return nil
}

func (s *Service) Eject(id string) error {
	device, found := s.find(id)
	if !found {
		return wire.Fail(files.CodeNotFound, "unknown device: %s", id)
	}
	if err := s.checkDetach(device, true); err != nil {
		return err
	}
	if device.Drive == "" {
		return wire.Fail(files.CodeNotSupported, "%s has no drive to eject", device.Name)
	}
	if device.Mounted {
		if err := s.Unmount(id, false); err != nil {
			return err
		}
	}

	if device.Ejectable {
		return wrapDBus(s.call(dbus.ObjectPath(device.Drive), DriveIface+".Eject", noOptions).Store())
	}
	if !device.CanPowerOff {
		return nil
	}
	return s.PowerOff(id)
}

func (s *Service) PowerOff(id string) error {
	device, found := s.find(id)
	if !found {
		return wire.Fail(files.CodeNotFound, "unknown device: %s", id)
	}
	if err := s.checkDetach(device, true); err != nil {
		return err
	}
	if device.Drive == "" || !device.CanPowerOff {
		return wire.Fail(files.CodeNotSupported, "%s cannot be powered off", device.Name)
	}
	return wrapDBus(s.call(dbus.ObjectPath(device.Drive), DriveIface+".PowerOff", noOptions).Store())
}

func (s *Service) Unlock(id, passphrase string) (string, error) {
	device, found := s.find(id)
	if !found {
		return "", wire.Fail(files.CodeNotFound, "unknown device: %s", id)
	}
	if !device.Encrypted {
		return "", wire.Fail(files.CodeInvalid, "%s is not encrypted", device.Name)
	}
	if device.Unlocked {
		return device.Cleartext, nil
	}

	var cleartext dbus.ObjectPath
	if err := s.call(dbus.ObjectPath(id), CryptoIface+".Unlock", passphrase, noOptions).Store(&cleartext); err != nil {
		return "", wrapDBus(err)
	}
	return string(cleartext), nil
}

func (s *Service) Lock(id string) error {
	device, found := s.find(id)
	if !found {
		return wire.Fail(files.CodeNotFound, "unknown device: %s", id)
	}
	if err := s.checkDetach(device, false); err != nil {
		return err
	}
	if !device.Encrypted {
		return wire.Fail(files.CodeInvalid, "%s is not encrypted", device.Name)
	}
	if !device.Unlocked {
		return nil
	}
	if device.Mounted {
		if err := s.Unmount(id, false); err != nil {
			return err
		}
	}
	return wrapDBus(s.call(dbus.ObjectPath(id), CryptoIface+".Lock", noOptions).Store())
}

func (s *Service) Usage(mountPoints []string, force bool) []Usage {
	if len(mountPoints) == 0 {
		for _, device := range s.Snapshot().Devices {
			if device.Mounted {
				mountPoints = append(mountPoints, device.MountPoints...)
			}
		}
	}

	out := make([]Usage, 0, len(mountPoints))
	for _, mountPoint := range mountPoints {
		out = append(out, s.usage.get(mountPoint, force))
	}
	return out
}

func (s *Service) mountable(id string) (Device, error) {
	device, found := s.find(id)
	switch {
	case !found:
		return Device{}, wire.Fail(files.CodeNotFound, "unknown device: %s", id)
	case device.Encrypted && !device.Unlocked:
		return Device{}, wire.Fail(files.CodeInvalid, "%s is locked", device.Name)
	}
	return device, nil
}

func (s *Service) checkDetach(device Device, wholeDrive bool) error {
	if device.Protected {
		return wire.Fail(CodePerm, "%s contains a protected system mount", device.Device)
	}
	if !wholeDrive || device.Drive == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	cleartext := cleartextByBacking(s.objects)
	for path, ifaces := range s.objects {
		if objectOf(ifaces[BlockIface]["Drive"]) != device.Drive {
			continue
		}
		if build(s.objects, path, ifaces, cleartext[path]).Protected {
			return wire.Fail(CodePerm, "%s contains a protected system mount", device.Drive)
		}
	}
	return nil
}

func mountTarget(device Device) dbus.ObjectPath {
	if device.Cleartext != "" {
		return dbus.ObjectPath(device.Cleartext)
	}
	return dbus.ObjectPath(device.ID)
}

func (s *Service) call(path dbus.ObjectPath, method string, args ...any) *dbus.Call {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return &dbus.Call{Err: dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown", Body: []any{"udisks2 is not running"}}}
	}
	return conn.Object(BusName, path).Call(method, 0, args...)
}
