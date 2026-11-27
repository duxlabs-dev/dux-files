package fileops

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-go/files"
)

type State string

const (
	StateQueued         State = "queued"
	StateRunning        State = "running"
	StatePausedConflict State = "paused-conflict"
	StatePausedError    State = "paused-error"
	StatePausedPassword State = "paused-password"
	StateCancelling     State = "cancelling"
	StateDone           State = "done"
	StateFailed         State = "failed"
)

type Kind string

const (
	KindCopy           Kind = "copy"
	KindMove           Kind = "move"
	KindRename         Kind = "rename"
	KindNewFolder      Kind = "newFolder"
	KindNewFile        Kind = "newFile"
	KindLink           Kind = "link"
	KindTrash          Kind = "trash"
	KindRestore        Kind = "restore"
	KindDelete         Kind = "delete"
	KindEmptyTrash     Kind = "emptyTrash"
	KindSetPermissions Kind = "setPermissions"
	KindSetOwner       Kind = "setOwner"
	KindCompress       Kind = "compress"
	KindExtract        Kind = "extract"
	KindDuplicate      Kind = "duplicate"
	KindNewFolderWith  Kind = "newFolderWith"
	KindPasteImage     Kind = "pasteImage"
	KindUnlockArchive  Kind = "unlockArchive"
)

type replay string

const (
	replayNone replay = ""
	replayUndo replay = "undo"
	replayRedo replay = "redo"
)

type Action string

const (
	ActionSkip     Action = "skip"
	ActionReplace  Action = "replace"
	ActionKeepBoth Action = "keepBoth"
	ActionMerge    Action = "merge"
	ActionRetry    Action = "retry"
	ActionCancel   Action = "cancel"
	ActionUnlock   Action = "unlock"

	ActionRetryElevated Action = "retryElevated"
)

type Progress struct {
	FilesDone   int    `json:"filesDone"`
	FilesTotal  int    `json:"filesTotal"`
	BytesDone   int64  `json:"bytesDone"`
	BytesTotal  int64  `json:"bytesTotal"`
	CurrentPath string `json:"currentPath"`
	BytesPerSec int64  `json:"bytesPerSec"`
	Estimating  bool   `json:"estimating"`
}

type Conflict struct {
	ID      string      `json:"id"`
	Source  files.Entry `json:"source"`
	Target  files.Entry `json:"target"`
	Actions []Action    `json:"actions"`
}

type Failure struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Escalatable bool     `json:"escalatable"`
	Actions     []Action `json:"actions"`
}

type PasswordPrompt struct {
	ID      string   `json:"id"`
	Path    string   `json:"path"`
	Wrong   bool     `json:"wrong"`
	Actions []Action `json:"actions"`
}

type Answer struct {
	ID         string
	Action     Action
	ApplyToAll bool
	Password   string
}

type View struct {
	ID             string          `json:"id"`
	Kind           Kind            `json:"kind"`
	State          State           `json:"state"`
	Scope          string          `json:"scope"`
	Sources        []string        `json:"sources"`
	Dest           string          `json:"dest,omitempty"`
	Progress       Progress        `json:"progress"`
	Conflict       *Conflict       `json:"conflict,omitempty"`
	Failure        *Failure        `json:"failure,omitempty"`
	PasswordPrompt *PasswordPrompt `json:"passwordPrompt,omitempty"`
	Skipped        int             `json:"skipped"`
	Warnings       int             `json:"warnings"`
	Cancelled      bool            `json:"cancelled"`
	Undoable       bool            `json:"undoable"`
	Redoable       bool            `json:"redoable"`
	Replay         string          `json:"replay,omitempty"`
	Created        []Created       `json:"created,omitempty"`
	Moved          []Moved         `json:"moved,omitempty"`
	Error          string          `json:"error,omitempty"`
	Code           string          `json:"code,omitempty"`
	StartedMs      int64           `json:"startedMs"`
	EndedMs        int64           `json:"endedMs,omitempty"`
}

type pair struct {
	src, dst string
	dir      bool
}

type Created struct {
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
}

type Moved struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type spec struct {
	kind      Kind
	scope     string
	sources   []string
	dest      string
	name      string
	template  string
	format    string
	password  string
	entries   []string
	layout    string
	toCache   bool
	mode      uint32
	uid       int
	gid       int
	recursive bool
	unique    bool
	pairs     []pair
	guards    []guard
	trashDir  string
	data      []byte
	replay    replay
	origin    *spec
	epoch     uint64
}

func (s spec) asOrigin() spec {
	s.replay = replayNone
	s.origin = nil
	s.data = nil
	s.epoch = 0
	return s
}

func (s spec) replayable() bool {
	switch {
	case s.kind == KindPasteImage:
		return false
	case s.kind == KindCompress && s.password != "":
		return false
	case s.toCache:
		return false
	}
	return true
}

type job struct {
	id     string
	spec   spec
	mgr    *Manager
	ctx    context.Context
	cancel context.CancelFunc

	answers chan Answer

	mu        sync.Mutex
	state     State
	progress  Progress
	conflict  *Conflict
	failure   *Failure
	password  *PasswordPrompt
	skipped   int
	warnings  int
	cancelled bool
	errMsg    string
	errCode   string
	startedMs int64
	endedMs   int64
	record    *undoRecord
	created   []Created
	nextEvent int
	pendingID string
	terminal  bool
	redoable  bool
	lastTick  time.Time
	rate      rateMeter
}

func (j *job) view() View {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.viewLocked()
}

func (j *job) viewLocked() View {
	return View{
		ID:             j.id,
		Kind:           j.spec.kind,
		State:          j.state,
		Scope:          j.spec.scope,
		Sources:        j.spec.sources,
		Dest:           j.spec.dest,
		Progress:       j.progress,
		Conflict:       j.conflict,
		Failure:        j.failure,
		PasswordPrompt: j.password,
		Skipped:        j.skipped,
		Warnings:       j.warnings,
		Cancelled:      j.cancelled,
		Undoable:       !j.record.empty() && j.spec.replay != replayUndo,
		Redoable:       j.redoable,
		Replay:         string(j.spec.replay),
		Created:        j.created,
		Moved:          j.movedLocked(),
		Error:          j.errMsg,
		Code:           j.errCode,
		StartedMs:      j.startedMs,
		EndedMs:        j.endedMs,
	}
}

func (j *job) movedLocked() []Moved {
	if !j.terminal || j.record == nil {
		return nil
	}
	switch j.record.kind {
	case KindMove, KindNewFolderWith:
	default:
		return nil
	}
	var moved []Moved
	for _, p := range j.record.pairs {
		if !p.dir {
			continue
		}
		moved = append(moved, Moved{From: p.dst, To: p.src})
	}
	return moved
}

func (j *job) setState(state State) {
	j.mu.Lock()
	if j.terminal {
		j.mu.Unlock()
		return
	}
	j.state = state
	if state == StateRunning && j.startedMs == 0 {
		j.startedMs = time.Now().UnixMilli()
	}
	view := j.viewLocked()
	j.mu.Unlock()
	j.mgr.publishJob(view)
}

func (j *job) nextID(prefix string) string {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.nextEvent++
	return prefix + strconv.Itoa(j.nextEvent)
}

func (j *job) closed() bool {
	select {
	case <-j.ctx.Done():
		return true
	default:
		return false
	}
}
