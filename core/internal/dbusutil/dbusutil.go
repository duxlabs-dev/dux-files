package dbusutil

import (
	"bytes"
	"errors"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
)

// udisks and gvfs terminate every ay string with a NUL.
func ByteString(raw []byte) string {
	return string(bytes.TrimRight(raw, "\x00"))
}

func ErrorMessage(call dbus.Error) string {
	if len(call.Body) == 0 {
		return call.Name
	}
	text, ok := call.Body[0].(string)
	if !ok || text == "" {
		return call.Name
	}
	return text
}

func WrapDBus(err error, code func(call dbus.Error, text string) files.Code) error {
	if err == nil {
		return nil
	}
	call, ok := err.(dbus.Error)
	if !ok {
		return &files.Error{Code: files.CodeIO, Err: err}
	}
	text := ErrorMessage(call)
	return &files.Error{Code: code(call, text), Err: errors.New(text)}
}
