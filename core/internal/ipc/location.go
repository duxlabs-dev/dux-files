package ipc

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/duxlabs-dev/dux-files/core/internal/pathutil"

	"github.com/duxlabs-dev/dux-files/core/internal/location"
	"github.com/duxlabs-dev/dux-files/core/internal/wire"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

const missingCheckLimit = 500

var networkFsTypes = []string{"nfs", "nfs4", "cifs", "smb3", "smbfs", "9p", "afs", "ceph", "glusterfs", "autofs", "davfs", "fuse.sshfs", "fuse.gvfsd-fuse", "fuse.rclone"}

var mountEscapes = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

func HandleLocation(_ context.Context, w *ConnWriter, req Request) {
	switch req.Method {
	case "location.resolve":
		home, _ := os.UserHomeDir()
		env := location.HomeEnv(nil)
		loc, err := location.Typed(params.StringOpt(req.Params, "text", ""), params.StringOpt(req.Params, "cwd", ""), home, env)
		if err != nil {
			wire.RespondError(w, req.ID, err)
			return
		}
		w.Respond(req.ID, map[string]any{"location": loc})
	case "location.missing":
		mountinfo, _ := os.ReadFile("/proc/self/mountinfo")
		w.Respond(req.ID, map[string]any{"missing": missingFolders(wire.StringList(req.Params, "paths"), networkMounts(string(mountinfo)))})
	default:
		w.RespondError(req.ID, "unknown location method: "+req.Method)
	}
}

func parentExists(path string) bool {
	info, err := os.Stat(filepath.Dir(path))
	return err == nil && info.IsDir()
}

func networkMounts(mountinfo string) []string {
	var out []string
	for line := range strings.SplitSeq(mountinfo, "\n") {
		mount, filesystem, ok := strings.Cut(line, " - ")
		fields := strings.Fields(mount)
		kind := strings.Fields(filesystem)
		if !ok || len(fields) < 5 || len(kind) == 0 || !slices.Contains(networkFsTypes, kind[0]) {
			continue
		}
		out = append(out, mountEscapes.Replace(fields[4]))
	}
	return out
}

func onMount(path string, mounts []string) bool {
	return slices.ContainsFunc(mounts, func(mount string) bool {
		return pathutil.Within(path, mount)
	})
}

func missingFolders(candidates, networks []string) []string {
	missing := []string{}
	for index, path := range candidates {
		if index >= missingCheckLimit {
			break
		}
		if !filepath.IsAbs(path) || onMount(path, networks) {
			continue
		}
		info, err := os.Stat(path)
		switch {
		case err == nil && !info.IsDir():
			missing = append(missing, path)
		case os.IsNotExist(err) && parentExists(path):
			missing = append(missing, path)
		}
	}
	return missing
}
