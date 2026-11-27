package helper

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
)

func lstat(path string, out io.Writer) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	var st unix.Stat_t
	if err := unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "lstat", Path: path, Err: err}
	}
	return json.NewEncoder(out).Encode(infoOf(name, &st))
}

func infoOf(name string, st *unix.Stat_t) Info {
	return Info{
		Name:    name,
		Size:    st.Size,
		Mode:    uint32(modeOf(st.Mode)),
		MtimeNs: st.Mtim.Nano(),
		UID:     st.Uid,
		GID:     st.Gid,
	}
}

func modeOf(raw uint32) fs.FileMode {
	mode := fs.FileMode(raw & 0o777)
	switch raw & syscall.S_IFMT {
	case syscall.S_IFBLK:
		mode |= fs.ModeDevice
	case syscall.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case syscall.S_IFDIR:
		mode |= fs.ModeDir
	case syscall.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= fs.ModeSymlink
	case syscall.S_IFSOCK:
		mode |= fs.ModeSocket
	}
	if raw&syscall.S_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if raw&syscall.S_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if raw&syscall.S_ISVTX != 0 {
		mode |= fs.ModeSticky
	}
	return mode
}

func readDir(path string, out io.Writer) error {
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	children, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	entries := make([]Entry, 0, len(children))
	for _, child := range children {
		entries = append(entries, Entry{Name: child.Name(), Mode: uint32(child.Type())})
	}
	return json.NewEncoder(out).Encode(entries)
}

func openDir(path string) (*os.File, error) {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return nil, err
	}
	defer unix.Close(parent)

	fd, err := dhelper.OpenNoSymlinks(parent, name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func readlink(path string, out io.Writer) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	for size := 256; ; size *= 2 {
		buf := make([]byte, size)
		n, err := unix.Readlinkat(parent, name, buf)
		if err != nil {
			return &os.PathError{Op: "readlink", Path: path, Err: err}
		}
		if n < size {
			return json.NewEncoder(out).Encode(map[string]string{"target": string(buf[:n])})
		}
	}
}

func read(path string, out io.Writer) error {
	_, err := dhelper.Read(path, out)
	return err
}

func write(path string, mode uint32, in io.Reader, ready io.Writer) error {
	_, err := dhelper.Create(path, mode, false, in, ready)
	return err
}

func mkdir(path string, mode uint32) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	if err := unix.Mkdirat(parent, name, mode); err != nil {
		return &os.PathError{Op: "mkdir", Path: path, Err: err}
	}
	return nil
}

func mkdirAll(path string, mode uint32) error {
	if !dhelper.ValidPath(path) {
		return dhelper.Invalid("not an absolute, clean path: " + path)
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i := range parts {
		prefix := "/" + strings.Join(parts[:i+1], "/")
		err := mkdir(prefix, mode)
		switch {
		case err == nil, errors.Is(err, fs.ErrExist):
			continue
		default:
			return err
		}
	}
	return nil
}

func symlink(target, path string) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	if err := unix.Symlinkat(target, parent, name); err != nil {
		return &os.PathError{Op: "symlink", Path: path, Err: err}
	}
	return nil
}

func rename(from, to string, noReplace bool) error {
	fromParent, fromName, err := dhelper.OpenParent(from)
	if err != nil {
		return err
	}
	defer unix.Close(fromParent)

	toParent, toName, err := dhelper.OpenParent(to)
	if err != nil {
		return err
	}
	defer unix.Close(toParent)

	var flags uint
	if noReplace {
		flags = unix.RENAME_NOREPLACE
	}
	err = unix.Renameat2(fromParent, fromName, toParent, toName, flags)
	if noReplace && (errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS)) {
		err = renameatIfAbsent(fromParent, fromName, toParent, toName)
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

func renameatIfAbsent(fromParent int, fromName string, toParent int, toName string) error {
	var st unix.Stat_t
	err := unix.Fstatat(toParent, toName, &st, unix.AT_SYMLINK_NOFOLLOW)
	switch {
	case err == nil:
		return unix.EEXIST
	case !errors.Is(err, unix.ENOENT):
		return err
	}
	return unix.Renameat(fromParent, fromName, toParent, toName)
}

func utimes(path string, mtimeNs int64) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	stamp := unix.NsecToTimespec(mtimeNs)
	if err := unix.UtimesNanoAt(parent, name, []unix.Timespec{stamp, stamp}, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "utimes", Path: path, Err: err}
	}
	return nil
}

