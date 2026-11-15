package fileops

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

const (
	maxConcurrent = 2
	finishedCap   = 20
	undoCap       = 50
	tickInterval  = 500 * time.Millisecond
)

type Publisher interface {
	Publish(topic string, data any)
}

const Topic = "jobs"

type event struct {
	Kind           string          `json:"kind"`
	JobID          string          `json:"jobId,omitempty"`
	Job            *View           `json:"job,omitempty"`
	Progress       *Progress       `json:"progress,omitempty"`
	Conflict       *Conflict       `json:"conflict,omitempty"`
	Failure        *Failure        `json:"failure,omitempty"`
	PasswordPrompt *PasswordPrompt `json:"passwordPrompt,omitempty"`
}

type undoRecord struct {
	kind     Kind
	pairs    []pair
	guards   []guard
	trashDir string
	dest     string
}

func (r *undoRecord) empty() bool {
	return r == nil || (len(r.pairs) == 0 && r.dest == "")
}

func (r *undoRecord) spec() *spec {
	if r.empty() {
		return nil
	}
	s := &spec{kind: r.kind, pairs: r.pairs, guards: r.guards, trashDir: r.trashDir, dest: r.dest}
	for _, p := range r.pairs {
		s.sources = append(s.sources, p.src)
	}
	return s
}

type undoEntry struct {
	inverse spec
	origin  *spec
}

type guard struct {
	size    int64
	mtimeMs int64
}

type ImageSource interface {
	Image() (data []byte, mime string, ok bool)
}

type Manager struct {
	bus    Publisher
	stater Stater
	slots  chan struct{}

	mu        sync.Mutex
	elevator  Elevator
	images    ImageSource
	nextID    int
	active    map[string]*job
	order     []string
	finished  []View
	undo      map[string][]undoEntry
	redo      map[string][]spec
	epoch     map[string]uint64
	portalKey string
}

func NewManager(bus Publisher, stater Stater) *Manager {
	return &Manager{
		bus:    bus,
		stater: stater,
		slots:  make(chan struct{}, maxConcurrent),
		active: map[string]*job{},
		undo:   map[string][]undoEntry{},
		redo:   map[string][]spec{},
		epoch:  map[string]uint64{},
	}
}

func (m *Manager) Forget(scope string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.undo, scope)
	delete(m.redo, scope)
	delete(m.epoch, scope)
}

func (m *Manager) SetElevator(e Elevator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.elevator = e
}

func (m *Manager) currentElevator() Elevator {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.elevator
}

func (m *Manager) SetImageSource(src ImageSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.images = src
}

func (m *Manager) imageSource() ImageSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.images
}

