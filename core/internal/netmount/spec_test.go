package netmount

import (
	"testing"

	"github.com/duxlabs-dev/dux-go/files"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Taken from ListMountableInfo on a machine with the stock gvfs backends.
// dav.mount lists Type=dav;davs, so the tracker reports one mountable per type.
var mountables = []Mountable{
	{Type: "sftp", Scheme: "sftp", Aliases: []string{"ssh"}, DefaultPort: 22, HostIsInet: true},
	{Type: "ftp", Scheme: "ftp", DefaultPort: 21, HostIsInet: true},
	{Type: "dav", Scheme: "dav"},
	{Type: "davs", Scheme: "davs"},
	{Type: "dav+sd", Scheme: "dav+sd"},
	{Type: "davs+sd", Scheme: "davs+sd"},
	{Type: "smb-share", Scheme: "smb", HostIsInet: true},
	{Type: "archive", Scheme: "archive"},
	{Type: "mtp", Scheme: "mtp"},
}

func TestAvailableSchemesKeepsOnlyInstalledBackends(t *testing.T) {
	assert.Equal(t, []string{"sftp", "smb", "ftp", "dav", "davs"}, availableSchemes(mountables))
	assert.Equal(t, []string{}, availableSchemes(nil))
}

func TestParseURIBuildsTheBackendSpec(t *testing.T) {
	spec, err := ParseURI("sftp://ana@files.example/srv/media", mountables)
	require.NoError(t, err)

	assert.Equal(t, "sftp", spec.Type())
	assert.Equal(t, "files.example", spec.Items["host"])
	assert.Equal(t, "ana", spec.Items["user"])
	assert.Equal(t, "/srv/media", spec.Prefix)
	assert.NotContains(t, spec.Items, "port")
}

func TestParseURIKeepsOnlyNonDefaultPorts(t *testing.T) {
	spec, err := ParseURI("sftp://host:22/", mountables)
	require.NoError(t, err)
	assert.NotContains(t, spec.Items, "port")

	spec, err = ParseURI("sftp://host:2222/", mountables)
	require.NoError(t, err)
	assert.Equal(t, "2222", spec.Items["port"])
}

func TestParseURIAcceptsSchemeAliases(t *testing.T) {
	spec, err := ParseURI("ssh://host/", mountables)
	require.NoError(t, err)
	assert.Equal(t, "sftp", spec.Type())
}

func TestParseURISplitsAnSMBShare(t *testing.T) {
	spec, err := ParseURI("smb://WORK;ana@server/public/reports", mountables)
	require.NoError(t, err)

	assert.Equal(t, "smb-share", spec.Type())
	assert.Equal(t, "server", spec.Items["server"])
	assert.Equal(t, "public", spec.Items["share"])
	assert.Equal(t, "WORK", spec.Items["domain"])
	assert.Equal(t, "ana", spec.Items["user"])
	assert.Equal(t, "/reports", spec.Prefix)
}

func TestParseURIBrowsesAnSMBServerWithoutAShare(t *testing.T) {
	spec, err := ParseURI("smb://server", mountables)
	require.NoError(t, err)

	assert.Equal(t, "smb-server", spec.Type())
	assert.Equal(t, "server", spec.Items["server"])
	assert.Equal(t, "/", spec.Prefix)
}

func TestParseURIRejectsWhatGvfsCannotMount(t *testing.T) {
	_, err := ParseURI("wat://host/", mountables)
	assert.Equal(t, files.CodeNotSupported, files.CodeOf(err))

	_, err = ParseURI("host/share", mountables)
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))

	_, err = ParseURI("", mountables)
	assert.Equal(t, files.CodeInvalid, files.CodeOf(err))
}

func TestURIOfRebuildsWhatWasMounted(t *testing.T) {
	spec, err := ParseURI("ftp://probe@127.0.0.1:2121/pub", mountables)
	require.NoError(t, err)
	assert.Equal(t, "ftp://probe@127.0.0.1:2121/pub", URIOf(spec, mountables))
}

func TestDavsCarriesTheSSLItem(t *testing.T) {
	spec, err := ParseURI("davs://ana@cloud.example/remote.php/dav", mountables)
	require.NoError(t, err)
	assert.Equal(t, "dav", spec.Type(), "gvfsd-dav registers no davs backend and answers Invalid backend type to it")
	assert.Equal(t, "true", spec.Items["ssl"], "gvfsd-dav picks https from ssl=true")
	assert.Equal(t, "davs://ana@cloud.example/remote.php/dav", URIOf(spec, mountables))

	discovered, err := ParseURI("davs+sd://cloud.example/", mountables)
	require.NoError(t, err)
	assert.Equal(t, "dav+sd", discovered.Type())
	assert.Equal(t, "true", discovered.Items["ssl"])

	plain, err := ParseURI("dav://cloud.example/", mountables)
	require.NoError(t, err)
	assert.NotContains(t, plain.Items, "ssl")
	assert.Equal(t, "dav://cloud.example/", URIOf(plain, mountables))
}

