package fileops

import (
	"errors"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(from, to string) error {
	err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOSYS):
		return renameIfAbsent(from, to)
	default:
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
}

func renameIfAbsent(from, to string) error {
	_, err := os.Lstat(to)
	switch {
	case err == nil:
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: fs.ErrExist}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return os.Rename(from, to)
}
