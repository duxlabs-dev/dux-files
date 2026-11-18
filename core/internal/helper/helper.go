package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"os"
	"strconv"
	"strings"

	"github.com/duxlabs-dev/dux-go/files"
	"golang.org/x/sys/unix"
)

const (
	OpLstat     = "lstat"
	OpReadDir   = "readdir"
	OpReadlink  = "readlink"
	OpRead      = "read"
	OpWrite     = "write"
	OpMkdir     = "mkdir"
	OpMkdirAll  = "mkdirall"
	OpSymlink   = "symlink"
	OpRename    = "rename"
	OpRemove    = "remove"
	OpRemoveAll = "removeall"
	OpChmod     = "chmod"
	OpChown     = "chown"
	OpUtimes    = "utimes"
)

const (
	FlagRecursive = "recursive"
	FlagNoReplace = "noreplace"
	Ready         = "ready"
)

type Info struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
	MtimeNs int64  `json:"mtimeNs"`
	UID     uint32 `json:"uid"`
	GID     uint32 `json:"gid"`
}

type Entry struct {
	Name string `json:"name"`
	Mode uint32 `json:"mode"`
}

type Failure struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

const (
	ExitFailed  = 1
	ExitRefused = 2
)

// (uid_t)-1 is the kernel's keep-unchanged sentinel; -1 is the only way to say keep
const MaxID = 1<<32 - 1

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	callerUID, err := strconv.Atoi(os.Getenv("PKEXEC_UID"))
	if err != nil {
		report(stderr, invalid("dfiles helper only runs under pkexec"))
		return ExitRefused
	}
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		report(stderr, invalid("dfiles helper must be started as root"))
		return ExitRefused
	}
	if err := confine(); err != nil {
		report(stderr, err)
		return ExitRefused
	}
	unix.Umask(0)

	err = Run(args, stdin, stdout)
	journal(callerUID, args, err)
	if err != nil {
		report(stderr, err)
		return ExitFailed
	}
	return 0
}

func Run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return invalid("missing operation")
	}
	op, rest := args[0], args[1:]

	switch op {
	case OpLstat:
		return one(rest, func(path string) error { return lstat(path, stdout) })
	case OpReadDir:
		return one(rest, func(path string) error { return readDir(path, stdout) })
	case OpReadlink:
		return one(rest, func(path string) error { return readlink(path, stdout) })
	case OpRead:
		return one(rest, func(path string) error { return read(path, stdout) })
	case OpWrite:
		return withMode(rest, func(path string, mode uint32) error { return write(path, mode, stdin, stdout) })
	case OpMkdir:
		return withMode(rest, mkdir)
	case OpMkdirAll:
		return withMode(rest, mkdirAll)
	case OpSymlink:
		if len(rest) != 2 {
			return invalid("symlink takes a target and a path")
		}
		return symlink(rest[0], rest[1])
	case OpRename:
		if len(rest) < 2 || len(rest) > 3 {
			return invalid("rename takes two paths and optionally noreplace")
		}
		noReplace, err := parseFlag(rest[2:], FlagNoReplace)
		if err != nil {
			return err
		}
		return rename(rest[0], rest[1], noReplace)
	case OpRemove:
		return one(rest, remove)
	case OpRemoveAll:
		return one(rest, removeAll)
	case OpChmod:
		return withModeRecursive(rest, chmod)
	case OpChown:
		return chownArgs(rest)
	case OpUtimes:
		if len(rest) != 2 {
			return invalid("utimes takes a path and a time in nanoseconds")
		}
		mtimeNs, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil {
			return invalid("not a time: " + rest[1])
		}
		return utimes(rest[0], mtimeNs)
	default:
		return invalid("unknown operation: " + op)
	}
}

func one(args []string, fn func(path string) error) error {
	if len(args) != 1 {
		return invalid("expected exactly one path")
	}
	return fn(args[0])
}

func withMode(args []string, fn func(path string, mode uint32) error) error {
	if len(args) != 2 {
		return invalid("expected a path and an octal mode")
	}
	mode, err := parseMode(args[1])
	if err != nil {
		return err
	}
	return fn(args[0], mode)
}

func withModeRecursive(args []string, fn func(path string, mode uint32, recursive bool) error) error {
	if len(args) < 2 || len(args) > 3 {
		return invalid("expected a path, an octal mode and optionally recursive")
	}
	mode, err := parseMode(args[1])
	if err != nil {
		return err
	}
	recursive, err := parseFlag(args[2:], FlagRecursive)
	if err != nil {
		return err
	}
	return fn(args[0], mode, recursive)
}

func chownArgs(args []string) error {
	if len(args) < 3 || len(args) > 4 {
		return invalid("expected a path, a uid, a gid and optionally recursive")
	}
	uid, err := parseID(args[1])
	if err != nil {
		return err
	}
	gid, err := parseID(args[2])
	if err != nil {
		return err
	}
	recursive, err := parseFlag(args[3:], FlagRecursive)
	if err != nil {
		return err
	}
	return chown(args[0], uid, gid, recursive)
}

func parseMode(raw string) (uint32, error) {
	mode, err := strconv.ParseUint(raw, 8, 32)
	if err != nil || mode&^0o777 != 0 {
		return 0, invalid("not an octal mode: " + raw)
	}
	return uint32(mode), nil
}

func parseID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < -1 || id >= MaxID {
		return 0, invalid("not a uid or gid: " + raw)
	}
	return id, nil
}

func parseFlag(args []string, name string) (bool, error) {
	switch {
	case len(args) == 0:
		return false, nil
	case args[0] == name:
		return true, nil
	default:
		return false, invalid("unexpected argument: " + args[0])
	}
}

func invalid(msg string) error {
	return &files.Error{Code: files.CodeInvalid, Err: errors.New(msg)}
}

func report(stderr io.Writer, err error) {
	line, _ := json.Marshal(Failure{Code: string(CodeOf(err)), Error: err.Error()})
	fmt.Fprintln(stderr, string(line))
}

func CodeOf(err error) files.Code {
	var coded *files.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return files.CodeFromOS(err)
}

func journal(uid int, args []string, err error) {
	writer, openErr := syslog.New(syslog.LOG_AUTHPRIV|syslog.LOG_INFO, "dfiles-helper")
	if openErr != nil {
		return
	}
	defer writer.Close()
	outcome := "ok"
	if err != nil {
		outcome = "failed: " + err.Error()
	}
	_ = writer.Info(fmt.Sprintf("uid=%d %s %s", uid, strings.Join(args, " "), outcome))
}
