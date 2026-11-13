package devices

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testUID = 1000

func props(pairs map[string]any) map[string]dbus.Variant {
	out := map[string]dbus.Variant{}
	for key, value := range pairs {
		out[key] = dbus.MakeVariant(value)
	}
	return out
}

func mountPoints(points ...string) dbus.Variant {
	raw := make([][]byte, 0, len(points))
	for _, point := range points {
		raw = append(raw, append([]byte(point), 0))
	}
	return dbus.MakeVariant(raw)
}

func table() Objects {
	return Objects{
		"/drives/nvme": {
			DriveIface: props(map[string]any{"Vendor": "ACME", "Model": "SSD", "Removable": false}),
		},
		"/drives/stick": {
			DriveIface: props(map[string]any{
				"Vendor": "Kingston", "Model": "DataTraveler", "Removable": true,
				"Ejectable": true, "CanPowerOff": true, "ConnectionBus": "usb",
			}),
		},
		"/block/root": {
			BlockIface: props(map[string]any{
				"Device": append([]byte("/dev/nvme0n1p3"), 0), "Drive": dbus.ObjectPath("/drives/nvme"),
				"IdType": "ext4", "IdUUID": "root-uuid", "Size": uint64(1 << 40),
				"HintSystem": true, "HintAuto": false, "HintIgnore": false,
			}),
			FsIface: map[string]dbus.Variant{"MountPoints": mountPoints("/")},
		},
		"/block/home": {
			BlockIface: props(map[string]any{
				"Device": append([]byte("/dev/nvme0n1p5"), 0), "Drive": dbus.ObjectPath("/drives/nvme"),
				"IdType": "ext4", "Size": uint64(1 << 40),
				"HintSystem": true, "HintAuto": false, "HintIgnore": false,
			}),
			FsIface: map[string]dbus.Variant{"MountPoints": mountPoints("/home")},
		},
		"/block/efi": {
			BlockIface: props(map[string]any{
				"Device": append([]byte("/dev/nvme0n1p1"), 0), "Drive": dbus.ObjectPath("/drives/nvme"),
				"IdType": "vfat", "Size": uint64(1 << 29), "HintIgnore": true,
			}),
			FsIface: map[string]dbus.Variant{"MountPoints": mountPoints("/boot/efi")},
		},
		"/block/stick": {
			BlockIface: props(map[string]any{
				"Device": append([]byte("/dev/sdb1"), 0), "Drive": dbus.ObjectPath("/drives/stick"),
				"IdLabel": "PHOTOS", "IdType": "vfat", "Size": uint64(1 << 35),
			}),
			FsIface: map[string]dbus.Variant{"MountPoints": mountPoints()},
		},
	}
}

func byID(devices []Device, id string) (Device, bool) {
	for _, device := range devices {
		if device.ID == id {
			return device, true
		}
	}
	return Device{}, false
}

func TestListHidesIgnoredAndFlagsSystemInternalVolumes(t *testing.T) {
	list := List(table(), testUID)

	ids := make([]string, 0, len(list))
	hidden := map[string]bool{}
	for _, device := range list {
		ids = append(ids, device.ID)
		hidden[device.ID] = device.Hidden
	}
	assert.Equal(t, []string{"/block/root", "/block/home", "/block/stick"}, ids)
	assert.Equal(t, map[string]bool{"/block/home": true, "/block/root": false, "/block/stick": false}, hidden)
}

func TestListMarksTheRootFilesystemAsTheSystemDevice(t *testing.T) {
	root, ok := byID(List(table(), testUID), "/block/root")
	require.True(t, ok)

	assert.Equal(t, KindSystem, root.Kind)
	assert.True(t, root.System)
	assert.True(t, root.Mounted)
	assert.Equal(t, "/", root.MountPoint)
	assert.Equal(t, "ACME SSD", root.Name)
	assert.Equal(t, "drive-harddisk-system", root.IconName)
}

func TestListKeepsEveryMountPointAndFindsRoot(t *testing.T) {
	objects := table()
	objects["/block/root"][FsIface]["MountPoints"] = mountPoints("/home", "/", "/var")

	root, ok := byID(List(objects, testUID), "/block/root")
	require.True(t, ok)
	assert.Equal(t, []string{"/home", "/", "/var"}, root.MountPoints)
	assert.Equal(t, "/", root.MountPoint)
	assert.True(t, root.System)
}

func TestListDescribesRemovableMedia(t *testing.T) {
	stick, ok := byID(List(table(), testUID), "/block/stick")
	require.True(t, ok)

	assert.Equal(t, KindRemovable, stick.Kind)
	assert.Equal(t, "PHOTOS", stick.Name)
	assert.True(t, stick.Removable)
	assert.True(t, stick.Ejectable)
	assert.True(t, stick.CanPowerOff)
	assert.False(t, stick.Mounted)
	assert.Equal(t, "drive-removable-media-usb", stick.IconName)
}

