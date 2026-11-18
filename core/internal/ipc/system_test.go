package ipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
)

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
