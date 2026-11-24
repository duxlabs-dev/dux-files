package fileops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
)

type resolution struct {
	dst      string
	skip     bool
	merge    bool
	replaced bool
	abort    bool
}

func (r *runner) resolve(src, dst string, info os.FileInfo, move bool) resolution {
	target, err := os.Lstat(dst)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return resolution{dst: dst}
	case err != nil:
		return resolution{dst: dst}
	case os.SameFile(info, target) && move:
		return resolution{skip: true}
	case os.SameFile(info, target):
		return resolution{dst: uniquePath(dst)}
	}

	switch r.conflictAction(src, dst, info.IsDir() && target.IsDir()) {
	case ActionReplace:
		return resolution{dst: dst, replaced: true}
	case ActionMerge:
		return resolution{dst: dst, merge: true}
	case ActionKeepBoth:
		return resolution{dst: uniquePath(dst)}
	case ActionSkip:
		r.skip()
		return resolution{skip: true}
	default:
		return resolution{abort: true}
	}
}

func (r *runner) conflictAction(src, dst string, bothDirs bool) Action {
	forTheRest := &r.conflictAll
	if bothDirs {
		forTheRest = &r.folderConflictAll
	}
	if *forTheRest != "" {
		return *forTheRest
	}

	answer := r.askConflict(src, dst, bothDirs)
	if answer.ApplyToAll {
		*forTheRest = answer.Action
	}
	return answer.Action
}

func (r *runner) askConflict(src, dst string, bothDirs bool) Answer {
	conflict := &Conflict{
		ID:      r.job.nextID("c"),
		Source:  r.describe(src),
		Target:  r.describe(dst),
		Actions: []Action{ActionSkip, ActionReplace, ActionKeepBoth, ActionCancel},
	}
	if bothDirs {
		conflict.Actions = []Action{ActionMerge, ActionSkip, ActionReplace, ActionKeepBoth, ActionCancel}
	}

	return r.job.ask(event{Kind: "conflict", JobID: r.job.id, Conflict: conflict}, StatePausedConflict)
}

func (r *runner) describe(path string) files.Entry {
	if r.stater == nil {
		return files.Entry{Path: path, Name: filepath.Base(path)}
	}
	entry, err := r.stater.Stat(path)
	if err != nil {
		return files.Entry{Path: path, Name: filepath.Base(path)}
	}
	return entry
}

func uniquePath(path string) string {
	dir := filepath.Dir(path)
	stem, ext := splitName(filepath.Base(path))

	for i := 1; ; i++ {
		suffix := " (copy)"
		if i > 1 {
			suffix = " (copy " + strconv.Itoa(i) + ")"
		}
		candidate := filepath.Join(dir, stem+suffix+ext)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}

func numberedPath(path string) string {
	if _, err := os.Lstat(path); err != nil {
		return path
	}
	dir := filepath.Dir(path)
	stem, ext := splitName(filepath.Base(path))

	for i := 2; ; i++ {
		candidate := filepath.Join(dir, stem+" "+strconv.Itoa(i)+ext)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}

func splitName(name string) (stem, ext string) {
	if strings.HasPrefix(name, ".") {
		rest := name[1:]
		dot := strings.LastIndex(rest, ".")
		if dot <= 0 {
			return name, ""
		}
		return name[:dot+1], rest[dot:]
	}
	ext = filepath.Ext(name)
	stem = strings.TrimSuffix(name, ext)
	if tar := filepath.Ext(stem); strings.EqualFold(tar, ".tar") {
		return strings.TrimSuffix(stem, tar), tar + ext
	}
	return stem, ext
}
