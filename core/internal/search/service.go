package search

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-go/files"
)

const (
	MaxResults    = 10000
	MaxConcurrent = 8
	topicPrefix   = "search:"
)

type Publisher interface {
	Publish(topic string, data any)
}

type Stater interface {
	Stat(path string) (files.Entry, error)
}

type Service struct {
	bus          Publisher
	stater       Stater
	workers      int
	attachOnOpen atomic.Bool
	index        indexBackend
	tools        toolbox
	mounts       func() []string
	excludes     func() []string

	running  sync.WaitGroup
	mu       sync.Mutex
	closed   bool
	searches map[string]*search
	nextID   int
}

func NewService(bus Publisher, stater Stater) *Service {
	return &Service{
		bus:      bus,
		stater:   stater,
		workers:  min(4, runtime.NumCPU()),
		searches: map[string]*search{},
		index:    indexBackend{now: time.Now, infoWait: infoTimeout, searchWait: searchTimeout},
		tools:    newToolbox(),
	}
}

func (s *Service) SetIndexEnabled(enabled bool) { s.index.enabled.Store(enabled) }

func (s *Service) Tools() []string {
	found := s.tools.locate()
	var names []string
	if found.fd != "" {
		names = append(names, "fd")
	}
	if found.rg != "" {
		names = append(names, "rg")
	}
	return names
}

func (s *Service) SetMounts(mounts func() []string) { s.mounts = mounts }

func (s *Service) SetExcludes(excludes func() []string) { s.excludes = excludes }

func (s *Service) Attach(topics []string) {
	for _, topic := range topics {
		id, ok := strings.CutPrefix(topic, topicPrefix)
		if !ok {
			continue
		}
		if sc, live := s.byID(id); live {
			sc.attach()
		}
	}
}

func (s *Service) AttachOnOpen() { s.attachOnOpen.Store(true) }

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	all := make([]*search, 0, len(s.searches))
	for _, sc := range s.searches {
		all = append(all, sc)
	}
	s.mu.Unlock()
	for _, sc := range all {
		sc.cancel()
	}
	s.running.Wait()
}

func (s *Service) Start(ctx context.Context, root, query string, opts Options) (id, topic string, err error) {
	if err := validate(root, query, opts); err != nil {
		return "", "", err
	}
	root = filepath.Clean(root)

	sc := newSearch(s)
	if err := s.register(sc); err != nil {
		return "", "", err
	}
	sc.follow(ctx)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		sc.cancel()
		return "", "", errors.New("search service is shutting down")
	}
	s.running.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.running.Done()
		sc.run(root, query, opts)
	}()
	return sc.id, sc.topic, nil
}

func (s *Service) Cancel(id string) bool {
	sc, ok := s.byID(id)
	if !ok {
		return false
	}
	sc.cancel()
	sc.mu.Lock()
	done := sc.done
	sc.mu.Unlock()
	if done {
		sc.drop()
	}
	return true
}

func (s *Service) register(sc *search) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.searches) >= MaxConcurrent {
		return &files.Error{Code: files.CodeBusy, Err: errors.New("too many searches running")}
	}
	s.nextID++
	sc.id = "s" + strconv.Itoa(s.nextID)
	sc.topic = topicPrefix + sc.id
	s.searches[sc.id] = sc
	return nil
}

func (s *Service) unregister(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.searches, id)
}

func (s *Service) byID(id string) (*search, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.searches[id]
	return sc, ok
}

func (s *Service) publish(topic string, payload any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(topic, payload)
}
