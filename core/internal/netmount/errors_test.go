package netmount

import (
	"errors"
	"testing"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

func TestWrapDBusMapsTheGIOCodeNotTheLocalisedText(t *testing.T) {
	cases := []struct {
		name string
		text string
		code files.Code
	}{
		{unmappedGIOError + "14", "Zugriff verweigert", files.CodeAccess},
		{unmappedGIOError + "19", "Vorgang abgebrochen", CodeCanceled},
		{unmappedGIOError + "26", "Datei ist belegt", files.CodeBusy},
		{unmappedGIOError + "0", "Failed to mount Windows share: Connection refused", files.CodeIO},
		{"org.freedesktop.DBus.Error.Failed", "Device is busy", files.CodeBusy},
	}
	for _, tc := range cases {
		err := wrapDBus(dbus.Error{Name: tc.name, Body: []any{tc.text}})
		assert.Equal(t, tc.code, files.CodeOf(err), tc.name)
		assert.EqualError(t, errors.Unwrap(err), tc.text)
	}
}
