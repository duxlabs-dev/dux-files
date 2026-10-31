package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dfiles [path|uri...]",
	Short: "Dux Files CLI",
	Long: `Dux Files is a standalone file manager for the dux desktop.
Without arguments it shows the file manager window. With locations it behaves like dfiles open.`,
	Args:          cobra.ArbitraryArgs,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		cmd.SilenceUsage = true
	},
	ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
	PreRunE: shellApp.ResolveConfig,
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return shellApp.CallOrLaunch("ui.show", nil)
		}
		return openLocations(args)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(_ *cobra.Command, _ []string) error {
		if jsonOutput {
			return printJSON(map[string]string{
				"version":   Version,
				"commit":    Commit,
				"buildTime": BuildTime,
			})
		}
		fmt.Printf("dfiles %s (commit %s, built %s)\n", Version, Commit, BuildTime)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(shellApp.CustomConfigVar(), "config", "c", "", "Path to a UI config dir (containing shell.qml) to use instead of the embedded UI (env: DFILES_SHELL_DIR)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output JSON for programmatic usage (where supported)")
	rootCmd.PersistentFlags().BoolVar(&dbusActivated, "dbus-activated", false, "Started by the bus for org.freedesktop.FileManager1: claim the name whatever the setting says, quit if no window ever opens")
	rootCmd.PersistentFlags().BoolVar(&keepRunning, "keep-running", false, "Started to stay resident (the systemd unit): closing the window hides it")
	rootCmd.PersistentFlags().BoolVar(&portalActivated, "portal-activated", false, "Started by the bus for the portal file chooser: quit once no dialog or window is open")
	_ = rootCmd.PersistentFlags().MarkHidden("dbus-activated")
	_ = rootCmd.PersistentFlags().MarkHidden("portal-activated")
	_ = rootCmd.PersistentFlags().MarkHidden("keep-running")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(shellApp.Commands()...)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(toggleCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(selectCmd)
	rootCmd.AddCommand(propertiesCmd)
	rootCmd.AddCommand(trashCmd)
	rootCmd.AddCommand(mimeCmd)
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(portalCmd)
	rootCmd.AddCommand(ipcCmd)
	rootCmd.AddCommand(helperCmd)
	rootCmd.AddCommand(blurCmd)
}
