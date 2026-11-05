package mimeapps

import (
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fixture copies the mini mime database and xdg tree into a scratch root and
// points every XDG variable at it; the host's own database is never read.
func fixture(t *testing.T) (*Service, string) {
	t.Helper()

	root := t.TempDir()
	copyTree(t, "testdata", root)
	writeMagic(t, filepath.Join(root, "mime", "magic"))

	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg", "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "xdg", "share"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg", "config"))
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "xdg", "etc-xdg"))
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	t.Setenv("INVOCATION_ID", "")

	return NewServiceForDirs([]string{filepath.Join(root, "mime")}), root
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}))
}

// writeMagic builds the binary half of the fixture database so the checked-in
// tree stays readable text.
func writeMagic(t *testing.T, path string) {
	t.Helper()

	buf := []byte(magicHeader)
	section := func(priority int, mime string) {
		buf = append(buf, '[')
		buf = append(buf, []byte(itoa(priority)+":"+mime)...)
		buf = append(buf, ']', '\n')
	}
	rule := func(indent, offset int, value []byte, mask []byte, rangeLen int) {
		if indent > 0 {
			buf = append(buf, []byte(itoa(indent))...)
		}
		buf = append(buf, '>')
		buf = append(buf, []byte(itoa(offset))...)
		buf = append(buf, '=')
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(value)))
		buf = append(buf, value...)
		if mask != nil {
			buf = append(buf, '&')
			buf = append(buf, mask...)
		}
		if rangeLen > 0 {
			buf = append(buf, '+')
			buf = append(buf, []byte(itoa(rangeLen))...)
		}
		buf = append(buf, '\n')
	}

	section(90, "application/x-magic-wins")
	rule(0, 0, []byte("MAGICWIN"), nil, 0)
	section(70, "audio/x-wav")
	rule(0, 0, []byte("RIFF"), nil, 0)
	rule(1, 8, []byte("WAVE"), nil, 0)
	section(70, "video/x-msvideo")
	rule(0, 0, []byte("RIFF"), nil, 0)
	rule(1, 8, []byte("AVI "), nil, 0)
	section(50, "image/png")
	rule(0, 0, []byte("\x89PNG\r\n\x1a\n"), nil, 0)
	section(50, "application/pdf")
	rule(0, 0, []byte("%PDF-"), nil, 0)
	section(50, "application/x-shellscript")
	rule(0, 0, []byte("#!"), nil, 0)
	rule(1, 2, []byte("/bin/sh"), nil, 8)
	section(40, "text/x-deep")
	rule(0, 600, []byte("DEEPMARK"), nil, 0)

	require.NoError(t, os.WriteFile(path, buf, 0o644))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

const (
	waitFor  = 3 * time.Second
	waitTick = 10 * time.Millisecond
)
