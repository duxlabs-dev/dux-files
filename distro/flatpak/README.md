# Dux Files Flatpak

The build provides `dfiles`, the Quickshell runtime and QtPdf (the qtwebengine source with everything but pdfium switched off, for pdf previews) and installs the desktop file, metainfo and icon as `dev.duxlabs.files`.

## Posture

A file manager has to see the user's files, so the manifest grants `--filesystem=host`. That leaves little of the sandbox standing: the app reads and writes everything the user can, the same as Nautilus, Dolphin and Nemo on Flathub. What the sandbox still buys is the runtime isolation (its own Qt and libraries), the absence of privilege escalation (no pkexec, no polkit helper: `core/internal/elevate` refuses inside a sandbox) and the bus policy below. Portals are used only where they are the only route out of the sandbox: OpenURI to launch handlers and Settings for the color scheme today. Trash and FileTransfer are version-probed for later use; trash without host access is currently unavailable, not portal-backed.

Inside the sandbox with host access (or a `home` grant) the daemon points its own `XDG_*_HOME` and `XDG_*_DIRS` at the host tree (`core/internal/sandbox.ExportHostDirs`), so trash lands in `~/.local/share/Trash`, thumbnails in `~/.cache/thumbnails`, `mimeapps.list` and `user-dirs.dirs` are the user's real ones, and the host's application and mime databases under `/run/host/usr/share` are searched after the runtime's. Settings therefore live in `~/.config/dfiles` like a native install, not under `~/.var/app`. Flatpak only exports `HOST_XDG_*_HOME` when the host overrides that variable; otherwise the spec defaults under `$HOME` are used.

`/etc`, `/usr`, `/bin`, `/lib*` and `/sbin` inside the sandbox are the runtime's. The host's copies are at `/run/host/etc` and `/run/host/usr`; `system.capabilities` reports `hostOsRoot` so the UI can offer them.

## Permissions

| finish-arg | why | without it |
| --- | --- | --- |
| `--socket=wayland`, `--socket=fallback-x11`, `--share=ipc`, `--device=dri` | the Quickshell window | no window |
| `--filesystem=host` | browse and edit the user's files (`dankgo/files`, `internal/fileops`); also implies `host-os` and `host-etc` (`/run/host/usr`, `/run/host/etc`) | only `~/.var/app/dev.duxlabs.files` is visible; trash, thumbnails and mimeapps go to that private tree; `features.hostFilesystem`, `trash`, `setDefault` report `NOHOSTFS` |
| `--filesystem=/tmp`, `--filesystem=/var` | `host` leaves these private to the sandbox; both are browsed often enough to grant, and `/var/lib/flatpak/exports/share` holds other Flatpak apps' desktop entries | the sandbox's own empty `/tmp` and `/var`; `/boot`, `/efi` and `/root` are deliberately not granted |
| `--filesystem=xdg-run/gvfs`, `--filesystem=xdg-run/gvfsd`, `--talk-name=org.gtk.vfs.*` | gvfs FUSE paths, backend sockets and the mount tracker for network shares, MTP and archives (`internal/netmount`) | `features.network` reports `NOGVFS`; the Network section explains it and hides Connect to Server |
| `--system-talk-name=org.freedesktop.UDisks2` | mount, unmount, eject, unlock through udisks2, polkit inside udisks (`internal/devices`) | `features.devices` reports `NOUDISKS`; the Devices section explains it |
| `--talk-name=org.freedesktop.Flatpak` | `flatpak-spawn --host` for Open in terminal and running executables (`internal/mimeapps`) | `features.terminal` and `run` report `NOHOSTSPAWN`; the menu entry and shortcut are hidden and executables open instead of run |
| `--own-name=org.freedesktop.FileManager1` | answer `ShowFolders`, `ShowItems` and `ShowItemProperties` while the app runs (`internal/filemanager1`) | the request fails, the settings row says the session bus refused it, and other apps fall back to whatever else owns or activates the name |

Not granted on purpose: `--share=network` (gvfs does the networking on the host), `--socket=pulseaudio`, `org.freedesktop.secrets` (gvfs keeps share credentials itself), `xdg-config/mimeapps.list` (covered by `host`; the linter flags it as unnecessary). No D-Bus activation file is exported for FileManager1: flatpak refuses to export a service file whose `Name=` differs from its file name (`common/flatpak-dir.c`, "has wrong name"), so the sandboxed app only answers while it is running. The portal file chooser backend is not available from the sandbox either: flatpak exports only `share/applications`, `share/icons`, `share/dbus-1/services`, `share/mime/packages`, `share/metainfo` and a few desktop-specific dirs (`flatpak_export_dir` in `common/flatpak-dir.c`), never `share/xdg-desktop-portal/portals`, and the backend name would have to live under the app id. `system.capabilities` reports `portalChooser` with reason `SANDBOXED` and the setting is disabled.

