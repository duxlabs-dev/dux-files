package wallpaper

import (
	"context"
	"errors"
	"testing"

	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-go/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runCall struct {
	name string
	args []string
}

type fakeRunner struct {
	paths map[string]bool
	runFn func(name string, args []string) (string, error)
	calls []runCall
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if f.paths[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, runCall{name: name, args: args})
	if f.runFn != nil {
		return f.runFn(name, args)
	}
	return "", nil
}

func TestFeatureAvailability(t *testing.T) {
	ctx := context.Background()

	t.Run("native with dms", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", &fakeRunner{paths: map[string]bool{"dms": true}})
		assert.Equal(t, sandbox.Feature{Available: true}, svc.Feature(ctx))
	})

	t.Run("native without dms", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", &fakeRunner{})
		assert.Equal(t, sandbox.Feature{Reason: ReasonNoDMS}, svc.Feature(ctx))
	})

	t.Run("sandboxed without hostSpawn", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: false}, "dms", &fakeRunner{})
		assert.Equal(t, sandbox.Feature{Reason: sandbox.ReasonNoHostSpawn}, svc.Feature(ctx))
	})

	t.Run("sandboxed with hostSpawn and dms on host", func(t *testing.T) {
		r := &fakeRunner{paths: map[string]bool{"flatpak-spawn": true}}
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: true}, "/custom/dms", r)
		assert.Equal(t, sandbox.Feature{Available: true}, svc.Feature(ctx))
		require.Len(t, r.calls, 1)
		assert.Equal(t, "flatpak-spawn", r.calls[0].name)
		assert.Equal(t, []string{"--host", "sh", "-c", `command -v "$1" >/dev/null 2>&1`, "_", "/custom/dms"}, r.calls[0].args)
	})

	t.Run("sandboxed with hostSpawn and dms missing on host", func(t *testing.T) {
		r := &fakeRunner{
			paths: map[string]bool{"flatpak-spawn": true},
			runFn: func(string, []string) (string, error) { return "", errors.New("exit status 1") },
		}
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: true}, "dms", r)
		assert.Equal(t, sandbox.Feature{Reason: ReasonNoDMS}, svc.Feature(ctx))
	})

	t.Run("sandboxed with hostSpawn but flatpak-spawn missing", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: true}, "dms", &fakeRunner{})
		assert.Equal(t, sandbox.Feature{Reason: sandbox.ReasonNoHostSpawn}, svc.Feature(ctx))
	})
}

func TestSetWallpaper(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects empty or relative path", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", &fakeRunner{})
		err := svc.Set(ctx, "", "")
		assert.Equal(t, files.CodeInvalid, files.CodeOf(err))

		err = svc.Set(ctx, "relative/wall.png", "")
		assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
	})

	t.Run("native set succeeds", func(t *testing.T) {
		r := &fakeRunner{}
		svc := NewWithRunner(sandbox.Capabilities{}, "/opt/dms", r)
		require.NoError(t, svc.Set(ctx, "/home/u/wall.png", "DP-1"))
		require.Len(t, r.calls, 1)
		assert.Equal(t, runCall{name: "/opt/dms", args: []string{"ipc", "call", "wallpaper", "set", "/home/u/wall.png"}}, r.calls[0])
	})

	t.Run("falls back to setFor on per-monitor mode", func(t *testing.T) {
		r := &fakeRunner{
			runFn: func(_ string, args []string) (string, error) {
				if len(args) >= 4 && args[3] == "set" {
					return "ERROR: Per-monitor mode enabled. Use setFor(screenName, path) instead.\n", errors.New("exit status 1")
				}
				return "", nil
			},
		}
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", r)
		require.NoError(t, svc.Set(ctx, "/home/u/wall.png", "DP-1"))
		require.Len(t, r.calls, 2)
		assert.Equal(t, []string{"ipc", "call", "wallpaper", "set", "/home/u/wall.png"}, r.calls[0].args)
		assert.Equal(t, []string{"ipc", "call", "wallpaper", "setFor", "DP-1", "/home/u/wall.png"}, r.calls[1].args)
	})

	t.Run("per-monitor mode without screen returns error", func(t *testing.T) {
		r := &fakeRunner{
			runFn: func(string, []string) (string, error) {
				return "ERROR: Per-monitor mode enabled. Use setFor(screenName, path) instead.", nil
			},
		}
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", r)
		err := svc.Set(ctx, "/home/u/wall.png", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Per-monitor mode enabled")
		assert.Len(t, r.calls, 1)
	})

	t.Run("sandboxed uses flatpak-spawn --host", func(t *testing.T) {
		r := &fakeRunner{paths: map[string]bool{"flatpak-spawn": true}}
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: true}, "dms", r)
		require.NoError(t, svc.Set(ctx, "/home/u/wall.png", ""))
		require.Len(t, r.calls, 1)
		assert.Equal(t, runCall{name: "flatpak-spawn", args: []string{"--host", "dms", "ipc", "call", "wallpaper", "set", "/home/u/wall.png"}}, r.calls[0])
	})

	t.Run("sandboxed without hostSpawn fails", func(t *testing.T) {
		svc := NewWithRunner(sandbox.Capabilities{Sandboxed: true, HostSpawn: false}, "dms", &fakeRunner{})
		err := svc.Set(ctx, "/home/u/wall.png", "")
		assert.Equal(t, files.CodeSandboxed, files.CodeOf(err))
	})

	t.Run("exit 0 with ERROR prefix returns IO error", func(t *testing.T) {
		r := &fakeRunner{
			runFn: func(string, []string) (string, error) {
				return "ERROR: No path provided\n", nil
			},
		}
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", r)
		err := svc.Set(ctx, "/home/u/wall.png", "DP-1")
		require.Error(t, err)
		assert.Equal(t, files.CodeIO, files.CodeOf(err))
		assert.Contains(t, err.Error(), "No path provided")
		assert.Len(t, r.calls, 1)
	})

	t.Run("setFor fallback failure returns fallback error", func(t *testing.T) {
		r := &fakeRunner{
			runFn: func(_ string, args []string) (string, error) {
				if len(args) >= 4 && args[3] == "set" {
					return "ERROR: Per-monitor mode enabled.", errors.New("exit status 1")
				}
				return "ERROR: Screen not found", errors.New("exit status 1")
			},
		}
		svc := NewWithRunner(sandbox.Capabilities{}, "dms", r)
		err := svc.Set(ctx, "/home/u/wall.png", "DP-99")
		require.Error(t, err)
		assert.Equal(t, files.CodeIO, files.CodeOf(err))
		assert.Contains(t, err.Error(), "Screen not found")
		assert.Len(t, r.calls, 2)
	})
}
