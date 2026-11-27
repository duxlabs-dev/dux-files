package fileops

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

var (
	rangeChunk = 4 << 20
	batchLimit = 256 << 10
)

type batch struct {
	r        *runner
	src, dst string
	srcFd    int
	dstFd    int
}

func (r *runner) openBatch(src, dst string) (*batch, bool) {
	srcFd, err := openRaw(unix.AT_FDCWD, src, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, false
	}
	dstFd, err := openRaw(unix.AT_FDCWD, dst, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		unix.Close(srcFd)
		return nil, false
	}
	return &batch{r: r, src: src, dst: dst, srcFd: srcFd, dstFd: dstFd}, true
}

func (b *batch) close() {
	unix.Close(b.srcFd)
	unix.Close(b.dstFd)
}

func (b *batch) place(srcName, dstName string) bool {
	in, err := openRaw(b.srcFd, srcName, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer unix.Close(in)

	var st unix.Stat_t
	if unix.Fstat(in, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Size == 0 || st.Size > int64(batchLimit) {
		return false
	}

	perm := os.FileMode(st.Mode & 0o777)
	out, ok := b.r.openUnnamed(b.dstFd, ".", perm)
	if !ok {
		return false
	}
	if !b.r.fill(in, out, st.Size) {
		unix.Close(out)
		return false
	}

	target := filepath.Join(b.dst, dstName)
	b.r.stamp(filepath.Join(b.src, srcName), target, in, out, perm, st.Mtim)
	if settle(out, b.dstFd, dstName) != nil {
		return false
	}
	b.r.advance(target, 1, st.Size)
	return true
}

func (r *runner) copyUnnamed(src, dst string, info os.FileInfo) (bool, error) {
	in, err := openRaw(unix.AT_FDCWD, src, unix.O_RDONLY, 0)
	if err != nil {
		return false, nil
	}
	defer unix.Close(in)

	perm := info.Mode().Perm()
	out, ok := r.openUnnamed(unix.AT_FDCWD, filepath.Dir(dst), perm)
	if !ok {
		return false, nil
	}
	if err := r.pumpFd(src, in, out, info.Size()); err != nil {
		unix.Close(out)
		return true, err
	}

	r.stamp(src, dst, in, out, perm, unix.NsecToTimespec(info.ModTime().UnixNano()))
	err = settle(out, unix.AT_FDCWD, dst)
	var linkErr *os.LinkError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &linkErr) && !errors.Is(err, unix.EEXIST):
		r.noUnnamed.Store(true)
		return false, nil
	}
	return true, err
}

func (r *runner) openUnnamed(dirfd int, dir string, perm os.FileMode) (int, bool) {
	out, err := openRaw(dirfd, dir, unix.O_WRONLY|unix.O_TMPFILE, perm)
	if err != nil && unsupported(err) {
		r.noUnnamed.Store(true)
	}
	return out, err == nil
}

func unsupported(err error) bool {
	for _, code := range []unix.Errno{unix.EOPNOTSUPP, unix.EISDIR, unix.EINVAL, unix.ENOSYS} {
		if errors.Is(err, code) {
			return true
		}
	}
	return false
}

func openRaw(dirfd int, path string, flags int, perm os.FileMode) (int, error) {
	for {
		fd, err := unix.Openat(dirfd, path, flags|unix.O_CLOEXEC, uint32(perm.Perm()))
		if err != unix.EINTR {
			return fd, err
		}
	}
}

func (r *runner) stamp(src, dst string, in, out int, perm os.FileMode, mtime unix.Timespec) {
	if err := unix.Fchmod(out, uint32(perm)); err != nil {
		r.warn(dst, err)
	}
	if err := setTimes(out, mtime); err != nil {
		r.warn(dst, err)
	}
	if size, err := unix.Flistxattr(in, nil); err != nil || size == 0 {
		return
	}
	if err := copyXattrs(src, procPath(out)); err != nil {
		r.warn(dst, err)
	}
}

func setTimes(fd int, mtime unix.Timespec) error {
	times := []unix.Timespec{mtime, mtime}
	if unix.UtimesNanoAt(fd, "", times, unix.AT_EMPTY_PATH) == nil {
		return nil
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, procPath(fd), times, 0)
}

