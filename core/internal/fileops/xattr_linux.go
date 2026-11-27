package fileops

import (
	"strings"

	"golang.org/x/sys/unix"
)

func copyXattrs(src, dst string) error {
	size, err := unix.Llistxattr(src, nil)
	if err != nil || size == 0 {
		return nil
	}

	names := make([]byte, size)
	read, err := unix.Llistxattr(src, names)
	if err != nil {
		return nil
	}

	var failed error
	for _, name := range strings.Split(strings.TrimRight(string(names[:read]), "\x00"), "\x00") {
		if !strings.HasPrefix(name, "user.") {
			continue
		}
		valueSize, err := unix.Lgetxattr(src, name, nil)
		if err != nil || valueSize < 0 {
			continue
		}
		value := make([]byte, valueSize)
		if _, err := unix.Lgetxattr(src, name, value); err != nil {
			continue
		}
		if err := unix.Setxattr(dst, name, value, 0); err != nil {
			failed = err
		}
	}
	return failed
}
