package netmount

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/files"
)

var kernelNetworkFsTypes = []string{
	"nfs", "nfs4", "cifs", "smb3", "smbfs", "9p", "afs", "ceph", "glusterfs", "autofs",
	"davfs", "fuse.sshfs", "fuse.gvfsd-fuse", "fuse.rclone",
}

var mountEscapes = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

func kernelNetworkMounts() []Mount {
	return parseKernelNetworkMounts(readMountInfo(), readFstab())
}

func parseKernelNetworkMounts(mountinfo, fstab string) []Mount {
	mounted := parseMountedNetworkMounts(mountinfo)
	mounted = slices.DeleteFunc(mounted, func(mount Mount) bool {
		return mount.Scheme == "fuse.gvfsd-fuse"
	})
	mountedByPath := make(map[string]Mount, len(mounted))
	for _, mount := range mounted {
		mountedByPath[mount.Path] = mount
	}

	seen := make(map[string]bool, len(mounted))
	var out []Mount
	for _, configured := range parseFstabNetworkMounts(fstab) {
		if current, ok := mountedByPath[configured.Path]; ok {
			current.Configured = true
			out = append(out, current)
		} else {
			out = append(out, configured)
		}
		seen[configured.Path] = true
	}
	for _, mount := range mounted {
		if !seen[mount.Path] {
			out = append(out, mount)
		}
	}
	return out
}

func MountedNetworkPaths(mountinfo string) []string {
	mounts := parseMountedNetworkMounts(mountinfo)
	paths := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		paths = append(paths, mount.Path)
	}
	return paths
}

func parseMountedNetworkMounts(mountinfo string) []Mount {
	var out []Mount
	for line := range strings.SplitSeq(mountinfo, "\n") {
		mount, filesystem, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		fields := strings.Fields(mount)
		kind := strings.Fields(filesystem)
		if len(fields) < 5 || len(kind) == 0 || !slices.Contains(kernelNetworkFsTypes, kind[0]) {
			continue
		}
		out = append(out, kernelMount(mountEscapes.Replace(fields[4]), sourceOf(kind), kind[0], true, false))
	}
	return out
}

func parseFstabNetworkMounts(fstab string) []Mount {
	var out []Mount
	for line := range strings.SplitSeq(fstab, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || !slices.Contains(kernelNetworkFsTypes, fields[2]) {
			continue
		}
		path := mountEscapes.Replace(fields[1])
		if path == "none" || !filepath.IsAbs(path) {
			continue
		}
		out = append(out, kernelMount(path, mountEscapes.Replace(fields[0]), fields[2], false, true))
	}
	return out
}

func kernelMount(path, source, scheme string, mounted, configured bool) Mount {
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = path
	}
	icon := "folder-unmounted"
	if mounted {
		icon = "folder-remote"
	}
	return Mount{
		ID:         "kernel:" + path,
		Name:       name,
		URI:        source,
		Path:       path,
		Scheme:     scheme,
		Kind:       KindNetwork,
		IconName:   icon,
		Kernel:     true,
		Configured: configured,
		Mounted:    mounted,
	}
}

func sourceOf(fields []string) string {
	if len(fields) < 2 {
		return ""
	}
	return mountEscapes.Replace(fields[1])
}

func unmountKernelMount(path string) error {
	if path == "" {
		return wire.Fail(files.CodeInvalid, "network mount has no mount point")
	}
	for _, mount := range parseMountedNetworkMounts(readMountInfo()) {
		if mount.Path != path {
			continue
		}
		cmd := exec.CommandContext(context.Background(), "umount", "--", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}
			if strings.Contains(strings.ToLower(message), "busy") {
				return wire.Fail(files.CodeBusy, "could not unmount %s: %s", path, message)
			}
			return wire.Fail(files.CodeIO, "could not unmount %s: %s", path, message)
		}
		return nil
	}
	return wire.Fail(files.CodeNotFound, "network mount is no longer mounted: %s", path)
}

func mountKernelMount(path string) error {
	for _, mount := range parseFstabNetworkMounts(readFstab()) {
		if mount.Path != path {
			continue
		}
		if err := exec.CommandContext(context.Background(), "mount", path).Run(); err != nil {
			return wire.Fail(files.CodeIO, "could not mount %s: %v", path, err)
		}
		return nil
	}
	return wire.Fail(files.CodeNotFound, "network mount is not configured: %s", path)
}

func readMountInfo() string {
	data, _ := os.ReadFile("/proc/self/mountinfo")
	return string(data)
}

func readFstab() string {
	data, _ := os.ReadFile("/etc/fstab")
	return string(data)
}