func TestListNamesUnlabelledVolumesAfterTheirDrive(t *testing.T) {
	objects := table()
	delete(objects["/block/stick"][BlockIface], "IdLabel")

	stick, ok := byID(List(objects, testUID), "/block/stick")
	require.True(t, ok)
	assert.Equal(t, "Kingston DataTraveler", stick.Name)
}

func TestListFoldsAnUnlockedContainerIntoOneDevice(t *testing.T) {
	objects := table()
	objects["/block/luks"] = map[string]map[string]dbus.Variant{
		BlockIface:  props(map[string]any{"Device": append([]byte("/dev/sdc1"), 0), "IdLabel": "VAULT", "IdType": "crypto_LUKS", "Size": uint64(1 << 35)}),
		CryptoIface: props(map[string]any{}),
	}
	objects["/block/clear"] = map[string]map[string]dbus.Variant{
		BlockIface: props(map[string]any{"Device": append([]byte("/dev/dm-0"), 0), "IdType": "ext4", "CryptoBackingDevice": dbus.ObjectPath("/block/luks"), "Size": uint64(1 << 35)}),
		FsIface:    {"MountPoints": mountPoints("/run/media/brandon/VAULT")},
	}

	list := List(objects, testUID)
	_, cleartextListed := byID(list, "/block/clear")
	assert.False(t, cleartextListed, "the cleartext device is folded into its container")

	vault, ok := byID(list, "/block/luks")
	require.True(t, ok)
	assert.True(t, vault.Encrypted)
	assert.True(t, vault.Unlocked)
	assert.Equal(t, "/block/clear", vault.Cleartext)
	assert.Equal(t, "/run/media/brandon/VAULT", vault.MountPoint)
	assert.Equal(t, "ext4", vault.Fstype)
}

func TestListKeepsALockedContainer(t *testing.T) {
	objects := table()
	objects["/block/luks"] = map[string]map[string]dbus.Variant{
		BlockIface:  props(map[string]any{"Device": append([]byte("/dev/sdc1"), 0), "IdLabel": "VAULT", "IdType": "crypto_LUKS", "Size": uint64(1 << 35)}),
		CryptoIface: props(map[string]any{}),
	}

	vault, ok := byID(List(objects, testUID), "/block/luks")
	require.True(t, ok)
	assert.True(t, vault.Encrypted)
	assert.False(t, vault.Unlocked)
	assert.False(t, vault.Mounted)
}

func TestListShowsOnlyThisUsersLoopDevices(t *testing.T) {
	objects := table()
	objects["/block/loop0"] = map[string]map[string]dbus.Variant{
		BlockIface: props(map[string]any{"Device": append([]byte("/dev/loop0"), 0), "IdType": "iso9660", "Size": uint64(1 << 26), "HintSystem": true}),
		FsIface:    {"MountPoints": mountPoints()},
		LoopIface:  props(map[string]any{"SetupByUID": uint32(testUID), "BackingFile": append([]byte("/home/user/archlinux.iso"), 0)}),
	}
	objects["/block/loop1"] = map[string]map[string]dbus.Variant{
		BlockIface: props(map[string]any{"Device": append([]byte("/dev/loop1"), 0), "IdType": "ext4", "Size": uint64(1 << 26), "HintSystem": true}),
		FsIface:    {"MountPoints": mountPoints()},
		LoopIface:  props(map[string]any{"SetupByUID": uint32(0)}),
	}

	list := List(objects, testUID)
	mine, ok := byID(list, "/block/loop0")
	require.True(t, ok)
	assert.Equal(t, KindLoop, mine.Kind)
	assert.True(t, mine.Ejectable)
	assert.Equal(t, "archlinux.iso", mine.Name)
	assert.Equal(t, "media-optical", mine.IconName)

	_, theirs := byID(list, "/block/loop1")
	assert.False(t, theirs)
}

func TestListDropsBlocksWithoutAFilesystem(t *testing.T) {
	objects := Objects{
		"/block/whole": {
			BlockIface: props(map[string]any{"Device": append([]byte("/dev/sdb"), 0), "Size": uint64(1 << 35)}),
		},
	}
	assert.Empty(t, List(objects, testUID))
}

func TestListProtectsSystemMounts(t *testing.T) {
	for _, point := range []string{"/", "/home", "/home/brandon", "/boot", "/boot/efi", "/efi", "/usr", "/var/log", "/etc", "/root", "/opt"} {
		t.Run(point, func(t *testing.T) {
			objects := table()
			objects["/block/stick"][FsIface]["MountPoints"] = mountPoints("/mnt/alias", point)
			device, ok := byID(List(objects, testUID), "/block/stick")
			require.True(t, ok)
			assert.True(t, device.Protected)
		})
	}
	for _, point := range []string{"", "/mnt/projects", "/media/backup", "/run/media/brandon/USB", "/home-backup"} {
		t.Run(point, func(t *testing.T) {
			objects := table()
			objects["/block/stick"][FsIface]["MountPoints"] = mountPoints(point)
			device, ok := byID(List(objects, testUID), "/block/stick")
			require.True(t, ok)
			assert.False(t, device.Protected)
		})
	}
}
