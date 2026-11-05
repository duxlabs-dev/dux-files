package elevate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-files/core/internal/xdg"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/duxlabs-dev/dux-go/portal"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/helper"
)

const (
	PolicyName   = "dev.duxlabs.files.policy"
	ActionID     = "dev.duxlabs.files.file-operations"
	waitAfterEOF = 5 * time.Second
)

const (
	ReasonSandboxed = "SANDBOXED"
	ReasonNoPkexec  = "NOPKEXEC"
	ReasonNoPolicy  = "NOPOLICY"
	ReasonNoBinary  = "NOBINARY"
)

type Client struct {
	pkexec string
	exe    string
	env    []string
	once   sync.Once
	exeErr error
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Route() string { return "pkexec" }

func (c *Client) Available() error {
	if portal.InFlatpak() {
		return &fileops.Unavailable{Reason: ReasonSandboxed}
	}
	if _, err := c.pkexecPath(); err != nil {
		return &fileops.Unavailable{Reason: ReasonNoPkexec}
	}
	exe, err := c.executable()
	if err != nil {
		return &fileops.Unavailable{Reason: ReasonNoBinary}
	}
	if !policyCovers(exe, actionDirs()) {
		return &fileops.Unavailable{Reason: ReasonNoPolicy}
	}
	return nil
}

func (c *Client) pkexecPath() (string, error) {
	if c.pkexec == "" {
		return exec.LookPath("pkexec")
	}
	if _, err := os.Stat(c.pkexec); err != nil {
		return "", err
	}
	return c.pkexec, nil
}

func (c *Client) executable() (string, error) {
	c.once.Do(func() {
		if c.exe != "" {
			return
		}
		exe, err := os.Executable()
		if err == nil {
			exe, err = filepath.EvalSymlinks(exe)
		}
		c.exe, c.exeErr = exe, err
	})
	return c.exe, c.exeErr
}

func actionDirs() []string {
	dirs := []string{"/usr/share/polkit-1/actions"}
	for _, dir := range xdg.DataDirs() {
		dirs = append(dirs, filepath.Join(dir, "polkit-1", "actions"))
	}
	return dirs
}

func policyCovers(exe string, dirs []string) bool {
	for _, dir := range dirs {
		body, err := os.ReadFile(filepath.Join(dir, PolicyName))
		if err != nil {
			continue
		}
		if annotatedPath(string(body)) == exe {
			return true
		}
	}
	return false
}

func annotatedPath(policy string) string {
	const key = `key="org.freedesktop.policykit.exec.path">`
	_, after, ok := strings.Cut(policy, key)
	if !ok {
		return ""
	}
	value, _, ok := strings.Cut(after, "<")
	if !ok {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return resolved
}

func (c *Client) command(op string, args ...string) (*exec.Cmd, error) {
	pkexec, err := c.pkexecPath()
	if err != nil {
		return nil, &fileops.Unavailable{Reason: ReasonNoPkexec}
	}
	exe, err := c.executable()
	if err != nil {
		return nil, &fileops.Unavailable{Reason: ReasonNoBinary}
	}
	cmd := exec.Command(pkexec, append([]string{exe, "helper", op}, args...)...)
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	return cmd, nil
}

func (c *Client) call(op string, args ...string) ([]byte, error) {
	cmd, err := c.command(op, args...)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, failure(err, stderr.Bytes())
	}
	return stdout.Bytes(), nil
}

func failure(err error, stderr []byte) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return &files.Error{Code: files.CodeIO, Err: err}
	}

	var reported helper.Failure
	if json.Unmarshal(lastLine(stderr), &reported) == nil && reported.Code != "" {
		return &files.Error{Code: files.Code(reported.Code), Err: errors.New(reported.Error)}
	}

	switch exit.ExitCode() {
	case 126:
		return &files.Error{Code: files.CodeAccess, Err: errors.New("administrator authorization was not granted")}
	case 127:
		if bytes.Contains(stderr, []byte("Not authorized")) {
			return &files.Error{Code: files.CodeAccess, Err: errors.New("administrator authorization failed")}
		}
		return &files.Error{Code: files.CodeNotSupported, Err: errors.New("pkexec failed: " + firstLine(stderr))}
	}
	return &files.Error{Code: files.CodeIO, Err: errors.New("helper exited " + strconv.Itoa(exit.ExitCode()) + ": " + firstLine(stderr))}
}

