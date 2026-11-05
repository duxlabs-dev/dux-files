package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/duxlabs-dev/dux-files/core/internal/devices"

	"github.com/spf13/cobra"
)

type deviceList struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason"`
	Devices   []devices.Device `json:"devices"`
}

var (
	devicesAllFlag   bool
	devicesForceFlag bool
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List, mount, unmount and eject drives through udisks2",
}

var devicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List drives and volumes",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return withSession(deviceServices, func(s *session) error {
			list, err := listDevices(s)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(list)
			}
			printDevices(list.Devices)
			return nil
		})
	},
}

var devicesMountCmd = &cobra.Command{
	Use:               "mount <device>",
	Short:             "Mount a device by /dev path, label, uuid or id",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDevices,
	RunE: func(_ *cobra.Command, args []string) error {
		return deviceAction("devices.mount", args[0], nil, func(out map[string]any) string {
			return fmt.Sprint(out["mountPoint"])
		})
	},
}

var devicesUnmountCmd = &cobra.Command{
	Use:               "unmount <device>",
	Short:             "Unmount a device",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDevices,
	RunE: func(_ *cobra.Command, args []string) error {
		return deviceAction("devices.unmount", args[0], map[string]any{"force": devicesForceFlag}, nil)
	},
}

var devicesEjectCmd = &cobra.Command{
	Use:               "eject <device>",
	Short:             "Unmount and eject removable media",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDevices,
	RunE: func(_ *cobra.Command, args []string) error {
		return deviceAction("devices.eject", args[0], nil, nil)
	},
}

func listDevices(s *session) (deviceList, error) {
	var list deviceList
	if err := s.call("devices.list", nil, &list); err != nil {
		return list, err
	}
	if !list.Available {
		return list, errors.New(cmp.Or(list.Reason, "udisks2 is unavailable"))
	}
	return list, nil
}

func deviceAction(method, target string, extra map[string]any, describe func(map[string]any) string) error {
	return withSession(deviceServices, func(s *session) error {
		list, err := listDevices(s)
		if err != nil {
			return err
		}
		dev, err := findDevice(list.Devices, target)
		if err != nil {
			return err
		}
		params := map[string]any{"id": dev.ID}
		for key, value := range extra {
			params[key] = value
		}
		var out map[string]any
		if err := s.call(method, params, &out); err != nil {
			return err
		}
		switch {
		case jsonOutput:
			return printJSON(out)
		case describe != nil:
			fmt.Println(describe(out))
		}
		return nil
	})
}

func findDevice(list []devices.Device, target string) (devices.Device, error) {
	for _, dev := range list {
		if target != "" && slices.Contains([]string{dev.ID, dev.Device, dev.UUID, dev.Label, dev.MountPoint}, target) {
			return dev, nil
		}
	}
	return devices.Device{}, fmt.Errorf("no device matches %s; see 'dfiles devices list'", target)
}

func printDevices(list []devices.Device) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEVICE\tNAME\tKIND\tFSTYPE\tSIZE\tMOUNTED AT")
	for _, dev := range list {
		if dev.Hidden && !devicesAllFlag {
			continue
		}
		name := cmp.Or(dev.Name, dev.Label)
		if dev.Kind == "system" && name == "" {
			name = "My Computer"
		}
		fstype := dev.Fstype
		if dev.Encrypted && !dev.Unlocked {
			fstype = "locked"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", dev.Device, cmp.Or(name, "-"), dev.Kind, cmp.Or(fstype, "-"), devices.HumanSize(dev.Size), cmp.Or(dev.MountPoint, "-"))
	}
	_ = w.Flush()
}

func completeDevices(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	_ = withSession(deviceServices, func(s *session) error {
		list, err := listDevices(s)
		for _, dev := range list.Devices {
			if !dev.Hidden {
				out = append(out, dev.Device+"\t"+cmp.Or(dev.Label, dev.Name, dev.Kind))
			}
		}
		return err
	})
	return out, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	devicesListCmd.Flags().BoolVar(&devicesAllFlag, "all", false, "Include system partitions the sidebar hides")
	devicesUnmountCmd.Flags().BoolVar(&devicesForceFlag, "force", false, "Unmount even while files are open")
	devicesCmd.AddCommand(devicesListCmd, devicesMountCmd, devicesUnmountCmd, devicesEjectCmd)
}
