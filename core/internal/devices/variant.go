package devices

import (
	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	variant "github.com/duxlabs-dev/dux-go/dbusutil"
	"github.com/godbus/dbus/v5"
)

func stringOf(v dbus.Variant) string { return variant.AsOr(v, "") }

func boolOf(v dbus.Variant) bool { return variant.AsOr(v, false) }

func uint64Of(v dbus.Variant) uint64 { return variant.AsOr[uint64](v, 0) }

func uint32Of(v dbus.Variant) uint32 { return variant.AsOr[uint32](v, 0) }

func objectOf(v dbus.Variant) string {
	path, _ := v.Value().(dbus.ObjectPath)
	if path == "" || path == "/" {
		return ""
	}
	return string(path)
}

func byteStringOf(v dbus.Variant) string {
	raw, _ := v.Value().([]byte)
	return dbusutil.ByteString(raw)
}