func lastLine(out []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	return lines[len(lines)-1]
}

func firstLine(out []byte) string {
	line, _, _ := bytes.Cut(bytes.TrimSpace(out), []byte("\n"))
	return string(line)
}

func canonical(path string) string {
	dir, base := filepath.Split(path)
	resolved, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return path
	}
	return filepath.Join(resolved, base)
}

func (c *Client) Lstat(path string) (os.FileInfo, error) {
	out, err := c.call(helper.OpLstat, canonical(path))
	if err != nil {
		return nil, err
	}
	var info helper.Info
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	return fileInfo{info}, nil
}

func (c *Client) ReadDir(path string) ([]os.DirEntry, error) {
	out, err := c.call(helper.OpReadDir, canonical(path))
	if err != nil {
		return nil, err
	}
	var raw []helper.Entry
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	entries := make([]os.DirEntry, 0, len(raw))
	for _, entry := range raw {
		entries = append(entries, dirEntry{entry})
	}
	return entries, nil
}

func (c *Client) Readlink(path string) (string, error) {
	out, err := c.call(helper.OpReadlink, canonical(path))
	if err != nil {
		return "", err
	}
	var reply struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(out, &reply); err != nil {
		return "", &files.Error{Code: files.CodeIO, Err: err}
	}
	return reply.Target, nil
}

func (c *Client) Open(path string) (io.ReadCloser, error) {
	cmd, err := c.command(helper.OpRead, canonical(path))
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	if err := cmd.Start(); err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	r := &reader{cmd: cmd, pipe: stdout, buffered: bufio.NewReader(stdout), stderr: &stderr}
	if _, err := r.buffered.Peek(1); err != nil {
		if waitErr := r.wait(); waitErr != nil {
			return nil, waitErr
		}
	}
	return r, nil
}

func (c *Client) Create(path string, perm os.FileMode) (fileops.Writer, error) {
	cmd, err := c.command(helper.OpWrite, canonical(path), octal(perm))
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	if err := cmd.Start(); err != nil {
		return nil, &files.Error{Code: files.CodeIO, Err: err}
	}
	w := &writer{cmd: cmd, pipe: stdin, stderr: &stderr}
	line, readErr := bufio.NewReader(stdout).ReadString('\n')
	if readErr != nil || strings.TrimSpace(line) != helper.Ready {
		if err := w.finish(); err != nil {
			return nil, err
		}
		return nil, &files.Error{Code: files.CodeIO, Err: errors.New("helper did not confirm the file")}
	}
	return w, nil
}

func (c *Client) Mkdir(path string, perm os.FileMode) error {
	_, err := c.call(helper.OpMkdir, canonical(path), octal(perm))
	return err
}

func (c *Client) MkdirAll(path string, perm os.FileMode) error {
	_, err := c.call(helper.OpMkdirAll, canonical(path), octal(perm))
	return err
}

func (c *Client) Symlink(target, path string) error {
	_, err := c.call(helper.OpSymlink, target, canonical(path))
	return err
}

func (c *Client) Rename(from, to string, noReplace bool) error {
	_, err := c.call(helper.OpRename, append([]string{canonical(from), canonical(to)}, flag(noReplace, helper.FlagNoReplace)...)...)
	return err
}

func (c *Client) Remove(path string) error {
	_, err := c.call(helper.OpRemove, canonical(path))
	return err
}

func (c *Client) RemoveAll(path string) error {
	_, err := c.call(helper.OpRemoveAll, canonical(path))
	return err
}

func (c *Client) Chmod(path string, mode os.FileMode, recursive bool) error {
	_, err := c.call(helper.OpChmod, append([]string{canonical(path), octal(mode)}, flag(recursive, helper.FlagRecursive)...)...)
	return err
}

