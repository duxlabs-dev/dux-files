package fileops

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/duxlabs-dev/dux-files/core/internal/helper"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/log"
)

var errSkipped = errors.New("skipped")

func eachSource[T any](r *runner, items []T, step func(T) error) error {
	for _, item := range items {
		if r.job.closed() {
			return nil
		}
		switch err := step(item); {
		case errors.Is(err, errSkipped):
			continue
		case errors.Is(err, context.Canceled):
			return nil
		case err != nil:
			return err
		}
	}
	return nil
}

func codeOf(err error) string {
	return string(helper.CodeOf(err))
}

type Stater interface {
	Stat(path string) (files.Entry, error)
}

type runner struct {
	job         *job
	stater      Stater
	conflictAll Action
	errorAll    Action
	elevateRest bool
	password    string
	scanned     map[string]scanResult
	estimating  bool
}

func execute(j *job) error {
	r := &runner{job: j, stater: j.mgr.stater, scanned: map[string]scanResult{}}
	defer j.tick(true)

	switch j.spec.kind {
	case KindCopy:
		return r.transferAll(false)
	case KindMove:
		return r.transferAll(true)
	case KindRename:
		return r.rename()
	case KindNewFolder:
		return r.newFolder()
	case KindNewFile:
		return r.newFile()
	case KindLink:
		return r.link()
	case KindTrash:
		return r.trash()
	case KindRestore:
		return r.restore()
	case KindDelete:
		return r.deleteAll()
	case KindEmptyTrash:
		return r.emptyTrash()
	case KindSetPermissions:
		return r.setPermissions()
	case KindSetOwner:
		return r.setOwner()
	case KindCompress:
		return r.compress()
	case KindExtract:
		return r.extract()
	case KindDuplicate:
		return r.transferAll(false)
	case KindNewFolderWith:
		return r.newFolderWith()
	case KindPasteImage:
		return r.pasteImage()
	}
	return errors.New("unknown operation: " + string(j.spec.kind))
}

func (r *runner) remember(kind Kind, from, to string) {
	p := pair{src: from, dst: to}
	g := guard{}
	if info, err := os.Lstat(from); err == nil {
		p.dir = info.IsDir()
		g = guard{size: info.Size(), mtimeMs: info.ModTime().UnixMilli()}
	}
	r.rememberPair(kind, p, g)
}

func (r *runner) rememberPair(kind Kind, p pair, g guard) {
	r.job.mu.Lock()
	defer r.job.mu.Unlock()
	if r.job.record == nil {
		r.job.record = &undoRecord{kind: kind}
	}
	r.job.record.pairs = append(r.job.record.pairs, p)
	r.job.record.guards = append(r.job.record.guards, g)
}

func guardOf(path string) guard {
	info, err := os.Lstat(path)
	if err != nil {
		return guard{}
	}
	return guard{size: info.Size(), mtimeMs: info.ModTime().UnixMilli()}
}

func (g guard) matches(path string) bool {
	if g.mtimeMs == 0 {
		return true
	}
	current := guardOf(path)
	return current.mtimeMs == g.mtimeMs && current.size == g.size
}

func (r *runner) retry(path string, step func(FS) error) error {
	return r.attempt(path, step, true)
}

func (r *runner) retryLocal(path string, step func() error) error {
	return r.attempt(path, func(FS) error { return step() }, false)
}

func (r *runner) attempt(path string, step func(FS) error, mayElevate bool) error {
	elevated := false
	for {
		err := step(r.fs(elevated))
		if err == nil {
			return nil
		}
		if errors.Is(err, errSkipped) || errors.Is(err, context.Canceled) {
			return err
		}
		if r.job.closed() {
			return context.Canceled
		}

		offer := mayElevate && r.canElevate(err)
		switch r.answerError(path, err, offer, !elevated) {
		case ActionRetry:
			elevated = false
		case ActionRetryElevated:
			elevated = offer
		case ActionSkip:
			r.skip()
			return errSkipped
		default:
			return context.Canceled
		}
	}
}

func (r *runner) fs(elevated bool) FS {
	if !elevated {
		return local
	}
	if elevator := r.job.mgr.currentElevator(); elevator != nil {
		return elevator
	}
	return local
}

func (r *runner) canElevate(err error) bool {
	if codeOf(err) != string(files.CodeAccess) {
		return false
	}
	return r.job.mgr.elevationAvailable()
}

func (r *runner) answerError(path string, err error, offer, firstTry bool) Action {
	switch {
	case r.errorAll != "":
		return r.errorAll
	case r.elevateRest && offer && firstTry:
		return ActionRetryElevated
	}

	code := codeOf(err)
	failure := &Failure{
		ID:          r.job.nextID("e"),
		Path:        path,
		Code:        code,
		Message:     err.Error(),
		Escalatable: offer,
		Actions:     []Action{ActionRetry, ActionSkip, ActionCancel},
	}
	if offer {
		failure.Actions = []Action{ActionRetryElevated, ActionRetry, ActionSkip, ActionCancel}
	}

	answer := r.job.ask(event{Kind: "error", JobID: r.job.id, Failure: failure}, StatePausedError)
	switch {
	case answer.Action == ActionRetryElevated:
		r.elevateRest = true
	case answer.ApplyToAll && answer.Action != ActionRetry:
		r.errorAll = answer.Action
	}
	return answer.Action
}

func (r *runner) warn(path string, err error) {
	r.job.mu.Lock()
	r.job.warnings++
	r.job.mu.Unlock()
	log.Debugf("fileops %s: %s: %v", r.job.id, path, err)
}

func (r *runner) skip() {
	r.job.mu.Lock()
	r.job.skipped++
	r.job.mu.Unlock()
}

func (r *runner) advance(path string, count int, bytes int64) {
	r.job.mu.Lock()
	r.job.progress.FilesDone += count
	r.job.progress.BytesDone += bytes
	if path != "" {
		r.job.progress.CurrentPath = path
	}
	if r.estimating {
		r.job.progress.FilesTotal = max(r.job.progress.FilesTotal, r.job.progress.FilesDone)
		r.job.progress.BytesTotal = max(r.job.progress.BytesTotal, r.job.progress.BytesDone)
	}
	r.job.mu.Unlock()
	r.job.tick(false)
}

func (r *runner) setTotals(count int, bytes int64, estimating bool) {
	r.estimating = estimating
	r.job.mu.Lock()
	r.job.progress.FilesTotal = count
	r.job.progress.BytesTotal = bytes
	r.job.progress.Estimating = estimating
	r.job.mu.Unlock()
	r.job.tick(true)
}

func (r *runner) targets() []pair {
	if len(r.job.spec.pairs) > 0 {
		return r.job.spec.pairs
	}
	dest := r.job.spec.dest
	pairs := make([]pair, 0, len(r.job.spec.sources))
	for _, src := range r.job.spec.sources {
		pairs = append(pairs, pair{src: src, dst: filepath.Join(dest, filepath.Base(src))})
	}
	return pairs
}
