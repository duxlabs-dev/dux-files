package search

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const versionTimeout = 2 * time.Second

type toolSpec struct {
	names   []string
	banner  string
	minimum [3]int
}

var (
	fdSpec = toolSpec{names: []string{"fd", "fdfind"}, banner: "fd", minimum: [3]int{8, 7, 0}}
	rgSpec = toolSpec{names: []string{"rg"}, banner: "ripgrep", minimum: [3]int{12, 0, 0}}
)

type tools struct {
	fd string
	rg string
}

type toolbox struct {
	lookPath func(string) (string, error)
	banner   func(path string) string
	now      func() time.Time
	logged   atomic.Bool

	mu      sync.Mutex
	checked time.Time
	found   tools
}

func newToolbox() toolbox {
	return toolbox{lookPath: exec.LookPath, banner: versionBanner, now: time.Now}
}

func (b *toolbox) locate() tools {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if !b.checked.IsZero() && now.Sub(b.checked) < locateTTL {
		return b.found
	}
	b.checked = now
	b.found = tools{fd: b.resolve(fdSpec, b.found.fd), rg: b.resolve(rgSpec, b.found.rg)}
	return b.found
}

func (b *toolbox) resolve(spec toolSpec, known string) string {
	for _, name := range spec.names {
		path, err := b.lookPath(name)
		if err != nil {
			continue
		}
		if path == known || spec.accepts(b.banner(path)) {
			return path
		}
	}
	return ""
}

func (s toolSpec) accepts(banner string) bool {
	fields := strings.Fields(banner)
	if len(fields) < 2 || fields[0] != s.banner {
		return false
	}
	version, ok := parseVersion(fields[1])
	if !ok {
		return false
	}
	for i, least := range s.minimum {
		if version[i] != least {
			return version[i] > least
		}
	}
	return true
}

func parseVersion(text string) ([3]int, bool) {
	var version [3]int
	parts := strings.SplitN(text, ".", 3)
	if len(parts) < 2 {
		return version, false
	}
	for i, part := range parts {
		digits := part
		if cut := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }); cut >= 0 {
			digits = part[:cut]
		}
		number, err := strconv.Atoi(digits)
		if err != nil {
			return version, false
		}
		version[i] = number
	}
	return version, true
}

func versionBanner(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return line
}