func remove(path string) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	return unlink(parent, name, path)
}

func unlink(parent int, name, path string) error {
	err := unix.Unlinkat(parent, name, 0)
	if errors.Is(err, unix.EISDIR) {
		err = unix.Unlinkat(parent, name, unix.AT_REMOVEDIR)
	}
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	return nil
}

func removeAll(path string) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	var st unix.Stat_t
	err = unix.Fstatat(parent, name, &st, unix.AT_SYMLINK_NOFOLLOW)
	switch {
	case errors.Is(err, unix.ENOENT):
		return nil
	case err != nil:
		return &os.PathError{Op: "lstat", Path: path, Err: err}
	case st.Mode&syscall.S_IFMT != syscall.S_IFDIR:
		return unlink(parent, name, path)
	}

	fd, err := dhelper.OpenNoSymlinks(parent, name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(fd), path)
	err = removeChildren(dir)
	dir.Close()
	if err != nil {
		return err
	}
	if err := unix.Unlinkat(parent, name, unix.AT_REMOVEDIR); err != nil {
		return &os.PathError{Op: "rmdir", Path: path, Err: err}
	}
	return nil
}

func removeChildren(dir *os.File) error {
	return walk(dir, func(dirfd int, child os.DirEntry) error {
		if child.IsDir() {
			return nil
		}
		return unlink(dirfd, child.Name(), filepath.Join(dir.Name(), child.Name()))
	}, func(dirfd int, child os.DirEntry) error {
		if err := unix.Unlinkat(dirfd, child.Name(), unix.AT_REMOVEDIR); err != nil {
			return &os.PathError{Op: "rmdir", Path: filepath.Join(dir.Name(), child.Name()), Err: err}
		}
		return nil
	})
}

func walk(dir *os.File, visit, after func(dirfd int, child os.DirEntry) error) error {
	children, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	dirfd := int(dir.Fd())
	for _, child := range children {
		if err := visit(dirfd, child); err != nil {
			return err
		}
		if !child.IsDir() {
			continue
		}
		fd, err := dhelper.OpenNoSymlinks(dirfd, child.Name(), unix.O_RDONLY|unix.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		sub := os.NewFile(uintptr(fd), filepath.Join(dir.Name(), child.Name()))
		err = walk(sub, visit, after)
		sub.Close()
		if err != nil {
			return err
		}
		if after == nil {
			continue
		}
		if err := after(dirfd, child); err != nil {
			return err
		}
	}
	return nil
}

func chmod(path string, mode uint32, recursive bool) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	if err := chmodAt(parent, name, mode); err != nil {
		return &os.PathError{Op: "chmod", Path: path, Err: err}
	}
	if !recursive {
		return nil
	}
	dir, err := openDir(path)
	if err != nil {
		if errors.Is(err, unix.ENOTDIR) {
			return nil
		}
		return err
	}
	defer dir.Close()
	return walk(dir, func(dirfd int, child os.DirEntry) error {
		if child.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if err := chmodAt(dirfd, child.Name(), mode); err != nil {
			return &os.PathError{Op: "chmod", Path: filepath.Join(dir.Name(), child.Name()), Err: err}
		}
		return nil
	}, nil)
}

// fchmodat has no AT_SYMLINK_NOFOLLOW on Linux, hence the O_PATH fd and /proc
func chmodAt(dirfd int, name string, mode uint32) error {
	fd, err := dhelper.OpenNoSymlinks(dirfd, name, unix.O_PATH, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	return unix.Chmod(dhelper.ProcPath(fd), mode)
}

func chown(path string, uid, gid int, recursive bool) error {
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return err
	}
	defer unix.Close(parent)

	if err := unix.Fchownat(parent, name, uid, gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "chown", Path: path, Err: err}
	}
	if !recursive {
		return nil
	}
	dir, err := openDir(path)
	if err != nil {
		if errors.Is(err, unix.ENOTDIR) {
			return nil
		}
		return err
	}
	defer dir.Close()
	return walk(dir, func(dirfd int, child os.DirEntry) error {
		if err := unix.Fchownat(dirfd, child.Name(), uid, gid, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return &os.PathError{Op: "chown", Path: filepath.Join(dir.Name(), child.Name()), Err: err}
		}
		return nil
	}, nil)
}
