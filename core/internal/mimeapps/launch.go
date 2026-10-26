package mimeapps

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/duxlabs-dev/dux-go/files"
)

type LaunchOptions struct {
	Paths           []string
	WorkingDir      string
	ActivationToken string
	Terminal        string
	Opener          string
}

func (s *Service) Launch(entry *Entry, opts LaunchOptions) error {
	if !entry.Launchable() {
		return fmt.Errorf("desktop entry is not launchable")
	}

	tokens, err := splitExec(entry.Exec)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return errors.New("empty Exec")
	}

	batches := [][]string{opts.Paths}
	if singularExec(tokens) && len(opts.Paths) > 1 {
		batches = batches[:0]
		for _, path := range opts.Paths {
			batches = append(batches, []string{path})
		}
	}

	for _, batch := range batches {
		argv := expandExec(tokens, entry, batch)
		if entry.Terminal {
			argv, err = s.wrapTerminal(argv, opts.Terminal)
			if err != nil {
				return err
			}
		}
		if err := spawn(argv, workingDir(entry, opts), opts.ActivationToken, entry.StartupNotify); err != nil {
			return err
		}
	}
	return nil
}

func workingDir(entry *Entry, opts LaunchOptions) string {
	if entry.WorkingDir != "" {
		return entry.WorkingDir
	}
	return opts.WorkingDir
}

func (s *Service) OpenTerminal(dir, preferred string) error {
	if Sandboxed() {
		term := preferred
		if term == "" {
			term = os.Getenv("TERMINAL")
		}
		if term == "" {
			return fail(files.CodeSandboxed, "set a terminal in settings: the sandbox cannot see the host's")
		}
		return hostSpawn(specFor(term).argv(dir), dir)
	}

	term, err := s.terminal(preferred)
	if err != nil {
		return fail(files.CodeNotSupported, "%s", err)
	}
	return spawn(term.argv(dir), dir, "", false)
}

func singularExec(tokens []string) bool {
	for _, tok := range tokens {
		switch {
		case strings.Contains(tok, "%F"), strings.Contains(tok, "%U"):
			return false
		}
	}
	return true
}

func expandExec(tokens []string, entry *Entry, paths []string) []string {
	argv := make([]string, 0, len(tokens)+len(paths))
	placed := false

	for _, tok := range tokens {
		switch tok {
		case "%f", "%u":
			if len(paths) > 0 {
				argv = append(argv, fieldValue(tok, paths[0]))
				placed = true
			}
			continue
		case "%F":
			argv = append(argv, paths...)
			placed = true
			continue
		case "%U":
			for _, path := range paths {
				argv = append(argv, pathURI(path))
			}
			placed = true
			continue
		case "%i":
			if entry.Icon != "" {
				argv = append(argv, "--icon", entry.Icon)
			}
			continue
		case "%c":
			argv = append(argv, entry.Name)
			continue
		case "%k":
			argv = append(argv, entry.Path)
			continue
		case "%d", "%D", "%n", "%N", "%v", "%m":
			continue
		}

		expanded, used := expandInline(tok, entry, paths)
		placed = placed || used
		argv = append(argv, expanded)
	}

	if !placed {
		argv = append(argv, paths...)
	}
	return argv
}

func fieldValue(code, path string) string {
	if code == "%u" {
		return pathURI(path)
	}
	return path
}

func expandInline(tok string, entry *Entry, paths []string) (string, bool) {
	if !strings.ContainsRune(tok, '%') {
		return tok, false
	}

	var b strings.Builder
	used := false
	for i := 0; i < len(tok); i++ {
		if tok[i] != '%' || i+1 >= len(tok) {
			b.WriteByte(tok[i])
			continue
		}
		i++
		switch tok[i] {
		case 'f', 'F':
			if len(paths) > 0 {
				b.WriteString(paths[0])
				used = true
			}
		case 'u', 'U':
			if len(paths) > 0 {
				b.WriteString(pathURI(paths[0]))
				used = true
			}
		case 'c':
			b.WriteString(entry.Name)
		case 'k':
			b.WriteString(entry.Path)
		case '%':
			b.WriteByte('%')
		}
	}
	return b.String(), used
}

func PathURI(path string) string { return pathURI(path) }

func pathURI(path string) string {
	if !strings.HasPrefix(path, "/") {
		return path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func spawn(argv []string, dir, activationToken string, startupNotify bool) error {
	if len(argv) == 0 {
		return errors.New("nothing to run")
	}

	argv = inOwnScope(argv, os.Getenv, exec.LookPath)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = childEnv(activationToken, startupNotify)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// Under a systemd unit, launched apps would otherwise stay in the unit's cgroup.
func inOwnScope(argv []string, getenv func(string) string, lookPath func(string) (string, error)) []string {
	if getenv("INVOCATION_ID") == "" {
		return argv
	}
	runner, err := lookPath("systemd-run")
	if err != nil {
		return argv
	}
	if _, err := lookPath(argv[0]); err != nil {
		return argv
	}
	return append([]string{runner, "--user", "--scope", "--quiet", "--collect", "--slice=app.slice", "--"}, argv...)
}

func childEnv(activationToken string, startupNotify bool) []string {
	env := os.Environ()
	if activationToken == "" || !startupNotify {
		return env
	}
	return append(env, "XDG_ACTIVATION_TOKEN="+activationToken, "DESKTOP_STARTUP_ID="+activationToken)
}

func splitExec(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inQuote, escaped, started := false, false, false

	flush := func() {
		if started || cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
			started = false
		}
	}

	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case inQuote && r == '\\':
			escaped = true
		case r == '"':
			inQuote = !inQuote
			started = true
		case !inQuote && (r == ' ' || r == '\t'):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote || escaped {
		return nil, errors.New("unterminated quoting in Exec")
	}
	flush()
	return args, nil
}
