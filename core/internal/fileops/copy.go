package fileops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"
)

var copyChunk = 256 << 10

const tempNameTail = 64

type outcome struct {
	dst      string
	undoable bool
	dir      bool
}

func (r *runner) transferAll(move bool) error {
	return r.transferPairs(r.targets(), move)
}

func (r *runner) transferPairs(pairs []pair, move bool) error {
	var needScan []string
	for _, p := range pairs {
		if move && sameDevice(p.src, filepath.Dir(p.dst)) {
			continue
		}
		needScan = append(needScan, p.src)
	}

	switch len(needScan) {
	case 0:
		r.setTotals(len(pairs), 0, false)
	default:
		count, bytes, complete := r.scan(needScan)
		r.setTotals(count+len(pairs)-len(needScan), bytes, !complete)
	}

	return eachSource(r, pairs, func(p pair) error {
		if !r.guarded(p.src) {
			return errors.New("changed since the operation it would replay: " + p.src)
		}
		done, err := r.transfer(p.src, p.dst, move)
		if err != nil {
			return err
		}
		if done.undoable {
			r.remember(inverseOf(move), done.dst, p.src)
		}
		if r.job.spec.kind == KindDuplicate && done.dst != "" {
			r.noteCreated(done.dst, done.dir)
		}
		return nil
	})
}

func (r *runner) transfer(src, dst string, move bool) (outcome, error) {
	var info os.FileInfo
	if err := r.retry(src, func(fs FS) error {
		var statErr error
		info, statErr = fs.Lstat(src)
		return statErr
	}); err != nil {
		return outcome{}, err
	}

	res := r.resolve(src, dst, info, move)
	switch {
	case res.abort:
		return outcome{}, context.Canceled
	case res.skip:
		return outcome{}, errSkipped
	}
	dst = res.dst

	undoable := !res.merge && !res.replaced
	if move && !res.merge && r.renameInto(src, dst, info) {
		return outcome{dst: dst, undoable: undoable, dir: info.IsDir()}, nil
	}
	if err := r.clearTarget(dst, res, info.IsDir()); err != nil {
		return outcome{}, err
	}

	switch {
	case info.IsDir():
		return outcome{dst: dst, undoable: undoable, dir: true}, r.transferDir(src, dst, move, res.merge)
	case info.Mode()&fs.ModeSymlink != 0:
		return outcome{dst: dst, undoable: undoable}, r.transferLink(src, dst, move, !res.replaced)
	case info.Mode().IsRegular():
		return outcome{dst: dst, undoable: undoable}, r.transferFile(src, dst, info, move, !res.replaced)
	default:
		r.skip()
		return outcome{}, errSkipped
	}
}

func (r *runner) renameInto(src, dst string, info os.FileInfo) bool {
	if !sameDevice(src, filepath.Dir(dst)) {
		return false
	}
	_, statErr := os.Lstat(dst)
	switch {
	case statErr == nil && info.IsDir():
		return false
	case statErr != nil && !errors.Is(statErr, fs.ErrNotExist):
		return false
	}
	if err := os.Rename(src, dst); err != nil {
		r.scanLate(src)
		return false
	}

	counted := r.scanned[src]
	if counted.files == 0 {
		counted.files = 1
	}
	r.advance(dst, counted.files, counted.bytes)
	return true
}

func (r *runner) scanLate(src string) {
	if _, known := r.scanned[src]; known {
		return
	}
	count, bytes, complete := r.scan([]string{src})
	r.job.mu.Lock()
	r.job.progress.FilesTotal += count - 1
	r.job.progress.BytesTotal += bytes
	r.job.progress.Estimating = r.job.progress.Estimating || !complete
	r.job.mu.Unlock()
	r.estimating = r.estimating || !complete
}

func (r *runner) clearTarget(dst string, res resolution, srcIsDir bool) error {
	if res.merge {
		return nil
	}
	target, err := os.Lstat(dst)
	if err != nil {
		return nil
	}
	if !target.IsDir() && !srcIsDir {
		return nil
	}
	return r.retry(dst, func(fs FS) error { return fs.RemoveAll(dst) })
}

