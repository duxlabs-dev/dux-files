package fileops

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

var scanBudget = 2 * time.Second

const scanBatch = 1024

type scanResult struct {
	files int
	bytes int64
}

var errScanBudget = errors.New("scan budget exceeded")

func (r *runner) scan(paths []string) (int, int64, bool) {
	return r.walk(paths, true)
}

func (r *runner) count(paths []string) (int, bool) {
	files, _, complete := r.walk(paths, false)
	return files, complete
}

func (r *runner) walk(paths []string, sized bool) (int, int64, bool) {
	walker := tally{r: r, sized: sized, deadline: time.Now().Add(scanBudget)}
	total := scanResult{}
	complete := true

	for _, path := range paths {
		walker.found = scanResult{}
		err := walker.root(path)

		total.files += walker.found.files
		total.bytes += walker.found.bytes
		if err != nil {
			complete = false
			continue
		}
		r.scanned[path] = walker.found
	}
	return total.files, total.bytes, complete
}

type tally struct {
	r        *runner
	sized    bool
	deadline time.Time
	found    scanResult
}

func (t *tally) root(path string) error {
	var st unix.Stat_t
	if unix.Fstatat(unix.AT_FDCWD, path, &st, unix.AT_SYMLINK_NOFOLLOW) != nil {
		return nil
	}
	if err := t.note(st.Mode&unix.S_IFMT == unix.S_IFREG, st.Size); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil
	}
	return t.dir(unix.AT_FDCWD, path, path)
}

func (t *tally) dir(parent int, name, path string) error {
	fd, err := openDirAt(parent, name)
	if err != nil {
		return nil
	}
	dir := os.NewFile(uintptr(fd), path)
	defer dir.Close()

	for {
		entries, readErr := dir.ReadDir(scanBatch)
		for _, entry := range entries {
			if err := t.entry(fd, path, entry); err != nil {
				return err
			}
		}
		if readErr != nil {
			return nil
		}
	}
}

func (t *tally) entry(fd int, parent string, entry os.DirEntry) error {
	if entry.IsDir() {
		if err := t.note(false, 0); err != nil {
			return err
		}
		return t.dir(fd, entry.Name(), filepath.Join(parent, entry.Name()))
	}
	if !t.sized || !entry.Type().IsRegular() {
		return t.note(false, 0)
	}

	var st unix.Stat_t
	if unix.Fstatat(fd, entry.Name(), &st, unix.AT_SYMLINK_NOFOLLOW) != nil {
		return t.note(false, 0)
	}
	return t.note(true, st.Size)
}

func (t *tally) note(regular bool, size int64) error {
	if t.r.job.closed() {
		return errScanBudget
	}
	t.found.files++
	if t.sized && regular {
		t.found.bytes += size
	}
	if time.Now().After(t.deadline) {
		return errScanBudget
	}
	return nil
}