func TestURIOfRebuildsAnSMBShare(t *testing.T) {
	spec, err := ParseURI("smb://ana@server/public/reports", mountables)
	require.NoError(t, err)
	assert.Equal(t, "smb://ana@server/public/reports", URIOf(spec, mountables))
}

func TestSpecEncodingCarriesTheTrailingNul(t *testing.T) {
	spec := Spec{Prefix: "/", Items: map[string]string{"type": "sftp", "host": "example"}}
	wire := encodeSpec(spec)

	assert.Equal(t, []byte("/\x00"), wire.Prefix)
	assert.Equal(t, []byte("sftp\x00"), wire.Items["type"].Value())
	assert.Equal(t, spec, decodeSpec(wire))
}

func TestDecodeMountDescribesAGvfsMount(t *testing.T) {
	raw := wireMount{
		DBusID:         ":1.42",
		ObjectPath:     dbus.ObjectPath("/org/gtk/vfs/mount/1"),
		DisplayName:    "probe on 127.0.0.1:2121",
		StableName:     "ftp:host=127.0.0.1,port=2121,user=probe",
		Icon:           ". GThemedIcon folder-remote folder folder-remote-symbolic",
		UserVisible:    true,
		FuseMountpoint: []byte("/run/user/1000/gvfs/ftp:host=127.0.0.1\x00"),
		Spec: wireSpec{
			Prefix: []byte("/\x00"),
			Items: map[string]dbus.Variant{
				"type": dbus.MakeVariant([]byte("ftp\x00")),
				"host": dbus.MakeVariant([]byte("127.0.0.1\x00")),
				"port": dbus.MakeVariant([]byte("2121\x00")),
				"user": dbus.MakeVariant([]byte("probe\x00")),
			},
		},
	}

	mount := decodeMount(raw, mountables)
	assert.Equal(t, "ftp:host=127.0.0.1,port=2121,user=probe", mount.ID)
	assert.Equal(t, "ftp://probe@127.0.0.1:2121/", mount.URI)
	assert.Equal(t, "/run/user/1000/gvfs/ftp:host=127.0.0.1", mount.Path)
	assert.Equal(t, "ftp", mount.Scheme)
	assert.Equal(t, KindNetwork, mount.Kind)
	assert.Equal(t, "folder-remote", mount.IconName)
}

func TestKindSortsBackendsIntoSections(t *testing.T) {
	assert.Equal(t, KindNetwork, kindOf("smb-share"))
	assert.Equal(t, KindDevice, kindOf("mtp"))
	assert.Equal(t, KindOther, kindOf("trash"))
}

func TestThemedIconFallsBackWhenTheIconIsNotThemed(t *testing.T) {
	assert.Equal(t, "folder-remote", themedIcon(""))
	assert.Equal(t, "drive-removable-media", themedIcon(". GThemedIcon drive-removable-media drive"))
}

func TestLocationInAMountFollowsTheRequestedPath(t *testing.T) {
	sftp := Mount{URI: "sftp://host/", Path: "/run/user/1000/gvfs/sftp:host=host"}
	share := Mount{URI: "smb://nas/public", Path: "/run/user/1000/gvfs/smb-share:server=nas,share=public"}

	tests := []struct {
		name  string
		mount Mount
		uri   string
		want  string
	}{
		{"host root", sftp, "sftp://host/", sftp.Path},
		{"deep path", sftp, "sftp://host/srv/my media", sftp.Path + "/srv/my media"},
		{"share root", share, "smb://nas/public", share.Path},
		{"inside a share", share, "smb://nas/public/reports/2026", share.Path + "/reports/2026"},
		{"sibling share with a shared prefix", share, "smb://nas/publicity", share.Path},
		{"unrelated uri", share, "ftp://elsewhere/", share.Path},
		{"dot segments stay inside the mount", sftp, "sftp://host/../../../../etc", sftp.Path + "/etc"},
		{"no local path", Mount{URI: "sftp://host/"}, "sftp://host/srv", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LocationIn(tt.mount, tt.uri))
		})
	}
}

func TestLocationInFromParsedURIs(t *testing.T) {
	spec, err := ParseURI("smb://WORK;ana@server/public/reports", mountables)
	require.NoError(t, err)
	mount := Mount{URI: "smb://ana@server/public", Path: "/gvfs/smb-share:server=server,share=public,user=ana"}
	assert.Equal(t, mount.Path+"/reports", LocationIn(mount, URIOf(spec, mountables)))
}