Portals need no finish-args. Versions seen through `system.capabilities` on 2026-09-21: Trash 1, OpenURI 5, Settings 2, FileTransfer 1 (on `org.freedesktop.portal.Documents`).

`flatpak-builder-lint manifest` reports `finish-args-host-filesystem-access`, `finish-args-host-tmp-access`, `finish-args-host-var-access` and `finish-args-flatpak-spawn-access`. Each is intended and needs an entry in Flathub's linter exceptions when the app is submitted; the table above is the justification.

## Capabilities

`dfiles ipc system.capabilities` is what the UI consults. Facts: `sandboxed`, `appId`, `hostFilesystem`, `userFilesystem` (`host` or `home` grant), `hostOsRoot`, `filesystems` (the resolved grants from `/.flatpak-info`, overrides applied), `udisks`, `gvfs`, `hostSpawn` (bus policy plus `flatpak-spawn` on PATH), `portals` (interface version by name, only probed inside a sandbox), `dirs` (the XDG dirs in use), `thumbnailers` (`thumbnail.image` always, then one entry per media kind the `.thumbnailer` files on the XDG data dirs cover, plus `thumbnail.sandboxed` when bwrap runs them; inside the sandbox bwrap is skipped and host thumbnailers are not on PATH, so the KDE runtime yields images only).

`features` is the per-feature table, one of `available`, `available` + `degraded` with a reason, or unavailable with a reason:

| feature | needs | reason when missing |
| --- | --- | --- |
| `hostFilesystem` | `host` grant and `/run/host/usr` mounted | `NOHOSTFS` |
| `trash` | `host` or `home` grant (the real `~/.local/share/Trash`) | `NOHOSTFS` |
| `thumbnails` | always available; degraded to the private cache without a `host` or `home` grant | `NOHOSTFS` |
| `setDefault` | `host` or `home` grant (writable `~/.config/mimeapps.list`) | `NOHOSTFS` |
| `devices` | udisks2 bus policy | `NOUDISKS` |
| `network` | gvfs bus policy plus both runtime dirs | `NOGVFS` |
| `terminal`, `run` | `org.freedesktop.Flatpak` policy plus `flatpak-spawn` | `NOHOSTSPAWN` |
| `openWith` | native install; in the sandbox the portal's chooser decides, so it is degraded | `PORTALCHOOSER` |
| `elevation` | native install with pkexec and the polkit policy (live answer from `system.elevation`) | `SANDBOXED`, `NOPKEXEC`, `NOPOLICY`, `NOBINARY`, `NOROUTE` |

Whether a permitted service is running is not part of this table: `devices`, `network` and `jobs` topics carry that live. Reason codes are mapped to sentences in `quickshell/Common/FileFormat.qml`; codes it does not distinguish fall back to a generic sentence.

Native installs answer the same method with everything available, no portals and the plain XDG dirs.

## Build and install

`make flatpak-build` is the same command as below. The manifest pins a commit of the GitHub repository, which the sync workflow moves on each release; while the repository is private that clone fails inside `org.flatpak.Builder`, and the pin can trail the working tree. For a local build copy the manifest somewhere under `$HOME` (the builder cannot see `/tmp`), replace the `dux-files` git source with `type: dir`, `path: /path/to/checkout` and a `skip` list (`.git`, `builddir`, `repo`, `.flatpak-builder`, `core/bin`, `core/vendor`, `core/internal/shellembed/dist`), copy `go.mod.yml` and `modules.txt` next to it, and run the same command against that copy.

```bash
flatpak run org.flatpak.Builder \
  --force-clean --user --install --repo=repo \
  --mirror-screenshots-url=https://dl.flathub.org/media \
  --compose-url-policy=full \
  builddir distro/flatpak/dev.duxlabs.files.yml
```

```bash
flatpak run dev.duxlabs.files
flatpak run dev.duxlabs.files open ~/Downloads
```

`make flatpak-modules` regenerates `go.mod.yml` and `modules.txt` from `core/go.mod` with `flatpak-go-mod`.

## Uninstall

```bash
flatpak uninstall --delete-data dev.duxlabs.files
```

Settings written while `host` was granted stay in `~/.config/dfiles` (see Posture).

## Lint

```bash
make flatpak-lint
```