func procPath(fd int) string {
	return "/proc/self/fd/" + strconv.Itoa(fd)
}

func settle(out, dstDir int, dst string) error {
	linkErr := linkUnnamed(out, dstDir, dst)
	closeErr := unix.Close(out)
	switch {
	case linkErr != nil:
		return &os.LinkError{Op: "link", Old: "unnamed file", New: dst, Err: linkErr}
	case closeErr != nil:
		_ = unix.Unlinkat(dstDir, dst, 0)
		return &os.PathError{Op: "close", Path: dst, Err: closeErr}
	}
	return nil
}

func linkUnnamed(out, dstDir int, dst string) error {
	err := unix.Linkat(out, "", dstDir, dst, unix.AT_EMPTY_PATH)
	if err == nil || errors.Is(err, unix.EEXIST) {
		return err
	}
	return unix.Linkat(unix.AT_FDCWD, procPath(out), dstDir, dst, unix.AT_SYMLINK_FOLLOW)
}

func (r *runner) fill(in, out int, size int64) bool {
	switch {
	case r.clone(in, out):
		return true
	case r.noRange.Load():
		return fillBuffered(in, out, size)
	}

	copied := int64(0)
	for copied < size {
		n, err := copyRange(in, out, int(size-copied))
		switch {
		case err != nil && copied == 0:
			r.noRange.Store(true)
			return fillBuffered(in, out, size)
		case err != nil, n == 0:
			return false
		}
		copied += int64(n)
	}
	return true
}

func fillBuffered(in, out int, size int64) bool {
	chunk := borrowChunk()
	defer chunks.Put(chunk)
	written, err := io.CopyBuffer(rawFile{fd: out}, io.LimitReader(rawFile{fd: in}, size), *chunk)
	return err == nil && written == size
}

func (r *runner) pump(path string, in io.Reader, out io.Writer, size int64) error {
	src, dst := fileOf(in), fileOf(out)
	if src == nil || dst == nil {
		return r.stream(path, in, out)
	}
	return r.pumpFd(path, int(src.Fd()), int(dst.Fd()), size)
}

func fileOf(stream any) *os.File {
	switch file := stream.(type) {
	case *os.File:
		return file
	case *localWriter:
		return file.File
	}
	return nil
}

func (r *runner) pumpFd(path string, in, out int, size int64) error {
	if r.clone(in, out) {
		r.advance(path, 0, size)
		return nil
	}
	finished, err := r.pumpRanges(path, in, out, size)
	if finished || err != nil {
		return err
	}
	return r.stream(path, rawFile{fd: in, path: path}, rawFile{fd: out, path: path})
}

func (r *runner) clone(in, out int) bool {
	if r.noClone.Load() {
		return false
	}
	if unix.IoctlFileClone(out, in) == nil {
		return true
	}
	r.noClone.Store(true)
	return false
}

func (r *runner) pumpRanges(path string, in, out int, size int64) (bool, error) {
	copied := int64(0)
	for !r.noRange.Load() && copied < size {
		if r.job.closed() {
			return false, context.Canceled
		}
		n, err := copyRange(in, out, int(min(int64(rangeChunk), size-copied)))
		switch {
		case err != nil && copied == 0:
			r.noRange.Store(true)
			return false, nil
		case err != nil, n == 0:
			return false, nil
		}
		copied += int64(n)
		r.advance(path, 0, int64(n))
	}
	return size > 0 && copied == size, nil
}

func copyRange(in, out, limit int) (int, error) {
	for {
		n, err := unix.CopyFileRange(in, nil, out, nil, limit, 0)
		if err != unix.EINTR {
			return n, err
		}
	}
}

type rawFile struct {
	fd   int
	path string
}

func (f rawFile) Read(p []byte) (int, error) {
	for {
		n, err := unix.Read(f.fd, p)
		switch {
		case err == unix.EINTR:
			continue
		case err != nil:
			return 0, &os.PathError{Op: "read", Path: f.path, Err: err}
		case n == 0:
			return 0, io.EOF
		}
		return n, nil
	}
}

func (f rawFile) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		n, err := unix.Write(f.fd, p[written:])
		switch {
		case err == unix.EINTR:
			continue
		case err != nil:
			return written, &os.PathError{Op: "write", Path: f.path, Err: err}
		}
		written += n
	}
	return written, nil
}
