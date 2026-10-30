package netmount

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/dbusutil"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
)

const CodeCanceled files.Code = "ECANCELED"

// GIO has no registered D-Bus domain, so gvfs ships GIOErrorEnum as this unmapped name.
const unmappedGIOError = "org.gtk.GDBus.UnmappedGError.Quark._g_2dio_2derror_2dquark.Code"

const (
	gioPermissionDenied = 14
	gioNotMounted       = 16
	gioAlreadyMounted   = 17
	gioCancelled        = 19
	gioBusy             = 26
)

func fail(code files.Code, format string, args ...any) error {
	return &files.Error{Code: code, Err: fmt.Errorf(format, args...)}
}

func wrapDBus(err error) error {
	if err == nil {
		return nil
	}
	call, ok := err.(dbus.Error)
	if !ok {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	if call.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return fail(files.CodeNotSupported, "gvfs is not running")
	}
	text := dbusutil.ErrorMessage(call)
	return &files.Error{Code: codeFor(call, text), Err: errors.New(text)}
}

func codeFor(call dbus.Error, text string) files.Code {
	code, ok := gioCode(call)
	if !ok {
		return codeForText(call.Name, text)
	}
	switch code {
	case gioPermissionDenied:
		return files.CodeAccess
	case gioCancelled:
		return CodeCanceled
	case gioNotMounted:
		return files.CodeInvalid
	case gioBusy:
		return files.CodeBusy
	}
	return files.CodeIO
}

func codeForText(name, text string) files.Code {
	switch {
	case strings.Contains(name, "PermissionDenied"), strings.Contains(text, "Permission denied"):
		return files.CodeAccess
	case strings.Contains(text, "cancelled"):
		return CodeCanceled
	case strings.Contains(text, "not mounted"):
		return files.CodeInvalid
	case strings.Contains(text, "busy"):
		return files.CodeBusy
	}
	return files.CodeIO
}

func gioCode(err error) (int, bool) {
	call, ok := err.(dbus.Error)
	if !ok {
		return 0, false
	}
	digits, ok := strings.CutPrefix(call.Name, unmappedGIOError)
	if !ok {
		return 0, false
	}
	code, parseErr := strconv.Atoi(digits)
	return code, parseErr == nil
}

func alreadyMounted(err error) bool {
	code, ok := gioCode(err)
	return ok && code == gioAlreadyMounted
}
