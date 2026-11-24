package fileops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf8"
)

var copyChunk = 256 << 10

const (
	tempNameTail = 64
	workers      = 2
)

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

	if !move && r.job.spec.replay == replayNone {
		pairs = r.copyPairsTogether(pairs)
	}
	return eachSource(r, pairs, func(p pair) error {
		if !r.guarded(p.src) {
			return errors.New("changed since the operation it would replay: " + p.src)
		}
		done, err := r.transfer(p.src, p.dst, move, false)
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

func (r *runner) transfer(src, dst string, move, intoFresh bool) (outcome, error) {
	var info os.FileInfo
	if err := r.retry(src, func(fs FS) error {
		var statErr error
		info, statErr = fs.Lstat(src)
		return statErr
	}); err != nil {
		return outcome{}, err
	}

	res := resolution{dst: dst}
	if !intoFresh {
		res = r.resolve(src, dst, info, move)
	}
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
		return outcome{dst: dst, undoable: undoable, dir: true}, r.transferDir(src, dst, info, move, res.merge)
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
	if !res.replaced {
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

func (r *runner) transferDir(src, dst string, info os.FileInfo, move, merge bool) error {
	dstFS := local
	fresh := false
	if err := r.retry(dst, func(fs FS) error {
		dstFS = fs
		var mkErr error
		fresh, mkErr = makeDir(fs, dst, info.Mode().Perm()|0o700)
		return mkErr
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

	if fresh && !merge && !move && srcFS == local && dstFS == local {
		children = r.copyFilesTogether(src, dst, children)
	}
	for _, child := range children {
		if r.job.closed() {
			return context.Canceled
		}
		_, err := r.transfer(filepath.Join(src, child.Name()), filepath.Join(dst, child.Name()), move, fresh && !merge)
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

func (r *runner) copyPairsTogether(pairs []pair) []pair {
	if len(pairs) < workers || r.noUnnamed.Load() {
		return pairs
	}
	group, ok := r.openBatch(filepath.Dir(pairs[0].src), filepath.Dir(pairs[0].dst))
	if !ok {
		return pairs
	}
	defer group.close()

	placed := r.together(len(pairs), func(i int) bool {
		p := pairs[i]
		if filepath.Dir(p.src) != group.src || filepath.Dir(p.dst) != group.dst {
			return false
		}
		if !group.place(filepath.Base(p.src), filepath.Base(p.dst)) {
			return false
		}
		r.remember(KindDelete, p.dst, p.src)
		if r.job.spec.kind == KindDuplicate {
			r.noteCreated(p.dst, false)
		}
		return true
	})
	return unplaced(pairs, placed)
}

func (r *runner) copyFilesTogether(src, dst string, children []os.DirEntry) []os.DirEntry {
	if len(children) < workers || r.noUnnamed.Load() {
		return children
	}
	group, ok := r.openBatch(src, dst)
	if !ok {
		return children
	}
	defer group.close()

	placed := r.together(len(children), func(i int) bool {
		name := children[i].Name()
		return children[i].Type().IsRegular() && group.place(name, name)
	})
	return unplaced(children, placed)
}

func (r *runner) together(count int, place func(int) bool) []bool {
	placed := make([]bool, count)
	var next atomic.Int64
	var crew sync.WaitGroup
	for range workers {
		crew.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= count || r.job.closed() {
					return
				}
				placed[i] = place(i)
			}
		})
	}
	crew.Wait()
	return placed
}

func unplaced[T any](items []T, placed []bool) []T {
	rest := make([]T, 0, len(items))
	for i, item := range items {
		if !placed[i] {
			rest = append(rest, item)
		}
	}
	return rest
}

func makeDir(fs FS, dst string, perm os.FileMode) (bool, error) {
	if fs.Mkdir(dst, perm) == nil {
		return true, nil
	}
	return false, fs.MkdirAll(dst, perm)
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

	if err := r.placeLink(target, dst, noReplace); err != nil {
		return err
	}

	r.advance(dst, 1, 0)
	return r.dropSource(src, move)
}

func (r *runner) placeLink(target, dst string, noReplace bool) error {
	if noReplace {
		return r.retry(dst, func(fs FS) error { return fs.Symlink(target, dst) })
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
	return r.commit(tmp, dst, made, false)
}

func (r *runner) transferFile(src, dst string, info os.FileInfo, move, noReplace bool) error {
	if err := r.place(src, dst, info, noReplace); err != nil {
		return err
	}

	r.advance(dst, 1, 0)
	return r.dropSource(src, move)
}

func (r *runner) place(src, dst string, info os.FileInfo, noReplace bool) error {
	placed, err := r.placeUnnamed(src, dst, info, noReplace)
	if placed || err != nil {
		return err
	}
	return r.placeNamed(src, dst, info, noReplace)
}

func (r *runner) placeUnnamed(src, dst string, info os.FileInfo, noReplace bool) (bool, error) {
	if !noReplace || r.noUnnamed.Load() {
		return false, nil
	}
	placed := false
	err := r.retryLocal(src, func() error {
		var copyErr error
		placed, copyErr = r.copyUnnamed(src, dst, info)
		return copyErr
	})
	return placed, err
}

func (r *runner) placeNamed(src, dst string, info os.FileInfo, noReplace bool) error {
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
	return r.commit(tmp, dst, made, noReplace)
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

	if err := r.pump(src, in, out, info.Size()); err != nil {
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
	tail := strings.TrimRight(truncateName(filepath.Base(dst), tempNameTail), " .")
	return filepath.Join(filepath.Dir(dst), ".dfiles-tmp-"+r.job.id+r.job.nextID("-t")+"-"+tail)
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
