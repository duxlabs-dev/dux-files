package fileops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

const removeBatch = 256

type sweep struct {
	r       *runner
	counted bool
	removed atomic.Int64
	helpers chan struct{}

	mu      sync.Mutex
	failure error
}

func (r *runner) newSweep(counted bool) *sweep {
	return &sweep{r: r, counted: counted, helpers: make(chan struct{}, workers-1)}
}

func (s *sweep) tree(path string) error {
	s.failure = nil
	unlinkErr := unlinkAt(unix.AT_FDCWD, path, 0)
	if unlinkErr == nil || errors.Is(unlinkErr, unix.ENOENT) {
		s.report(path, s.count(s.settle(unlinkErr, path)))
		return s.stopped()
	}

	fd, err := openDirAt(unix.AT_FDCWD, path)
	switch {
	case err == nil:
	case errors.Is(err, unix.ENOTDIR), errors.Is(err, unix.ELOOP):
		return &os.PathError{Op: "unlinkat", Path: path, Err: unlinkErr}
	default:
		return &os.PathError{Op: "openat", Path: path, Err: err}
	}
	s.children(fd, path)
	if err := s.stopped(); err != nil {
		return err
	}
	s.report(path, s.count(s.settle(unlinkAt(unix.AT_FDCWD, path, unix.AT_REMOVEDIR), path)))
	return s.failure
}

func (s *sweep) children(fd int, path string) {
	dir := os.NewFile(uintptr(fd), path)
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		s.keep(err)
	}

	var crew sync.WaitGroup
	defer crew.Wait()
	pending := 0
	for _, entry := range entries {
		if pending >= removeBatch {
			s.report(path, pending)
			pending = 0
			if s.r.job.closed() {
				return
			}
		}
		name := entry.Name()
		switch {
		case !entry.IsDir():
			pending += s.count(s.settle(unlinkAt(fd, name, 0), filepath.Join(path, name)))
		case s.borrowHelper():
			crew.Go(func() {
				defer s.returnHelper()
				s.report(path, s.count(s.subtree(fd, path, name)))
			})
		default:
			pending += s.count(s.subtree(fd, path, name))
		}
	}
	s.report(path, pending)
}

func (s *sweep) subtree(parent int, parentPath, name string) bool {
	path := filepath.Join(parentPath, name)
	fd, err := openDirAt(parent, name)
	if err != nil {
		return s.settle(err, path)
	}
	s.children(fd, path)
	if s.r.job.closed() {
		return false
	}
	return s.settle(unlinkAt(parent, name, unix.AT_REMOVEDIR), path)
}

func (s *sweep) borrowHelper() bool {
	select {
	case s.helpers <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *sweep) returnHelper() {
	<-s.helpers
}

func (s *sweep) count(removed bool) int {
	if removed {
		return 1
	}
	return 0
}

func (s *sweep) settle(err error, path string) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, unix.ENOENT):
	default:
		s.keep(&os.PathError{Op: "unlinkat", Path: path, Err: err})
	}
	return false
}

func (s *sweep) keep(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure == nil {
		s.failure = err
	}
}

func (s *sweep) report(path string, removed int) {
	s.removed.Add(int64(removed))
	if s.counted && removed > 0 {
		s.r.advance(path, removed, 0)
	}
}

func (s *sweep) stopped() error {
	if s.r.job.closed() {
		return context.Canceled
	}
	return nil
}

func unlinkAt(dirfd int, name string, flags int) error {
	for {
		err := unix.Unlinkat(dirfd, name, flags)
		if err != unix.EINTR {
			return err
		}
	}
}

func openDirAt(dirfd int, name string) (int, error) {
	for {
		fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != unix.EINTR {
			return fd, err
		}
	}
}
