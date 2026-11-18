package devices

import (
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	"github.com/godbus/dbus/v5"
)

const (
	BusName     = "org.freedesktop.UDisks2"
	RootPath    = dbus.ObjectPath("/org/freedesktop/UDisks2")
	BlockIface  = BusName + ".Block"
	FsIface     = BusName + ".Filesystem"
	CryptoIface = BusName + ".Encrypted"
	DriveIface  = BusName + ".Drive"
	LoopIface   = BusName + ".Loop"
)

const (
	KindSystem    = "system"
	KindInternal  = "internal"
	KindRemovable = "removable"
	KindOptical   = "optical"
	KindLoop      = "loop"
)

type Objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

type Device struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Device      string   `json:"device"`
	Drive       string   `json:"drive,omitempty"`
	Label       string   `json:"label,omitempty"`
	Fstype      string   `json:"fstype,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
	Size        uint64   `json:"size"`
	Removable   bool     `json:"removable"`
	Ejectable   bool     `json:"ejectable"`
	CanPowerOff bool     `json:"canPowerOff"`
	Encrypted   bool     `json:"encrypted"`
	Unlocked    bool     `json:"unlocked"`
	Cleartext   string   `json:"cleartext,omitempty"`
	Mounted     bool     `json:"mounted"`
	MountPoint  string   `json:"mountPoint,omitempty"`
	MountPoints []string `json:"mountPoints,omitempty"`
	IconName    string   `json:"iconName"`
	System      bool     `json:"system"`
	Protected   bool     `json:"protected"`
	Hidden      bool     `json:"hidden,omitempty"`
}

func List(objects Objects, uid uint32) []Device {
	cleartext := cleartextByBacking(objects)
	out := make([]Device, 0, len(objects))

	for path, ifaces := range objects {
		block, ok := ifaces[BlockIface]
		if !ok {
			continue
		}
		if boolOf(block["HintIgnore"]) || objectOf(block["CryptoBackingDevice"]) != "" {
			continue
		}
		_, hasFs := ifaces[FsIface]
		_, hasCrypto := ifaces[CryptoIface]
		if !hasFs && !hasCrypto {
			continue
		}

		device := build(objects, path, ifaces, cleartext[path])
		if !visible(device, ifaces, uid) {
			continue
		}
		device.Hidden = systemInternal(device, ifaces)
		out = append(out, device)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i]) != rank(out[j]) {
			return rank(out[i]) < rank(out[j])
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func build(objects Objects, path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant, unlocked dbus.ObjectPath) Device {
	block := ifaces[BlockIface]
	drivePath := objectOf(block["Drive"])
	drive := objects[dbus.ObjectPath(drivePath)][DriveIface]

	device := Device{
		ID:          string(path),
		Device:      byteStringOf(block["Device"]),
		Drive:       drivePath,
		Label:       stringOf(block["IdLabel"]),
		Fstype:      stringOf(block["IdType"]),
		UUID:        stringOf(block["IdUUID"]),
		Size:        uint64Of(block["Size"]),
		Removable:   boolOf(drive["Removable"]) || boolOf(drive["MediaRemovable"]),
		Ejectable:   boolOf(drive["Ejectable"]),
		CanPowerOff: boolOf(drive["CanPowerOff"]),
		Encrypted:   hasIface(ifaces, CryptoIface),
	}

	mountSource := ifaces
	if unlocked != "" {
		device.Unlocked = true
		device.Cleartext = string(unlocked)
		mountSource = objects[unlocked]
		if inner := mountSource[BlockIface]; inner != nil {
			device.Fstype = stringOf(inner["IdType"])
		}
	}
	device.MountPoints = allMountPoints(mountSource[FsIface])
	device.MountPoint = preferredMountPoint(device.MountPoints)
	device.Mounted = device.MountPoint != ""
	device.System = device.MountPoint == "/"
	device.Protected = protectedMounts(device.MountPoints)

	device.Kind = kindOf(device, ifaces, drive)
	device.Name = nameOf(device, block, drive)
	device.IconName = iconOf(device, drive)
	return device
}

func protectedMounts(points []string) bool {
	for _, point := range points {
		point = filepath.Clean(point)
		if point == "/" {
			return true
		}
		base, _, _ := strings.Cut(strings.TrimPrefix(point, "/"), "/")
		switch base {
		case "home", "root", "boot", "efi", "usr", "var", "etc", "bin", "sbin", "lib", "lib32", "lib64", "opt":
			return true
		}
	}
	return false
}

func kindOf(device Device, ifaces map[string]map[string]dbus.Variant, drive map[string]dbus.Variant) string {
	switch {
	case device.System:
		return KindSystem
	case hasIface(ifaces, LoopIface):
		return KindLoop
	case stringOf(drive["Media"]) != "" && strings.HasPrefix(stringOf(drive["Media"]), "optical"):
		return KindOptical
	case device.Removable:
		return KindRemovable
	default:
		return KindInternal
	}
}

func nameOf(device Device, block, drive map[string]dbus.Variant) string {
	switch {
	case device.Label != "":
		return device.Label
	case stringOf(block["HintName"]) != "":
		return stringOf(block["HintName"])
	}

	model := strings.TrimSpace(stringOf(drive["Vendor"]) + " " + stringOf(drive["Model"]))
	if model != "" {
		return model
	}
	return HumanSize(device.Size)
}

func iconOf(device Device, drive map[string]dbus.Variant) string {
	if hinted := stringOf(drive["MediaIconName"]); hinted != "" && device.Kind == KindOptical {
		return hinted
	}
	switch device.Kind {
	case KindSystem:
		return "drive-harddisk-system"
	case KindOptical:
		return "media-optical"
	case KindLoop:
		return "drive-harddisk"
	case KindRemovable:
		if stringOf(drive["ConnectionBus"]) == "usb" {
			return "drive-removable-media-usb"
		}
		return "drive-removable-media"
	default:
		return "drive-harddisk"
	}
}

func visible(device Device, ifaces map[string]map[string]dbus.Variant, uid uint32) bool {
	switch {
	case device.System:
		return true
	case hasIface(ifaces, LoopIface):
		return uint32Of(ifaces[LoopIface]["SetupByUID"]) == uid
	case device.Size == 0 && !device.Encrypted:
		return false
	default:
		return true
	}
}

func systemInternal(device Device, ifaces map[string]map[string]dbus.Variant) bool {
	block := ifaces[BlockIface]
	return !device.System && boolOf(block["HintSystem"]) && !boolOf(block["HintAuto"])
}

func rank(device Device) int {
	switch device.Kind {
	case KindSystem:
		return 0
	case KindInternal:
		return 1
	case KindLoop:
		return 2
	case KindOptical:
		return 3
	default:
		return 4
	}
}

func cleartextByBacking(objects Objects) map[dbus.ObjectPath]dbus.ObjectPath {
	out := map[dbus.ObjectPath]dbus.ObjectPath{}
	for path, ifaces := range objects {
		backing := objectOf(ifaces[BlockIface]["CryptoBackingDevice"])
		if backing == "" {
			continue
		}
		out[dbus.ObjectPath(backing)] = path
	}
	return out
}

func preferredMountPoint(points []string) string {
	switch {
	case len(points) == 0:
		return ""
	case slices.Contains(points, "/"):
		return "/"
	}
	return points[0]
}

func allMountPoints(fs map[string]dbus.Variant) []string {
	points, ok := fs["MountPoints"].Value().([][]byte)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(points))
	for _, point := range points {
		if path := dbusutil.ByteString(point); path != "" {
			out = append(out, path)
		}
	}
	return out
}

func hasIface(ifaces map[string]map[string]dbus.Variant, name string) bool {
	_, ok := ifaces[name]
	return ok
}

func HumanSize(size uint64) string {
	units := []string{"B", "kB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	return strconv.FormatFloat(value, 'f', -1, 64) + " " + units[unit]
}
