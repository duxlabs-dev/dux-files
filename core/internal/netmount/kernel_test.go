package netmount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKernelNetworkMounts(t *testing.T) {
	mountinfo := `22 1 259:2 / / rw,relatime - ext4 /dev/nvme0n1p2 rw
98 22 0:55 / /mnt/nas\040share rw,relatime - nfs4 nas:/export rw
99 22 0:56 / /home/me/remote rw - fuse.sshfs me@host: rw
`
	fstab := `//nas/share /mnt/nas\040share cifs noauto,credentials=/run/secret
//nas/photos /mnt/photos cifs noauto,credentials=/run/secret
/home/me/local /mnt/local ext4 defaults
`

	mounts := parseKernelNetworkMounts(mountinfo, fstab)
	if assert.Len(t, mounts, 3) {
		assert.Equal(t, Mount{
			ID: "kernel:/mnt/nas share", Name: "nas share", URI: "nas:/export", Path: "/mnt/nas share",
			Scheme: "nfs4", Kind: KindNetwork, IconName: "folder-remote", Kernel: true, Configured: true, Mounted: true,
		}, mounts[0])
		assert.Equal(t, Mount{
			ID: "kernel:/mnt/photos", Name: "photos", URI: "//nas/photos", Path: "/mnt/photos",
			Scheme: "cifs", Kind: KindNetwork, IconName: "folder-unmounted", Kernel: true, Configured: true,
		}, mounts[1])
		assert.Equal(t, "/home/me/remote", mounts[2].Path)
	}
}
