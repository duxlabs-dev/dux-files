# Dux Files

<div align="center">
  <img src="assets/dev.duxlabs.files.svg" alt="Dux Files" width="160">

### A file manager for the dux desktop

Built with [Quickshell](https://quickshell.org/) and [Go](https://go.dev/)

</div>

Dux Files is a standalone file manager for Wayland. A small Go daemon does the file work and a Quickshell UI sits on top of it. It looks and feels like the rest of the dux desktop and picks up Dux Shell's colors when DMS is running, but it works fine on its own.

## Features

**Tabs, panes and views**
Tabs, a split view with two panes, and list, grid and column views. Sort and view can be remembered per folder.

**Fast navigation**
Type to jump to a name or filter the folder, jump to any folder you have visited before, and reopen closed tabs.

**Search**
Recursive search by name with filters. Uses [danksearch](https://github.com/duxlabs-dev/dux-search) when it is running and walks the tree when it isn't.

**File operations**
Copy, move, rename, trash and delete run as jobs with progress and conflict handling, and most of them can be undone. Denied operations offer Retry as administrator through polkit.

**Archives**
Opening an archive extracts it next to itself, like Nautilus. Compress to zip, tar.gz, tar.xz and tar.zst. All pure Go, no external tools.

**Devices and network**
Mount, unmount and eject drives through udisks2. Connect to sftp, smb, ftp, WebDAV, nfs and afp shares through gvfs.

**Trash, recent files and bookmarks**
Browse and restore the trash, see recently used files, and keep bookmarks in the sidebar.

**Open and save dialogs**
Can serve the desktop portal's file chooser, so other apps get the same window, sidebar and keys when they open or save a file.

**Desktop integration**
Owns `org.freedesktop.FileManager1`, so "show in folder" from browsers and other apps lands here.

## Installation

### From source

You need Go and `quickshell`.

```bash
git clone --recurse-submodules https://github.com/duxlabs-dev/dux-files.git
cd dux-files
make
sudo make install
```

`make install` puts the `dfiles` binary (UI included), the desktop entry, icon, metainfo, polkit policy and the D-Bus activation files under `/usr/local`. `sudo make uninstall` removes them.

To keep it running in the background so windows open instantly:

```bash
make install-systemd
systemctl --user enable --now dfiles
```

### NixOS / home-manager

```nix
{
  inputs.dux-files.url = "github:duxlabs-dev/dux-files";

  # home-manager or NixOS
  imports = [ inputs.dux-files.homeModules.dux-files ];
  programs.dux-files.enable = true;
}
```

Use `nixosModules.dux-files` for a NixOS configuration. `programs.dux-files.systemd.enable` starts it with the session, and `programs.dux-files.portal.enable` registers the open and save dialogs.

### Flatpak

See [distro/flatpak/README.md](distro/flatpak/README.md).

### Optional dependencies

Picked up from `PATH` when the daemon starts:

| package | enables |
| --- | --- |
| `bubblewrap` | runs external thumbnailers in a sandbox, like gnome-desktop does |
| `ffmpegthumbnailer` | video thumbnails |
| `poppler` (`poppler-utils` on Debian) | pdf thumbnails |
| `gvfs` | network shares and the trash on removable drives |
| `udisks2` | the devices sidebar |

Images in png, jpeg, gif, bmp, tiff and webp are decoded in-process. Anything else uses the freedesktop thumbnailers installed on the system, so packages that ship one (glycin, gsf, evince, totem) just work. `dfiles ipc system.capabilities` shows what was found.

PDF previews use the optional `QtQuick.Pdf` QML module: `qt6-webengine` on Arch, `qt6-qtpdf` on Fedora, `qt6-pdf-imports` on openSUSE, `qml6-module-qtquick-pdf` on Debian and Ubuntu. The nix package wires it in by default, `dux-files.override { withPdfPreview = false; }` drops it.
Without it, the preview shows an installation hint; image previews and the rest of the file manager still work.

## Usage

```bash
dfiles show                     # Show the window, starting dfiles if needed
dfiles open ~/Downloads         # Open folders, one tab each
dfiles open --new-window ~/src  # Same, in a new window
dfiles select ~/notes.txt       # Open the folder with the file selected
dfiles properties ~/notes.txt   # Properties dialog
```

`dfiles open` accepts plain paths and these locations:

| location | opens |
| --- | --- |
| `/path`, `relative/path`, `file:///path` | that path |
| `trash:///` | the trash |
| `computer:///` | `/` |
| `network:///` | the Connect to Server dialog |
| `admin:///path` | the path, with Retry as administrator on denied operations |
| `sftp:// ssh:// smb:// ftp:// ftps:// dav:// davs:// nfs:// afp://` | mounted through gvfs, then opened in a new tab |

Started by the systemd unit, closing the window just hides it. Started any other way, closing the last window quits unless Settings, Close behavior says otherwise.

### Command line

The CLI talks to the running dfiles when there is one and does the work itself when there isn't. `--json` gives machine-readable output.

```bash
dfiles trash put|list|count|restore|empty
dfiles mime query|default|set
dfiles devices list|mount|unmount|eject
dfiles search <root> <query>
dfiles ipc list                 # Every IPC method the daemon exposes
dfiles ipc <method> key=value...
```

Jobs started from the CLI answer their own prompts. A restore that finds its original path taken keeps both (`--conflict skip|replace` changes that), and a failed item is skipped, reported, and makes the command exit 1.

## Default file manager

To open folders here from other apps:

```bash
xdg-mime default dev.duxlabs.files.desktop inode/directory
```

The installer doesn't do this for you.

While it runs, dfiles owns `org.freedesktop.FileManager1`, so `ShowFolders`, `ShowItems` and `ShowItemProperties` calls come here. Chromium based browsers go through the portal's `OpenDirectory`, which forwards to `ShowItems`. dfiles never takes the name from another file manager, it waits in line until that one quits. Turn it off with Settings, General, "Register as the session file manager".

`make install` also ships an activation file for the name, so a call can start dfiles when nothing owns it. Nautilus, Dolphin, Thunar and Nemo ship one too. dbus-daemon prefers the first service directory, and the default `PREFIX=/usr/local` comes before `/usr/share`.

## Open and save dialogs

Nothing changes until you turn it on. Use Settings, General, "Use Dux Files for open and save dialogs", or:

```bash
dfiles portal install
dfiles portal status
dfiles portal uninstall   # put things back
```

This puts dfiles first for the FileChooser in your `~/.config/xdg-desktop-portal/<desktop>-portals.conf`, keeps the old backend as a fallback, and remembers what it changed. xdg-desktop-portal only reads its config at startup, so it has to restart. Both the settings page and the CLI ask first, since a restart cancels dialogs other apps have open.

Apps that use the portal pick it up: Flatpak apps, Chromium browsers, Firefox with `widget.use-xdg-desktop-portal.file-picker` set to 1, GTK 4 apps with `GDK_DEBUG=portals` and Qt apps with `QT_QPA_PLATFORMTHEME=xdgdesktopportal`. Apps that draw their own dialogs are unchanged.

The dialog is its own window. Qt 6.12 and Quickshell can't attach it to the calling app's window, so `modal` is ignored. Without the systemd unit the first dialog is a cold start. A niri rule to float it:

```kdl
window-rule {
    match app-id=r#"^dev\.duxlabs\.files$"# title=r#"^(Open|Save|Select|Upload|Export|Import)"#
    open-floating true
}
```

Packagers: install `assets/portal/dfiles.portal` under `share/xdg-desktop-portal/portals/` and the service file under `share/dbus-1/services/` with the binary path substituted. Don't ship a `portals.conf` that prefers it. The Flatpak build can't register the backend because Flatpak exports no portal files.

## Repository Structure

```
dux-files/
├── core/               # Go daemon and the dfiles CLI
│   ├── cmd/dfiles/     # CLI commands and the daemon entrypoint
│   └── internal/       # File ops, IPC, devices, search, portal, D-Bus
├── quickshell/         # The UI
│   ├── Common/         # Theme, settings, i18n
│   ├── Services/       # Bridges to the daemon
│   ├── Modules/        # Windows, panes, dialogs, settings
│   └── translations/   # POEditor catalogs
├── dux-qml-common/    # Shared widget library (submodule, linked as quickshell/DCommon)
├── distro/             # Nix and Flatpak packaging
└── assets/             # Icon, desktop entry, D-Bus, portal and systemd files
```

## Development

```bash
make run                                                  # Build and run against the repo's QML
DFILES_ENABLE_HOTRELOAD=1 core/bin/dfiles run -c quickshell  # Hot reload the UI
make test                                                 # Go tests
make test-qml                                             # QML tests
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, the shared widget library, translations and our rules on AI generated code.

## License

MIT License - See [LICENSE](LICENSE) for details.
