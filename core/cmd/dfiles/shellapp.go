package main

import (
	"context"
	"os"

	"github.com/duxlabs-dev/dux-files/core/internal/icontheme"
	"github.com/duxlabs-dev/dux-files/core/internal/shellembed"
	"github.com/duxlabs-dev/dux-go/shellapp"
)

var shellApp = shellapp.New(shellapp.Config{
	ID:       "dfiles",
	QSAppID:  "dev.duxlabs.files",
	Version:  Version,
	Embedded: embeddedShell{},
	Boot:     bootBackend,
	ExtraEnv: uiEnv,
})

var (
	dbusActivated   bool
	portalActivated bool
	keepRunning     bool
)

func uiEnv(string) []string {
	var env []string
	if dbusActivated || portalActivated {
		env = append(env, "DFILES_DBUS_ACTIVATED=1")
	}
	if keepRunning {
		env = append(env, "DFILES_KEEP_RUNNING=1")
	}
	if os.Getenv("QS_ICON_THEME") == "" {
		home, _ := os.UserHomeDir()
		if theme := icontheme.Effective(home); theme != "" {
			env = append(env, "QS_ICON_THEME="+theme)
		}
	}
	return env
}

func bootBackend(ctx context.Context) (shellapp.Backend, error) {
	svc, err := bootDaemonServices(ctx, true)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

type embeddedShell struct{}

func (embeddedShell) Available() bool { return shellembed.Available() }

func (embeddedShell) Extract(baseDir string) (string, error) { return shellembed.Extract(baseDir) }

func (embeddedShell) Prune(baseDir, keep string) { shellembed.Prune(baseDir, keep) }
