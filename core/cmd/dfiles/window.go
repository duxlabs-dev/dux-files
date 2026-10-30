package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-files/core/internal/fileops"
	"github.com/duxlabs-dev/dux-files/core/internal/location"
	"github.com/duxlabs-dev/dux-files/core/internal/sandbox"
	"github.com/duxlabs-dev/dux-go/paths"
)

var showCmd = &cobra.Command{
	Use:     "show",
	Short:   "Show the file manager window, launching dfiles if it is not running",
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, _ []string) error {
		return shellApp.CallOrLaunch("ui.show", nil)
	},
}

var toggleCmd = &cobra.Command{
	Use:     "toggle",
	Short:   "Toggle file manager window visibility, launching dfiles if it is not running",
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, _ []string) error {
		return shellApp.CallOrLaunch("ui.toggle", nil)
	},
}

var openNewWindow bool

var openCmd = &cobra.Command{
	Use:   "open [path|uri...]",
	Short: "Open folders, one tab each; a file opens its folder with the file selected",
	Long: `Open folders, one tab each; a file opens its folder with the file selected.
With no argument a new window opens at the home folder, like launching the app.
--new-window opens the given locations in a new window instead of the last used one.

Accepted locations:
  /path, relative/path     a local path
  file:///path             a local path, percent-encoded
  trash:///                the home trash's files folder
  recent:///               recently used files
  computer:///             the root filesystem
  network:///              the Connect to Server dialog
  admin:///path            the path; denied operations offer Retry as administrator
  sftp:// ssh:// smb:// ftp:// ftps:// dav:// davs:// nfs:// afp://
                           mounted through gvfs, then opened in a new tab`,
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return callWindow("ui.open", map[string]any{"newWindow": true}, func(location.Env) error { return nil })
		}
		return openLocations(args)
	},
}

func openLocations(args []string) error {
	targets, err := locationArgs(args)
	if err != nil {
		return err
	}
	params := map[string]any{"paths": targets}
	if openNewWindow {
		params["newWindow"] = true
	}
	return callWindow("ui.open", params, func(env location.Env) error {
		_, err := location.Resolve(targets, nil, env)
		return err
	})
}

func init() {
	for _, cmd := range []*cobra.Command{rootCmd, openCmd} {
		cmd.Flags().BoolVarP(&openNewWindow, "new-window", "n", false, "Open a new window instead of using the last used one")
	}
}

var selectCmd = &cobra.Command{
	Use:     "select <path...>",
	Short:   "Open the folders holding paths with those paths selected",
	Args:    cobra.MinimumNArgs(1),
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, args []string) error {
		targets, err := locationArgs(args)
		if err != nil {
			return err
		}
		return callWindow("ui.open", map[string]any{"select": targets}, func(env location.Env) error {
			_, err := location.Resolve(nil, targets, env)
			return err
		})
	},
}

var propertiesCmd = &cobra.Command{
	Use:     "properties <path...>",
	Short:   "Show the properties dialog for paths",
	Args:    cobra.MinimumNArgs(1),
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, args []string) error {
		targets, err := locationArgs(args)
		if err != nil {
			return err
		}
		return callWindow("ui.properties", map[string]any{"paths": targets}, func(env location.Env) error {
			_, err := location.LocalPaths(targets, env)
			return err
		})
	},
}

func locationArgs(args []string) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, location.Absolute(arg, cwd))
	}
	return out, nil
}

// A cold start opens the new window itself, so --new-window is dropped before launching.
func callWindow(method string, params map[string]any, validate func(location.Env) error) error {
	if s, err := dialDaemon(); err == nil {
		defer s.Close()
		return s.call(method, params, nil)
	}
	sandbox.ExportHostDirs(sandbox.Current())
	if err := validate(location.HostEnv(paths.XDGDataHome(), fileops.IsArchive)); err != nil {
		return err
	}
	delete(params, "newWindow")
	return shellApp.CallOrLaunch(method, params)
}