func (r *runner) transferDir(src, dst string, move, merge bool) error {
	var info os.FileInfo
	if err := r.retry(src, func(fs FS) error {
		var statErr error
		info, statErr = fs.Lstat(src)
		return statErr
	}); err != nil {
		return err
	}

	dstFS := local
	if err := r.retry(dst, func(fs FS) error {
		dstFS = fs
		return fs.MkdirAll(dst, info.Mode().Perm()|0o700)
	}); err != nil {
		return err
	}

	var children []os.DirEntry
	srcFS := local
	if err := r.retry(src, func(fs FS) error {
		var readErr error
		children, readErr = fs.ReadDir(src)
		srcFS = fs
		return readErr
	}); err != nil {
		return err
	}
	if r.estimating {
		r.growTotals(len(children))
	}

	for _, child := range children {
		if r.job.closed() {
			return context.Canceled
		}
		_, err := r.transfer(filepath.Join(src, child.Name()), filepath.Join(dst, child.Name()), move)
		switch {
		case err == nil, errors.Is(err, errSkipped):
			continue
		default:
			return err
		}
	}

	r.preserve(src, dst, info, dstFS)
	r.advance(dst, 1, 0)
	if !move {
		return nil
	}
	if err := srcFS.Remove(src); err != nil {
		r.warn(src, err)
	}
	return nil
}

func (r *runner) transferLink(src, dst string, move, noReplace bool) error {
	var target string
	if err := r.retry(src, func(fs FS) error {
		var readErr error
		target, readErr = fs.Readlink(src)
		return readErr
	}); err != nil {
		return err
	}

	tmp := r.tempPath(dst)
	made := &creation{path: tmp}
	if err := r.retry(dst, func(fs FS) error {
		made.discard()
		if err := fs.Symlink(target, tmp); err != nil {
			return err
		}
		made.fs = fs
		return nil
	}); err != nil {
		return err
	}
	if err := r.commit(tmp, dst, made, noReplace); err != nil {
		return err
	}

	r.advance(dst, 1, 0)
	return r.dropSource(src, move)
}

func (r *runner) transferFile(src, dst string, info os.FileInfo, move, noReplace bool) error {
	tmp := r.tempPath(dst)
	made := &creation{path: tmp}
	if err := r.retryLocal(src, func() error {
		made.discard()
		return r.copyBytes(src, tmp, info, made)
	}); err != nil {
		made.discard()
		return err
	}

	r.preserve(src, tmp, info, made.fs)
	if err := r.commit(tmp, dst, made, noReplace); err != nil {
		return err
	}

	r.advance(dst, 1, 0)
	return r.dropSource(src, move)
}

func (r *runner) commit(tmp, dst string, made *creation, noReplace bool) error {
	if err := r.retry(dst, func(fs FS) error { return fs.Rename(tmp, dst, noReplace) }); err != nil {
		made.discard()
		return err
	}
	return nil
}

func (r *runner) dropSource(src string, move bool) error {
	if !move {
		return nil
	}
	return r.retry(src, func(fs FS) error { return fs.Remove(src) })
}

func (r *runner) copyBytes(src, tmp string, info os.FileInfo, made *creation) error {
	var in io.ReadCloser
	if err := r.retry(src, func(fs FS) error {
		opened, err := fs.Open(src)
		in = opened
		return err
	}); err != nil {
		return err
	}
	defer in.Close()

	var out Writer
	if err := r.retry(tmp, func(fs FS) error {
		created, err := fs.Create(tmp, info.Mode().Perm())
		if err != nil {
			return err
		}
		out = created
		made.fs = fs
		return nil
	}); err != nil {
		return err
	}

	if err := r.stream(src, in, out); err != nil {
		made.abort(out)
		return err
	}
	if err := out.Close(); err != nil {
		made.discard()
		return err
	}
	return nil
}

func (r *runner) preserve(src, dst string, info os.FileInfo, through FS) {
	if through == nil {
		through = local
	}
	if err := through.Chmod(dst, info.Mode().Perm(), false); err != nil {
		r.warn(dst, err)
	}
	if err := through.Chtimes(dst, info.ModTime()); err != nil {
		r.warn(dst, err)
	}
	if through != local {
		return
	}
	if err := copyXattrs(src, dst); err != nil {
		r.warn(dst, err)
	}
}

func (r *runner) growTotals(children int) {
	r.job.mu.Lock()
	r.job.progress.FilesTotal += children
	r.job.mu.Unlock()
}

func (r *runner) tempPath(dst string) string {
	return filepath.Join(filepath.Dir(dst), ".dfiles-tmp-"+r.job.id+r.job.nextID("-t")+"-"+truncateName(filepath.Base(dst), tempNameTail))
}

func truncateName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return name[:cut]
}

func inverseOf(move bool) Kind {
	if move {
		return KindMove
	}
	return KindDelete
}

func sameDevice(src, destDir string) bool {
	srcDev, ok := deviceOf(src)
	if !ok {
		return false
	}
	dstDev, ok := deviceOf(destDir)
	if !ok {
		return false
	}
	return srcDev == dstDev
}

func deviceOf(path string) (uint64, bool) {
	for {
		var st syscall.Stat_t
		if err := syscall.Lstat(path, &st); err == nil {
			return uint64(st.Dev), true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return 0, false
		}
		path = parent
	}
}
