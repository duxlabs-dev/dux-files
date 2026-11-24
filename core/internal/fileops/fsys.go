package fileops

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-go/elevate"
)

type FS interface {
	Lstat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
	Readlink(path string) (string, error)
	Open(path string) (io.ReadCloser, error)
	Create(path string, perm os.FileMode) (Writer, error)
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Symlink(target, path string) error
	Rename(from, to string, noReplace bool) error
	Remove(path string) error
	RemoveAll(path string) error
	Chmod(path string, mode os.FileMode, recursive bool) error
	Lchown(path string, uid, gid int, recursive bool) error
	Chtimes(path string, mtime time.Time) error
}

type Writer interface {
	io.WriteCloser
	Abort() error
}

type Elevator interface {
	FS
	Route() string
	Available() error
}

// An alias, so a helper that cannot start matches it from any operation, not only from Available.
type Unavailable = elevate.Unavailable

type localFS struct{}

var local FS = localFS{}

func (localFS) Lstat(path string) (os.FileInfo, error)     { return os.Lstat(path) }
func (localFS) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (localFS) Readlink(path string) (string, error)       { return os.Readlink(path) }
func (localFS) Open(path string) (io.ReadCloser, error) {
	return openUnpolled(path, unix.O_RDONLY, 0)
}
func (localFS) Mkdir(path string, perm os.FileMode) error { return os.Mkdir(path, perm) }
func (localFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
func (localFS) Symlink(target, path string) error { return os.Symlink(target, path) }
func (localFS) Remove(path string) error          { return os.Remove(path) }
func (localFS) RemoveAll(path string) error       { return os.RemoveAll(path) }

func (localFS) Rename(from, to string, noReplace bool) error {
	if !noReplace {
		return os.Rename(from, to)
	}
	return renameNoReplace(from, to)
}

func (localFS) Chtimes(path string, mtime time.Time) error {
	return os.Chtimes(path, mtime, mtime)
}

func (localFS) Create(path string, perm os.FileMode) (Writer, error) {
	file, err := openUnpolled(path, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, perm)
	if err != nil {
		return nil, err
	}
	return &localWriter{File: file, path: path}, nil
}

func openUnpolled(path string, flags int, perm os.FileMode) (*os.File, error) {
	for {
		fd, err := unix.Open(path, flags|unix.O_CLOEXEC, uint32(perm.Perm()))
		switch {
		case err == unix.EINTR:
			continue
		case err != nil:
			return nil, &os.PathError{Op: "open", Path: path, Err: err}
		}
		return os.NewFile(uintptr(fd), path), nil
	}
}

func (localFS) Chmod(path string, mode os.FileMode, recursive bool) error {
	if !recursive {
		return os.Chmod(path, mode)
	}
	return filepath.WalkDir(path, func(child string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		return os.Chmod(child, mode)
	})
}

func (localFS) Lchown(path string, uid, gid int, recursive bool) error {
	if !recursive {
		return os.Lchown(path, uid, gid)
	}
	return filepath.WalkDir(path, func(child string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(child, uid, gid)
	})
}

type localWriter struct {
	*os.File
	path string
}

func (w *localWriter) Abort() error {
	w.File.Close()
	return os.Remove(w.path)
}

type creation struct {
	fs   FS
	path string
}

func (c *creation) discard() {
	if c.fs == nil {
		return
	}
	_ = c.fs.Remove(c.path)
	c.fs = nil
}

func (c *creation) abort(w Writer) {
	_ = w.Abort()
	c.fs = nil
}