type Elevation struct {
	Available bool   `json:"available"`
	Route     string `json:"route,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (m *Manager) Elevation() Elevation {
	elevator := m.currentElevator()
	if elevator == nil {
		return Elevation{Reason: "NOROUTE"}
	}
	err := elevator.Available()
	if err == nil {
		return Elevation{Available: true, Route: elevator.Route()}
	}
	var unavailable *Unavailable
	if errors.As(err, &unavailable) {
		return Elevation{Route: elevator.Route(), Reason: unavailable.Reason}
	}
	return Elevation{Route: elevator.Route(), Reason: err.Error()}
}

func (m *Manager) elevationAvailable() bool {
	elevator := m.currentElevator()
	return elevator != nil && elevator.Available() == nil
}

func (m *Manager) enqueue(s spec) *job {
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.nextID++
	j := &job{
		id:      "j" + strconv.Itoa(m.nextID),
		spec:    s,
		mgr:     m,
		ctx:     ctx,
		cancel:  cancel,
		answers: make(chan Answer, 1),
		state:   StateQueued,
	}
	m.active[j.id] = j
	m.order = append(m.order, j.id)
	if s.replay == replayNone {
		delete(m.redo, s.scope)
		m.epoch[s.scope]++
	}
	m.mu.Unlock()

	m.publishJob(j.view())
	go m.run(j)
	return j
}

func (m *Manager) run(j *job) {
	defer j.cancel()

	select {
	case m.slots <- struct{}{}:
	case <-j.ctx.Done():
		j.finish(nil)
		return
	}
	defer func() { <-m.slots }()

	j.setState(StateRunning)
	j.finish(execute(j))
}

func (j *job) finish(err error) {
	j.mu.Lock()
	j.cancelled = j.cancelled || j.ctx.Err() != nil
	j.terminal = true
	j.endedMs = time.Now().UnixMilli()
	j.conflict = nil
	j.failure = nil
	j.password = nil
	j.pendingID = ""
	switch {
	case err != nil:
		j.state = StateFailed
		j.errMsg = err.Error()
		j.errCode = codeOf(err)
	default:
		j.state = StateDone
	}
	entry, redo := j.outcome()
	scope := j.spec.scope
	j.mu.Unlock()

	redoable := false
	switch {
	case entry != nil:
		j.mgr.pushUndo(scope, *entry)
	case redo != nil:
		redoable = j.mgr.pushRedo(scope, *redo)
	}

	j.mu.Lock()
	j.redoable = redoable
	view := j.viewLocked()
	j.mu.Unlock()
	j.mgr.retire(view)
	j.mgr.publishJob(view)
}

func (j *job) outcome() (*undoEntry, *spec) {
	inverse := j.record.spec()
	if j.spec.replay != replayUndo {
		if inverse == nil {
			return nil, nil
		}
		entry := &undoEntry{inverse: *inverse}
		if j.spec.replayable() {
			origin := j.spec.asOrigin()
			entry.origin = &origin
		}
		return entry, nil
	}
	clean := j.state == StateDone && !j.cancelled && j.skipped == 0
	switch {
	case inverse != nil:
		inverse.epoch = j.spec.epoch
		return nil, inverse
	case clean && j.spec.origin != nil:
		origin := *j.spec.origin
		origin.epoch = j.spec.epoch
		return nil, &origin
	}
	return nil, nil
}

func (m *Manager) retire(view View) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, view.ID)
	for i, id := range m.order {
		if id != view.ID {
			continue
		}
		m.order = append(m.order[:i], m.order[i+1:]...)
		break
	}
	m.finished = append(m.finished, view)
	if len(m.finished) > finishedCap {
		m.finished = m.finished[len(m.finished)-finishedCap:]
	}
}

func (m *Manager) pushUndo(scope string, entry undoEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.undo[scope] = capped(append(m.undo[scope], entry))
}

func (m *Manager) pushRedo(scope string, s spec) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.epoch != m.epoch[scope] {
		return false
	}
	m.redo[scope] = capped(append(m.redo[scope], s))
	return true
}

func capped[T any](stack []T) []T {
	if len(stack) <= undoCap {
		return stack
	}
	drop := len(stack) - undoCap
	clear(stack[:drop])
	return stack[drop:]
}

func pop[T any](stacks map[string][]T, scope string) (T, bool) {
	stack := stacks[scope]
	if len(stack) == 0 {
		delete(stacks, scope)
		var none T
		return none, false
	}
	top := stack[len(stack)-1]
	clear(stack[len(stack)-1:])
	if len(stack) == 1 {
		delete(stacks, scope)
		return top, true
	}
	stacks[scope] = stack[:len(stack)-1]
	return top, true
}

func (m *Manager) Undo(scope string) (View, error) {
	m.mu.Lock()
	entry, ok := pop(m.undo, scope)
	epoch := m.epoch[scope]
	m.mu.Unlock()
	if !ok {
		return View{}, errors.New("nothing to undo")
	}

	s := entry.inverse
	s.scope = scope
	s.replay = replayUndo
	s.origin = entry.origin
	s.epoch = epoch
	return m.enqueue(s).view(), nil
}

func (m *Manager) Redo(scope string) (View, error) {
	m.mu.Lock()
	s, ok := pop(m.redo, scope)
	m.mu.Unlock()
	if !ok {
		return View{}, errors.New("nothing to redo")
	}

	s.scope = scope
	s.replay = replayRedo
	return m.enqueue(s).view(), nil
}

func (m *Manager) Jobs() []View {
	m.mu.Lock()
	defer m.mu.Unlock()

	views := make([]View, 0, len(m.order)+len(m.finished))
	for _, id := range m.order {
		views = append(views, m.active[id].view())
	}
	return append(views, m.finished...)
}

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	j, ok := m.active[id]
	m.mu.Unlock()
	if !ok {
		return errors.New("unknown job: " + id)
	}

	j.mu.Lock()
	if j.terminal {
		j.mu.Unlock()
		return errors.New("job already finished: " + id)
	}
	j.cancelled = true
	j.mu.Unlock()

	j.setState(StateCancelling)
	j.cancel()
	return nil
}

func (m *Manager) Respond(jobID string, answer Answer) error {
	m.mu.Lock()
	j, ok := m.active[jobID]
	m.mu.Unlock()
	if !ok {
		return errors.New("unknown job: " + jobID)
	}

	j.mu.Lock()
	claimed := j.pendingID != "" && j.pendingID == answer.ID
	if claimed {
		j.pendingID = ""
	}
	j.mu.Unlock()
	if !claimed {
		return errors.New("job is not waiting for that answer: " + jobID)
	}

	select {
	case j.answers <- answer:
		return nil
	case <-j.ctx.Done():
		return errors.New("job is no longer running: " + jobID)
	}
}

func (m *Manager) publish(payload event) {
	if m.bus == nil {
		return
	}
	m.bus.Publish(Topic, payload)
}

func (m *Manager) publishJob(view View) {
	m.publish(event{Kind: "job", JobID: view.ID, Job: &view})
}

func (j *job) ask(payload event, state State) Answer {
	j.mu.Lock()
	j.conflict = payload.Conflict
	j.failure = payload.Failure
	j.password = payload.PasswordPrompt
	j.pendingID = payload.answerID()
	j.state = state
	view := j.viewLocked()
	j.mu.Unlock()

	j.mgr.publish(payload)
	j.mgr.publishJob(view)

	for {
		select {
		case answer := <-j.answers:
			if answer.ID != payload.answerID() {
				continue
			}
			j.mu.Lock()
			j.conflict = nil
			j.failure = nil
			j.password = nil
			j.pendingID = ""
			j.rate.restart(time.Now(), j.progress.BytesDone)
			j.mu.Unlock()
			j.setState(StateRunning)
			if answer.Action == ActionCancel {
				j.cancel()
			}
			return answer
		case <-j.ctx.Done():
			j.mu.Lock()
			j.pendingID = ""
			j.mu.Unlock()
			return Answer{Action: ActionCancel}
		}
	}
}

func (e event) answerID() string {
	switch {
	case e.Conflict != nil:
		return e.Conflict.ID
	case e.Failure != nil:
		return e.Failure.ID
	case e.PasswordPrompt != nil:
		return e.PasswordPrompt.ID
	}
	return ""
}

func (j *job) tick(force bool) {
	j.mu.Lock()
	now := time.Now()
	if !force && now.Sub(j.lastTick) < tickInterval {
		j.mu.Unlock()
		return
	}
	j.progress.BytesPerSec = j.rate.sample(now, j.progress.BytesDone)
	j.lastTick = now
	progress := j.progress
	j.mu.Unlock()

	j.mgr.publish(event{Kind: "progress", JobID: j.id, Progress: &progress})
}
