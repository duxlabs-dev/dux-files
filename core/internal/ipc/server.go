package ipc

import (
	"context"

	dankipc "github.com/duxlabs-dev/dux-go/ipc"
)

type Server = dankipc.Server

func NewServer(deps Deps) *Server {
	if deps.Bus == nil {
		deps.Bus = NewEventBus()
	}

	cfg := dankipc.Config{
		AppName:                "dfiles",
		APIVersion:             APIVersion,
		Capabilities:           []string{"subscribe", "ui", "system", "files", "files.op", "devices", "network", "clipboard", "search", "bookmarks", "games", "trash"},
		DefaultSubscribeTopics: []string{"ui"},
		Bus:                    deps.Bus,
		OnSubscribe: func(topics []string, _ *Subscriber) {
			publishPending(deps, topics)
			if deps.Files != nil {
				deps.Files.Attach(topics)
			}
			if deps.Search != nil {
				deps.Search.Attach(topics)
			}
		},
	}

	return dankipc.NewServer(cfg, func(ctx context.Context, w *ConnWriter, req Request, _ *Subscriber) {
		Route(ctx, w, req, deps)
	})
}

func publishPending(deps Deps, topics []string) {
	if deps.Pending == nil {
		republishChoosers(deps)
		return
	}
	for _, t := range topics {
		if t != "ui" {
			continue
		}
		deps.Pending.Flush(deps.Bus)
		republishChoosers(deps)
		return
	}
}
