package ipc

import (
	"context"
	"os"
	"path/filepath"

	"github.com/duxlabs-dev/dux-files/core/internal/icontheme"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func HandleIconCatalog(ctx context.Context, w *ConnWriter, req Request) {
	catalog := icontheme.Default()
	w.Respond(req.ID, map[string]any{"badges": catalog.Badges, "icons": catalog.Icons})
}

// HandleIconThemes reports the icon themes the user can pick from and the one
// in use.
func HandleIconThemes(ctx context.Context, w *ConnWriter, req Request) {
	home, _ := os.UserHomeDir()
	roots := icontheme.Roots(home, os.Getenv("XDG_DATA_HOME"), filepath.SplitList(os.Getenv("XDG_DATA_DIRS")))
	w.Respond(req.ID, map[string]any{
		"setting":   icontheme.Setting(home),
		"effective": icontheme.Effective(home),
		"installed": icontheme.Installed(roots),
		"paths":     icontheme.Paths(roots, icontheme.Effective(home)),
	})
}

// HandleSetIconTheme stores the icon theme the user picked.
func HandleSetIconTheme(ctx context.Context, w *ConnWriter, req Request, deps Deps) {
	home, _ := os.UserHomeDir()
	if err := icontheme.Set(home, params.StringOpt(req.Params, "name", "")); err != nil {
		w.RespondError(req.ID, err.Error())
		return
	}
	if deps.Bus != nil {
		deps.Bus.Publish("iconTheme", map[string]any{"setting": icontheme.Setting(home), "effective": icontheme.Effective(home)})
	}
	HandleIconThemes(ctx, w, req)
}
