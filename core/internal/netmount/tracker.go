package netmount

import (
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	"github.com/godbus/dbus/v5"
)

const (
	VfsBus       = "org.gtk.vfs.Daemon"
	TrackerPath  = dbus.ObjectPath("/org/gtk/vfs/mounttracker")
	TrackerIface = "org.gtk.vfs.MountTracker"
	MountIface   = "org.gtk.vfs.Mount"
	OpIface      = "org.gtk.vfs.MountOperation"
	OpRoot       = "/dev/duxlabs/files/mountop/"
)

const (
	KindNetwork = "network"
	KindDevice  = "device"
	KindOther   = "other"
)

type wireSpec struct {
	Prefix []byte
	Items  map[string]dbus.Variant
}

type wireMount struct {
	DBusID           string
	ObjectPath       dbus.ObjectPath
	DisplayName      string
	StableName       string
	XContentTypes    string
	Icon             string
	SymbolicIcon     string
	FilenameEncoding string
	UserVisible      bool
	FuseMountpoint   []byte
	Spec             wireSpec
	DefaultLocation  []byte
}

type wireSource struct {
	DBusID string
	Path   dbus.ObjectPath
}

type Mount struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URI        string `json:"uri"`
	Path       string `json:"path,omitempty"`
	Scheme     string `json:"scheme"`
	Kind       string `json:"kind"`
	IconName   string `json:"iconName"`
	Kernel     bool   `json:"kernel,omitempty"`
	Configured bool   `json:"configured,omitempty"`
	Mounted    bool   `json:"mounted"`

	dbusID string
	object dbus.ObjectPath
}

func decodeMount(raw wireMount, mountables []Mountable) Mount {
	spec := decodeSpec(raw.Spec)
	scheme := SchemeOf(spec, mountables)
	return Mount{
		ID:       raw.StableName,
		Name:     raw.DisplayName,
		URI:      URIOf(spec, mountables),
		Path:     dbusutil.ByteString(raw.FuseMountpoint),
		Scheme:   scheme,
		Kind:     kindOf(spec.Type()),
		IconName: themedIcon(raw.Icon),
		Mounted:  true,
		dbusID:   raw.DBusID,
		object:   raw.ObjectPath,
	}
}

func decodeSpec(raw wireSpec) Spec {
	spec := Spec{Prefix: dbusutil.ByteString(raw.Prefix), Items: map[string]string{}}
	for key, value := range raw.Items {
		bytes, ok := value.Value().([]byte)
		if !ok {
			continue
		}
		spec.Items[key] = dbusutil.ByteString(bytes)
	}
	return spec
}

func encodeSpec(spec Spec) wireSpec {
	items := make(map[string]dbus.Variant, len(spec.Items))
	for key, value := range spec.Items {
		items[key] = dbus.MakeVariant(byteStringOf(value))
	}
	return wireSpec{Prefix: byteStringOf(spec.Prefix), Items: items}
}

func kindOf(mountType string) string {
	switch mountType {
	case "mtp", "gphoto2", "cdda", "afc":
		return KindDevice
	case "sftp", "ssh", "smb-share", "smb-server", "smb-browse", "ftp", "ftps", "ftpis",
		"dav", "davs", "dav+sd", "nfs", "afp-volume", "afp-server", "http", "https":
		return KindNetwork
	}
	return KindOther
}

// A serialised GThemedIcon reads ". GThemedIcon folder-remote folder folder-remote-symbolic".
func themedIcon(serialised string) string {
	fields := strings.Fields(serialised)
	if len(fields) < 3 || fields[1] != "GThemedIcon" {
		return "folder-remote"
	}
	return fields[2]
}

func byteStringOf(value string) []byte {
	return append([]byte(value), 0)
}
