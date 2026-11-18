package devices

import (
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
)

const CodePerm files.Code = "EPERM"

func wrapDBus(err error) error {
	return dbusutil.WrapDBus(err, func(call dbus.Error, _ string) files.Code { return codeForName(call.Name) })
}

func codeForName(name string) files.Code {
	switch strings.TrimPrefix(name, BusName+".Error.") {
	case "NotAuthorized", "NotAuthorizedCanObtain", "NotAuthorizedDismissed":
		return files.CodeAccess
	case "DeviceBusy":
		return files.CodeBusy
	case "AlreadyMounted", "AlreadyUnmounting":
		return files.CodeExists
	case "NotMounted", "OptionNotPermitted", "MountedByOtherUser":
		return files.CodeInvalid
	case "NotSupported":
		return files.CodeNotSupported
	}
	switch name {
	case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner":
		return files.CodeNotSupported
	case "org.freedesktop.DBus.Error.AccessDenied":
		return files.CodeAccess
	case "org.freedesktop.DBus.Error.UnknownMethod", "org.freedesktop.DBus.Error.UnknownInterface":
		return files.CodeNotSupported
	case "org.freedesktop.DBus.Error.UnknownObject":
		return files.CodeNotFound
	}
	return files.CodeIO
}
