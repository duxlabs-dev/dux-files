package fileops

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"

	"github.com/duxlabs-dev/dux-files/core/internal/atomicfile"

	"github.com/duxlabs-dev/dux-go/paths"
	"github.com/duxlabs-dev/dux-go/trash"
)

const (
	trashInfoExt = ".trashinfo"
	expungedName = "expunged"
)

var listTrash = trash.List

func (r *runner) trash() error {
	sources := r.job.spec.sources
	r.setTotals(len(sources), 0, false)

	var dirs []trash.Entry
	err := eachSource(r, sources, func(path string) error {
		var entry trash.Entry
		err := r.retryLocal(path, func() error {
			var putErr error
			entry, putErr = trash.Put(path)
			return putErr
		})
		if err != nil {
			return err
		}
		if entry.IsDir {
			dirs = append(dirs, entry)
		}
		r.rememberPair(KindRestore, pair{src: entry.Name, dst: entry.TrashDir}, guard{})
		r.advance(path, 1, 0)
		return nil
	})
	r.recordDirectorySizes(dirs)
	return err
}

func (r *runner) recordDirectorySizes(dirs []trash.Entry) {
	paths := make([]string, 0, len(dirs))
	for _, entry := range dirs {
		paths = append(paths, entry.FilesPath)
	}
	r.scan(paths)

	for _, entry := range dirs {
		measured, complete := r.scanned[entry.FilesPath]
		if !complete {
			continue
		}
		writeDirectorySize(entry, measured.bytes)
	}
}

func (r *runner) restore() error {
	items := r.restoreItems()
	r.setTotals(len(items), 0, false)

	return eachSource(r, items, func(item pair) error {
		source := filepath.Join(item.dst, "files", item.src)
		var target string
		err := r.retryLocal(source, func() error {
			var resolveErr error
			target, resolveErr = restoreTarget(item.dst, item.src)
			if resolveErr != nil {
				return resolveErr
			}
			return os.MkdirAll(filepath.Dir(target), 0o755)
		})
		if err != nil {
			return err
		}
		done, err := r.transfer(source, target, true, false)
		if err != nil {
			return err
		}
		os.Remove(filepath.Join(item.dst, "info", item.src+trashInfoExt))
		dropDirectorySize(item.dst, item.src)
		if done.undoable {
			r.rememberPair(KindTrash, pair{src: done.dst, dst: ""}, guard{})
		}
		return nil
	})
}

func (r *runner) restoreItems() []pair {
	if len(r.job.spec.pairs) > 0 {
		return r.job.spec.pairs
	}
	dir := r.job.spec.trashDir
	items := make([]pair, 0, len(r.job.spec.sources))
	for _, name := range r.job.spec.sources {
		items = append(items, pair{src: name, dst: dir})
	}
	return items
}

func restoreTarget(trashDir, name string) (string, error) {
	body, err := os.ReadFile(filepath.Join(trashDir, "info", name+trashInfoExt))
	if err != nil {
		return "", err
	}

	stored := ""
	for _, line := range strings.Split(string(body), "\n") {
		value, ok := strings.CutPrefix(line, "Path=")
		if !ok {
			continue
		}
		stored = value
		break
	}
	if stored == "" {
		return "", errors.New("invalid .trashinfo: missing Path")
	}
	if decoded, err := url.PathUnescape(stored); err == nil {
		stored = decoded
	}
	target, ok := RestorePath(trashDir, stored)
	if !ok || (trashDir != HomeTrash() && !resolvesInside(target, trashTopDir(trashDir))) {
		return "", errors.New("invalid .trashinfo: Path leaves its volume: " + stored)
	}
	return target, nil
}

func RestorePath(trashDir, stored string) (string, bool) {
	if stored == "" {
		return "", false
	}
	topDir := trashTopDir(trashDir)
	if filepath.IsAbs(stored) {
		target := filepath.Clean(stored)
		return target, trashDir == HomeTrash() || pathutil.Under(target, topDir)
	}
	target := filepath.Join(topDir, stored)
	return target, pathutil.Under(target, topDir)
}

func trashTopDir(trashDir string) string {
	topDir := filepath.Dir(trashDir)
	if filepath.Base(topDir) == ".Trash" {
		return filepath.Dir(topDir)
	}
	return topDir
}

func HomeTrash() string {
	return filepath.Join(paths.XDGDataHome(), "Trash")
}

func resolvesInside(target, topDir string) bool {
	root, err := filepath.EvalSymlinks(topDir)
	if err != nil {
		return false
	}
	existing := filepath.Dir(target)
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return false
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return false
	}
	return resolved == root || pathutil.Under(resolved, root)
}

