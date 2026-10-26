package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/duxlabs-dev/dux-files/core/internal/portal"
)

var (
	portalRestart   bool
	portalNoRestart bool
)

var portalCmd = &cobra.Command{
	Use:   "portal",
	Short: "Serve other apps' open and save dialogs through the desktop portal",
	Long: `Serve other apps' open and save dialogs through the desktop portal.

install  writes the per-user .portal file and D-Bus activation file, and makes dfiles
         the preferred org.freedesktop.impl.portal.FileChooser backend in your
         <desktop>-portals.conf, keeping its other keys
uninstall undoes only what install changed
status   shows each part and whether xdg-desktop-portal still needs a restart

The change takes effect after xdg-desktop-portal restarts. Restarting it cancels
file dialogs other apps have open.`,
}

var portalStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether dfiles serves the portal file chooser",
	RunE: func(_ *cobra.Command, _ []string) error {
		return withPortal("system.portal.get", func(c *portal.Controller) (portal.Report, error) { return c.Status(), nil }, false)
	},
}

var portalInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Make dfiles the portal file chooser for this user",
	RunE: func(_ *cobra.Command, _ []string) error {
		return withPortal("system.portal.install", (*portal.Controller).Install, true)
	},
}

var portalUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Restore the previous portal file chooser",
	RunE: func(_ *cobra.Command, _ []string) error {
		return withPortal("system.portal.uninstall", (*portal.Controller).Uninstall, true)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{portalInstallCmd, portalUninstallCmd} {
		cmd.Flags().BoolVar(&portalRestart, "restart", false, "Restart xdg-desktop-portal afterwards without asking")
		cmd.Flags().BoolVar(&portalNoRestart, "no-restart", false, "Never restart xdg-desktop-portal")
	}
	portalCmd.AddCommand(portalStatusCmd, portalInstallCmd, portalUninstallCmd)
}

func withPortal(method string, local func(*portal.Controller) (portal.Report, error), changes bool) error {
	report, restart, err := runPortal(method, local)
	if err != nil {
		return err
	}
	if changes && report.Frontend.RestartNeeded && wantRestart() {
		if err := restart(); err != nil {
			return err
		}
		report.Frontend.RestartNeeded = false
	}
	if jsonOutput {
		return printJSON(report)
	}
	printPortalReport(report)
	return nil
}

func runPortal(method string, local func(*portal.Controller) (portal.Report, error)) (portal.Report, func() error, error) {
	if s, err := dialDaemon(); err == nil {
		defer s.Close()
		var report portal.Report
		if err := s.call(method, nil, &report); err != nil {
			return report, nil, err
		}
		return report, func() error { return s.call("system.portal.restart", nil, nil) }, nil
	}
	controller := portal.NewController(nil, false)
	report, err := local(controller)
	return report, controller.Restart, err
}

func wantRestart() bool {
	switch {
	case portalNoRestart:
		return false
	case portalRestart:
		return true
	case !stdinIsTerminal():
		return false
	}
	fmt.Print("Restart xdg-desktop-portal now? Open and save dialogs other apps have open will be cancelled. [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func printPortalReport(r portal.Report) {
	switch {
	case !r.Available && r.Reason != "":
		fmt.Printf("backend:    unavailable (%s)\n", r.Reason)
	case r.Backend.Owned:
		fmt.Println("backend:    owned by this instance")
	case r.Service.Found:
		fmt.Printf("backend:    activatable through %s\n", r.Service.Path)
	default:
		fmt.Println("backend:    not reachable (no activation file)")
	}
	switch {
	case r.PortalFile.Found:
		fmt.Printf("portal:     %s\n", r.PortalFile.Path)
	default:
		fmt.Println("portal:     dfiles.portal not installed")
	}
	switch {
	case r.SystemManaged:
		fmt.Println("preference: managed by the system (XDG_DESKTOP_PORTAL_DIR); add dfiles there")
	case r.Config.Effective == "dfiles":
		fmt.Printf("preference: %s prefers dfiles\n", r.Config.Path)
	case r.Config.Effective != "":
		fmt.Printf("preference: %s is used (%s)\n", r.Config.Effective, r.Config.Path)
	default:
		fmt.Printf("preference: no file chooser backend configured (%s)\n", r.Config.Path)
	}
	switch {
	case !r.Frontend.Running:
		fmt.Println("service:    xdg-desktop-portal is not running")
	case r.Frontend.RestartNeeded && r.Frontend.UnitKnown:
		fmt.Println("service:    restart needed: systemctl --user restart xdg-desktop-portal.service")
	case r.Frontend.RestartNeeded:
		fmt.Printf("service:    restart needed: kill %d, the bus starts xdg-desktop-portal again on demand\n", r.Frontend.PID)
	default:
		fmt.Println("service:    xdg-desktop-portal is up to date")
	}
	if r.Kept {
		fmt.Printf("note:       %s was edited by hand since install and was left alone\n", r.Config.Path)
	}
}
