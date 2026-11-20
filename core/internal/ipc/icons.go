package ipc

import (
	"context"

	"github.com/duxlabs-dev/dux-go/ipc/params"

	"github.com/duxlabs-dev/dux-files/core/internal/icontheme"
)

func HandleIconCatalog(ctx context.Context, w *ConnWriter, req Request) {
	catalog := icontheme.Default(params.StringOpt(req.Params, "theme", ""))
	w.Respond(req.ID, map[string]any{"badges": catalog.Badges, "icons": catalog.Icons})
}