func forgetTrashed(path string) {
	filesDir := filepath.Dir(path)
	trashDir := filepath.Dir(filesDir)
	if filepath.Base(filesDir) != "files" || !isTrashDir(trashDir) {
		return
	}
	name := filepath.Base(path)
	os.Remove(filepath.Join(trashDir, "info", name+trashInfoExt))
	dropDirectorySize(trashDir, name)
}

func isTrashDir(dir string) bool {
	uid := strconv.Itoa(os.Getuid())
	switch {
	case dir == HomeTrash():
		return true
	case filepath.Base(dir) == ".Trash-"+uid:
		return true
	default:
		return filepath.Base(dir) == uid && filepath.Base(filepath.Dir(dir)) == ".Trash"
	}
}

func (r *runner) emptyTrash() error {
	entries, err := listTrash()
	if err != nil {
		return err
	}

	dirs := map[string]bool{}
	var doomed []condemned
	for _, entry := range entries {
		if !dirs[entry.TrashDir] {
			dirs[entry.TrashDir] = true
			doomed = append(doomed, strays(entry.TrashDir)...)
		}
		doomed = append(doomed, r.setAside(entry))
	}
	r.setTotals(len(doomed), 0, false)

	err = eachSource(r, doomed, func(c condemned) error {
		if _, err := r.remove(c.path, false); err != nil {
			return err
		}
		os.Remove(c.info)
		r.advance(c.path, 1, c.size)
		return nil
	})
	spare(doomed)
	if err != nil {
		return err
	}

	for dir := range dirs {
		os.Remove(filepath.Join(dir, "directorysizes"))
		os.Remove(filepath.Join(dir, expungedName))
	}
	return nil
}

type condemned struct {
	path, info     string
	home, homeInfo string
	size           int64
}

func (r *runner) setAside(entry trash.Entry) condemned {
	spot := condemned{path: entry.FilesPath, info: entry.InfoPath, size: entry.Size}
	dir := filepath.Join(entry.TrashDir, expungedName)
	aside := filepath.Join(dir, r.job.id+r.job.nextID("-x"))
	if os.MkdirAll(dir, 0o700) != nil || os.Rename(entry.FilesPath, aside) != nil {
		return spot
	}
	spot.path, spot.home = aside, entry.FilesPath
	if os.Rename(entry.InfoPath, aside+trashInfoExt) != nil {
		return spot
	}
	spot.info, spot.homeInfo = aside+trashInfoExt, entry.InfoPath
	return spot
}

func strays(trashDir string) []condemned {
	dir := filepath.Join(trashDir, expungedName)
	children, _ := os.ReadDir(dir)
	left := make([]condemned, 0, len(children))
	for _, child := range children {
		path := filepath.Join(dir, child.Name())
		if _, err := os.Lstat(strings.TrimSuffix(path, trashInfoExt)); err == nil && strings.HasSuffix(path, trashInfoExt) {
			continue
		}
		left = append(left, condemned{path: path, info: path + trashInfoExt})
	}
	return left
}

func spare(doomed []condemned) {
	for _, c := range doomed {
		if c.home == "" {
			continue
		}
		if _, err := os.Lstat(c.path); err != nil {
			continue
		}
		if os.Rename(c.path, c.home) != nil || c.homeInfo == "" {
			continue
		}
		os.Rename(c.info, c.homeInfo)
	}
}

func directorySizesPath(trashDir string) string {
	return filepath.Join(trashDir, "directorysizes")
}

func writeDirectorySize(entry trash.Entry, size int64) {
	info, err := os.Lstat(entry.InfoPath)
	if err != nil {
		return
	}
	line := strconv.FormatInt(size, 10) + " " + strconv.FormatInt(info.ModTime().Unix(), 10) + " " + url.PathEscape(entry.Name)
	rewriteDirectorySizes(entry.TrashDir, entry.Name, line)
}

func dropDirectorySize(trashDir, name string) {
	rewriteDirectorySizes(trashDir, name, "")
}

var directorySizesLock sync.Mutex

func rewriteDirectorySizes(trashDir, name, line string) {
	directorySizesLock.Lock()
	defer directorySizesLock.Unlock()

	path := directorySizesPath(trashDir)
	body, _ := os.ReadFile(path)

	kept := make([]string, 0, 8)
	for _, existing := range strings.Split(string(body), "\n") {
		if existing == "" || directorySizeName(existing) == name {
			continue
		}
		kept = append(kept, existing)
	}
	if line != "" {
		kept = append(kept, line)
	}
	if len(kept) == 0 {
		os.Remove(path)
		return
	}

	_ = atomicfile.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o600)
}

func directorySizeName(line string) string {
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 3 {
		return ""
	}
	name, err := url.PathUnescape(fields[2])
	if err != nil {
		return fields[2]
	}
	return name
}
