package search

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxlabs-dev/dux-go/log"
)

const (
	toolWaitDelay  = time.Second
	toolUsageError = 2
	maxPathBytes   = 1 << 20
)

func (sc *search) find(out *sink, fd string, targets []string, m matcher, backend event) {
	backend.Backend = "fd"
	sc.publish(backend)
	rules := sc.skipRules(out.opts.IncludeIgnored)

	var unreadable atomic.Int64
	var lock sync.Mutex
	var broken []string
	var running sync.WaitGroup
	for _, root := range targets {
		running.Go(func() {
			count, err := sc.findRoot(out, fd, root, m, rules)
			if err == nil || sc.ctx.Err() != nil {
				unreadable.Add(int64(count))
				return
			}
			if !sc.svc.tools.logged.Swap(true) {
				log.Warnf("fd failed, searching with the walk instead: %v", err)
			}
			lock.Lock()
			broken = append(broken, root)
			lock.Unlock()
		})
	}
	running.Wait()
	if len(broken) > 0 {
		unreadable.Add(int64(sc.walkInto(out, broken, m)))
	}
	out.flush()
	out.em.finish(false, int(unreadable.Load()))
}

func (sc *search) findRoot(out *sink, fd, root string, m matcher, rules ignoreRules) (unreadable int, err error) {
	ctx, stop := context.WithCancel(sc.ctx)
	defer stop()
	cmd := exec.CommandContext(ctx, fd, fdArgs(root, m, out.opts, rules)...)
	cmd.WaitDelay = toolWaitDelay
	var complaints lineCounter
	cmd.Stderr = &complaints
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	base := strings.TrimSuffix(root, "/")
	inside := base + "/"
	listed := 0
	interrupted := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxPathBytes)
	scanner.Split(splitNUL)
	for scanner.Scan() {
		listed++
		path := strings.TrimSuffix(scanner.Text(), "/")
		if !strings.HasPrefix(path, inside) {
			continue
		}
		points, tier := m.scorePath(base, path)
		if !out.offer(path, points, tier) {
			interrupted = true
			break
		}
	}
	if interrupted || scanner.Err() != nil {
		stop()
	}
	return complaints.lines, toolFailure(sc.ctx, cmd.Wait(), listed)
}

func toolFailure(ctx context.Context, waited error, produced int) error {
	var exit *exec.ExitError
	if waited == nil || ctx.Err() != nil || produced > 0 {
		return nil
	}
	if errors.As(waited, &exit) && exit.ExitCode() != toolUsageError {
		return nil
	}
	return waited
}

func fdArgs(root string, m matcher, opts Options, rules ignoreRules) []string {
	args := []string{"--color", "never", "--print0", "--full-path", "--ignore-case", "--no-require-git", "--show-errors"}
	if opts.IncludeHidden {
		args = append(args, "--hidden")
	}
	if opts.IncludeIgnored {
		args = append(args, "--no-ignore")
	}
	if !opts.CrossDevice {
		args = append(args, "--one-file-system")
	}
	for _, glob := range rules.globs(root) {
		args = append(args, "--exclude", glob)
	}
	return append(args, "--", m.fdPattern(), root)
}

func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if end := bytes.IndexByte(data, 0); end >= 0 {
		return end + 1, data[:end], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

type lineCounter struct {
	lines int
}

func (c *lineCounter) Write(p []byte) (int, error) {
	c.lines += bytes.Count(p, []byte{'\n'})
	return len(p), nil
}
