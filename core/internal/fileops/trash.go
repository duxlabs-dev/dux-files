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

const trashInfoExt = ".trashinfo"

var listTrash = trash.List

func (r *runner) trash() error {
	sources := r.job.spec.sources
	count, bytes, complete := r.scan(sources)
	r.setTotals(count, bytes, !complete)

	return eachSource(r, sources, func(path string) error {
		var entry trash.Entry
		err := r.retryLocal(path, func() error {
			var putErr error
			entry, putErr = trash.Put(path)
			return putErr
		})
		if err != nil {
			return err
		}
		counted, measured := r.scanned[path]
		if entry.IsDir && measured {
			writeDirectorySize(entry, counted.bytes)
		}
		r.rememberPair(KindRestore, pair{src: entry.Name, dst: entry.TrashDir}, guard{})
		r.advance(path, max(counted.files, 1), counted.bytes)
		return nil
	})
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
		done, err := r.transfer(source, target, true)
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
	r.setTotals(len(entries), 0, false)

	dirs := map[string]bool{}
	err = eachSource(r, entries, func(entry trash.Entry) error {
		if err := r.retry(entry.FilesPath, func(fs FS) error { return fs.RemoveAll(entry.FilesPath) }); err != nil {
			return err
		}
		os.Remove(entry.InfoPath)
		dirs[entry.TrashDir] = true
		r.advance(entry.FilesPath, 1, entry.Size)
		return nil
	})
	if err != nil {
		return err
	}

	for dir := range dirs {
		os.Remove(filepath.Join(dir, "directorysizes"))
	}
	return nil
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
