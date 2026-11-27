package elevate

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/duxlabs-dev/dux-go/elevate"
	"github.com/duxlabs-dev/dux-go/files"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/helper"
)

const PolicyName = "dev.duxlabs.files.policy"

type Client struct {
	helper *elevate.Client
}

func NewClient() *Client {
	return &Client{helper: &elevate.Client{Policy: PolicyName}}
}

func (c *Client) Route() string { return "pkexec" }

func (c *Client) Available() error {
	if reason := c.helper.Available(); reason != "" {
		return &fileops.Unavailable{Reason: reason}
	}
	return nil
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
	out, err := c.helper.Call(helper.OpLstat, canonical(path))
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
	out, err := c.helper.Call(helper.OpReadDir, canonical(path))
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
	out, err := c.helper.Call(helper.OpReadlink, canonical(path))
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
	return c.helper.Reader(helper.OpRead, canonical(path))
}

func (c *Client) Create(path string, perm os.FileMode) (fileops.Writer, error) {
	return c.helper.Writer(helper.OpWrite, canonical(path), octal(perm))
}

func (c *Client) Mkdir(path string, perm os.FileMode) error {
	_, err := c.helper.Call(helper.OpMkdir, canonical(path), octal(perm))
	return err
}

func (c *Client) MkdirAll(path string, perm os.FileMode) error {
	_, err := c.helper.Call(helper.OpMkdirAll, canonical(path), octal(perm))
	return err
}

func (c *Client) Symlink(target, path string) error {
	_, err := c.helper.Call(helper.OpSymlink, target, canonical(path))
	return err
}

func (c *Client) Rename(from, to string, noReplace bool) error {
	_, err := c.helper.Call(helper.OpRename, append([]string{canonical(from), canonical(to)}, flag(noReplace, helper.FlagNoReplace)...)...)
	return err
}

func (c *Client) Remove(path string) error {
	_, err := c.helper.Call(helper.OpRemove, canonical(path))
	return err
}

func (c *Client) RemoveAll(path string) error {
	_, err := c.helper.Call(helper.OpRemoveAll, canonical(path))
	return err
}

func (c *Client) Chmod(path string, mode os.FileMode, recursive bool) error {
	_, err := c.helper.Call(helper.OpChmod, append([]string{canonical(path), octal(mode)}, flag(recursive, helper.FlagRecursive)...)...)
	return err
}

func (c *Client) Lchown(path string, uid, gid int, recursive bool) error {
	_, err := c.helper.Call(helper.OpChown, append([]string{canonical(path), strconv.Itoa(uid), strconv.Itoa(gid)}, flag(recursive, helper.FlagRecursive)...)...)
	return err
}

func (c *Client) Chtimes(path string, mtime time.Time) error {
	_, err := c.helper.Call(helper.OpUtimes, canonical(path), strconv.FormatInt(mtime.UnixNano(), 10))
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
