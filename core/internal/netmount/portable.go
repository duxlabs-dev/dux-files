package netmount

import (
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
	"github.com/godbus/dbus/v5"
)

const (
	MonitorPath  = dbus.ObjectPath("/org/gtk/Private/RemoteVolumeMonitor")
	MonitorIface = "org.gtk.Private.RemoteVolumeMonitor"
)

var portableMonitors = []string{"org.gtk.vfs.MTPVolumeMonitor", "org.gtk.vfs.GPhoto2VolumeMonitor"}

type wireVolume struct {
	ID              string
	Name            string
	Icon            string
	SymbolicIcon    string
	UUID            string
	ActivationURI   string
	CanMount        bool
	ShouldAutomount bool
	DriveID         string
	MountID         string
	Identifiers     map[string]string
	SortKey         string
	Expansion       map[string]dbus.Variant
}

type Portable struct {
	ID       string `json:"id"`
	Monitor  string `json:"monitor"`
	Name     string `json:"name"`
	IconName string `json:"iconName"`
	Kind     string `json:"kind"`
	Mounted  bool   `json:"mounted"`
}

func (s *Service) Portables() []Portable {
	conn := s.connection()
	if conn == nil {
		return nil
	}

	var out []Portable
	for _, monitor := range portableMonitors {
		for _, volume := range listVolumes(conn, monitor) {
			if !volume.CanMount {
				continue
			}
			out = append(out, Portable{
				ID:       volume.ID,
				Monitor:  monitor,
				Name:     volume.Name,
				IconName: themedIcon(volume.Icon),
				Kind:     KindDevice,
				Mounted:  volume.MountID != "",
			})
		}
	}
	return out
}

func (s *Service) MountPortable(monitor, id string) error {
	conn := s.connection()
	if conn == nil {
		return fail(files.CodeNotSupported, "gvfs is not running")
	}
	if !knownMonitor(monitor) {
		return fail(files.CodeInvalid, "unknown volume monitor: %s", monitor)
	}

	call := conn.Object(monitor, MonitorPath).Call(MonitorIface+".VolumeMount", 0, id, "", uint32(0), "")
	if call.Err != nil && !alreadyMounted(call.Err) {
		return wrapDBus(call.Err)
	}
	s.reload(conn)
	s.publish(s.Snapshot())
	return nil
}

func listVolumes(conn *dbus.Conn, monitor string) []wireVolume {
	var drives []dbus.Variant
	var volumes []wireVolume
	var mounts []dbus.Variant

	err := conn.Object(monitor, MonitorPath).Call(MonitorIface+".List", dbus.FlagNoAutoStart).Store(&drives, &volumes, &mounts)
	switch {
	case err == nil:
		return volumes
	case monitorNotRunning(err):
		return nil
	}
	log.Debugf("network: %s List failed: %v", monitor, err)
	return nil
}

func monitorNotRunning(err error) bool {
	call, ok := err.(dbus.Error)
	if !ok {
		return false
	}
	switch call.Name {
	case "org.freedesktop.DBus.Error.NameHasNoOwner", "org.freedesktop.DBus.Error.ServiceUnknown":
		return true
	}
	return false
}

func knownMonitor(name string) bool {
	for _, monitor := range portableMonitors {
		if monitor == name {
			return true
		}
	}
	return false
}