func (c *Client) Lchown(path string, uid, gid int, recursive bool) error {
	_, err := c.call(helper.OpChown, append([]string{canonical(path), strconv.Itoa(uid), strconv.Itoa(gid)}, flag(recursive, helper.FlagRecursive)...)...)
	return err
}

func (c *Client) Chtimes(path string, mtime time.Time) error {
	_, err := c.call(helper.OpUtimes, canonical(path), strconv.FormatInt(mtime.UnixNano(), 10))
	return err
}

func octal(mode os.FileMode) string {
	return strconv.FormatUint(uint64(mode.Perm()), 8)
}

func flag(set bool, name string) []string {
	if !set {
		return nil
	}
	return []string{name}
}

type reader struct {
	cmd      *exec.Cmd
	pipe     io.ReadCloser
	buffered *bufio.Reader
	stderr   *bytes.Buffer
	done     error
	waited   bool
}

// Wait closes the pipe, so a read error after it is EOF, not a failure
func (r *reader) Read(p []byte) (int, error) {
	n, err := r.buffered.Read(p)
	if err == nil {
		return n, nil
	}
	if waitErr := r.wait(); waitErr != nil {
		return n, waitErr
	}
	return n, io.EOF
}

func (r *reader) Close() error {
	r.pipe.Close()
	return r.wait()
}

func (r *reader) wait() error {
	if r.waited {
		return r.done
	}
	r.waited = true
	r.done = waitBounded(r.cmd, r.stderr)
	return r.done
}

// the daemon cannot signal a root helper, so a stuck one must not hang the job
func waitBounded(cmd *exec.Cmd, stderr *bytes.Buffer) error {
	result := make(chan error, 1)
	go func() { result <- cmd.Wait() }()
	ctx, cancel := context.WithTimeout(context.Background(), waitAfterEOF)
	defer cancel()
	select {
	case err := <-result:
		if err == nil {
			return nil
		}
		return failure(err, stderr.Bytes())
	case <-ctx.Done():
		return &files.Error{Code: files.CodeIO, Err: errors.New("helper did not exit")}
	}
}

type writer struct {
	cmd    *exec.Cmd
	pipe   io.WriteCloser
	stderr *bytes.Buffer
	closed bool
}

func (w *writer) Write(p []byte) (int, error) {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(p)))
	if _, err := w.pipe.Write(header[:]); err != nil {
		return 0, w.failed(err)
	}
	if _, err := w.pipe.Write(p); err != nil {
		return 0, w.failed(err)
	}
	return len(p), nil
}

func (w *writer) Close() error {
	if w.closed {
		return nil
	}
	_, err := w.pipe.Write([]byte{0, 0, 0, 0})
	if err != nil {
		return w.finish()
	}
	return w.finish()
}

func (w *writer) Abort() error {
	if w.closed {
		return nil
	}
	_ = w.finish()
	return nil
}

func (w *writer) failed(writeErr error) error {
	if err := w.finish(); err != nil {
		return err
	}
	return writeErr
}

func (w *writer) finish() error {
	w.closed = true
	w.pipe.Close()
	return waitBounded(w.cmd, w.stderr)
}

type fileInfo struct{ info helper.Info }

func (f fileInfo) Name() string       { return f.info.Name }
func (f fileInfo) Size() int64        { return f.info.Size }
func (f fileInfo) Mode() os.FileMode  { return os.FileMode(f.info.Mode) }
func (f fileInfo) ModTime() time.Time { return time.Unix(0, f.info.MtimeNs) }
func (f fileInfo) IsDir() bool        { return f.Mode().IsDir() }
func (f fileInfo) Sys() any           { return nil }

type dirEntry struct{ entry helper.Entry }

func (d dirEntry) Name() string      { return d.entry.Name }
func (d dirEntry) IsDir() bool       { return d.Type().IsDir() }
func (d dirEntry) Type() os.FileMode { return os.FileMode(d.entry.Mode) }
func (d dirEntry) Info() (os.FileInfo, error) {
	return nil, errors.New("not available through the helper")
}
