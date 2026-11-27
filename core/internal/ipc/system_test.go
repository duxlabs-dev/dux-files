package ipc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-files/core/internal/wallpaper"
	"github.com/duxlabs-dev/dux-go/files"
)

type fakeWallpaperRunner struct {
	paths map[string]bool
	calls [][]string
	runFn func(name string, args []string) (string, error)
}

func (f *fakeWallpaperRunner) LookPath(file string) (string, error) {
	if f.paths[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New("not found")
}

func (f *fakeWallpaperRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.runFn != nil {
		return f.runFn(name, args)
	}
	return "", nil
}

func TestSystemCapabilitiesReportsTheTable(t *testing.T) {
	caps := sandbox.Capabilities{Sandboxed: true, AppID: "com.example.App", Features: map[string]sandbox.Feature{
		"terminal": {Reason: sandbox.ReasonNoHostSpawn},
		"trash":    {Available: true},
	}}
	out := routeAndRead(t, Request{ID: 3, Method: "system.capabilities"}, Deps{Bus: NewEventBus(), Sandbox: caps})

	result, ok := out["result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, result["sandboxed"])
	assert.Equal(t, "com.example.App", result["appId"])
	assert.Equal(t, []any{}, result["thumbnailers"])
	assert.Equal(t, []any{}, result["searchTools"])

	features, ok := result["features"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"available": false, "reason": "NOHOSTSPAWN"}, features["terminal"])
	assert.Equal(t, map[string]any{"available": true}, features["trash"])
	assert.NotContains(t, features, "elevation", "no file operations manager, no overlay")
}

func TestSystemCapabilitiesOverlaysLiveElevation(t *testing.T) {
	bus := NewEventBus()
	caps := sandbox.Capabilities{Features: map[string]sandbox.Feature{"elevation": {Available: true}}}
	deps := Deps{Bus: bus, Sandbox: caps, FileOps: fileops.NewManager(bus, nil)}
	out := routeAndRead(t, Request{ID: 4, Method: "system.capabilities"}, deps)

	features := out["result"].(map[string]any)["features"].(map[string]any)
	assert.Equal(t, map[string]any{"available": false, "reason": "NOROUTE"}, features["elevation"])
	assert.True(t, caps.Features["elevation"].Available, "the boot-time table is left untouched")
}

func TestSystemCapabilitiesOverlaysWallpaper(t *testing.T) {
	runner := &fakeWallpaperRunner{paths: map[string]bool{"dms": true}}
	deps := Deps{
		Bus:       NewEventBus(),
		Wallpaper: wallpaper.NewWithRunner(sandbox.Capabilities{}, "dms", runner),
	}
	out := routeAndRead(t, Request{ID: 5, Method: "system.capabilities"}, deps)
	features := out["result"].(map[string]any)["features"].(map[string]any)
	assert.Equal(t, map[string]any{"available": true}, features["setWallpaper"])
}

func TestSystemSetWallpaper(t *testing.T) {
	t.Run("requires wallpaper service", func(t *testing.T) {
		out := routeAndRead(t, Request{ID: 6, Method: "system.setWallpaper", Params: map[string]any{"path": "/tmp/a.png"}}, Deps{Bus: NewEventBus()})
		assert.Equal(t, string(files.CodeNotSupported), out["code"])
	})

	t.Run("requires absolute path", func(t *testing.T) {
		deps := Deps{
			Bus:       NewEventBus(),
			Wallpaper: wallpaper.NewWithRunner(sandbox.Capabilities{}, "dms", &fakeWallpaperRunner{}),
		}
		out := routeAndRead(t, Request{ID: 7, Method: "system.setWallpaper", Params: map[string]any{"path": "a.png"}}, deps)
		assert.Equal(t, string(files.CodeInvalid), out["code"])
	})

	t.Run("invokes dms and falls back to setFor", func(t *testing.T) {
		runner := &fakeWallpaperRunner{
			runFn: func(_ string, args []string) (string, error) {
				if len(args) >= 4 && args[3] == "set" {
					return "ERROR: Per-monitor mode enabled. Use setFor(screenName, path) instead.", errors.New("exit status 1")
				}
				return "", nil
			},
		}
		deps := Deps{
			Bus:       NewEventBus(),
			Wallpaper: wallpaper.NewWithRunner(sandbox.Capabilities{}, "dms", runner),
		}
		out := routeAndRead(t, Request{
			ID:     8,
			Method: "system.setWallpaper",
			Params: map[string]any{"path": "/home/u/wall.png", "screen": "HDMI-A-1"},
		}, deps)
		assert.Equal(t, map[string]any{"set": true}, out["result"])
		require.Len(t, runner.calls, 2)
		assert.Equal(t, []string{"dms", "ipc", "call", "wallpaper", "setFor", "HDMI-A-1", "/home/u/wall.png"}, runner.calls[1])
	})
}
