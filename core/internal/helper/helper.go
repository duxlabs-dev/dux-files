package helper

import (
	"io"
	"strconv"

	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
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

// (uid_t)-1 is the kernel's keep-unchanged sentinel; -1 is the only way to say keep
const MaxID = 1<<32 - 1

// UnderPkexec runs "dfiles helper <op> ..." and exits when pkexec started this process.
func UnderPkexec() {
	dhelper.UnderPkexec("dfiles", func(args []string, stdin io.Reader, stdout io.Writer) (any, error) {
		return nil, Run(args, stdin, stdout)
	})
}

func Run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return dhelper.Invalid("missing operation")
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
			return dhelper.Invalid("symlink takes a target and a path")
		}
		return symlink(rest[0], rest[1])
	case OpRename:
		if len(rest) < 2 || len(rest) > 3 {
			return dhelper.Invalid("rename takes two paths and optionally noreplace")
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
			return dhelper.Invalid("utimes takes a path and a time in nanoseconds")
		}
		mtimeNs, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil {
			return dhelper.Invalid("not a time: " + rest[1])
		}
		return utimes(rest[0], mtimeNs)
	default:
		return dhelper.Invalid("unknown operation: " + op)
	}
}

func one(args []string, fn func(path string) error) error {
	if len(args) != 1 {
		return dhelper.Invalid("expected exactly one path")
	}
	return fn(args[0])
}

func withMode(args []string, fn func(path string, mode uint32) error) error {
	if len(args) != 2 {
		return dhelper.Invalid("expected a path and an octal mode")
	}
	mode, err := parseMode(args[1])
	if err != nil {
		return err
	}
	return fn(args[0], mode)
}

func withModeRecursive(args []string, fn func(path string, mode uint32, recursive bool) error) error {
	if len(args) < 2 || len(args) > 3 {
		return dhelper.Invalid("expected a path, an octal mode and optionally recursive")
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
		return dhelper.Invalid("expected a path, a uid, a gid and optionally recursive")
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
		return 0, dhelper.Invalid("not an octal mode: " + raw)
	}
	return uint32(mode), nil
}

func parseID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < -1 || id >= MaxID {
		return 0, dhelper.Invalid("not a uid or gid: " + raw)
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
		return false, dhelper.Invalid("unexpected argument: " + args[0])
	}
}
