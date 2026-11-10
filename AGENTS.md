## What is this?

Dux Files is an open-source, MIT-licensed standalone file manager for linux. It runs as a daemon with a quickshell UI and shares the look and feel of Dux Shell.

## Repo Structure

Two apps that operate together, plus a `dux-qml-common` submodule holding the shared QML widget library used across the Dux suite (symlinked in as `quickshell/DCommon`), and a `dux-qt-base` submodule, the quickshell runtime fork.

### 1- core

Go backend, daemon, and CLI (`cmd/dfiles`). Runs the unix socket server the UI talks to.

- cmd/dfiles/ - CLI commands and the daemon entrypoint
- internal/ipc/ - unix socket handlers the UI calls, plus the method registry that drives `dfiles ipc list` and shell completion
- internal/colorscheme/ - desktop portal color-scheme watcher
- internal/shellembed/ - the QML tree embedded into the binary at build time (`make sync-shell`)
- internal/qtbaseembed/ - libdux-qt-base.so embedded with `QTBASE=1` (withqtbase tag). By default `dfiles qs` dlopens an installed libdux-qt-base.so.0, next to `dfiles` or in the system library dirs, and fails without one; `dfiles run --qs-child` (DFILES_QS_CHILD=1) runs the UI on upstream `qs` instead. The QML must keep loading on upstream quickshell: fork-only pragmas and window properties are warnings or guarded assignments, never bindings.

### 2- quickshell

The entire UI, a quickshell app. Talks to core over two unix sockets: one for requests, one for the event subscription stream.

- Common/ - Theme, SettingsData, I18n, Paths, Anims, CacheData, SessionData
- Services/ - DuxFilesService (the core bridge), Log, PortalService, SessionService
- Modules/ - MainWindow
- translations/ - POEditor-synced catalogs

`shell.qml` injects the app singletons into DCommon at startup (`DC.Style.theme`, `DC.Style.settings`, `DC.I18n.backend`, `DC.Paths.backend`, `DC.Log.backend`, `DC.Host.session`, `DC.Host.cache`). The stubs in `dux-qml-common/Common/` and `dux-qml-common/Services/` are the full list of properties the library reads. When bumping the submodule, diff the stub singletons against ours and add anything new.

### Other directories

- distro/ - nix and flatpak packaging
- assets/ - icon, desktop entry, metainfo, systemd unit

## General Rules

- Keep it simple. Do not overcomplicate things.
- Follow each apps' own conventions. QML uses Theme tokens instead of hardcoded colors, spacing, or other constants.
- Resource usage matters. The daemon runs 24/7 - audit any change for idle CPU cost, extra processes, timers, and retained memory.
- `core` and `quickshell` are tightly coupled through the unix socket protocol. When you are changing one, be mindful of the consequences for the other.
- Use the D* widgets in DCommon/ (DListView, DFlickable, and so on) instead of raw ListView/Flickable/ScrollView.
- All user-facing text goes through `I18n.tr()` with translator context as the second argument. Prefer reusing existing terms in `quickshell/translations/en.json` over adding new ones. Never edit the translation catalogs by hand, they are synced with POEditor. Strings under `DCommon/` belong to dux-qml-common, not this repo.
- Go must build and test with `CGO_ENABLED=0`. No cgo dependencies.
- Do not start editing code in response to a question. We'll tell you when to edit code.
- Do not leave paragraphs of comments on top of the code. You should try to avoid them as much as possible with understandable function names and code. If they are necessary even then, make them concise. Remove such comments when you come by them in the codebase. Comments should always move with code, not be left behind.
- Use guard statement patterns in any code you write. Prefer switch/case over if/else chains.
- Do not write any useless tests, tests should cover input output validation - not useless things like "main.go contains func main()". Go tests use testify and must run sandboxed: no network, no system services, `t.TempDir()`.
- If we are missing a glaring issue when we ask you to do something, do not hesitate to point it out.
- Reinvent the wheel but do not reinvent the car. If you are solving a simple problem do not introduce a library. If you are solving a complex but a common problem, there is likely a modern library for it, if so, use it.
- Never commit or push code unless explicitly asked to do so.
- Never make a PR unless explicitly asked to do so.

## Common Commands

Root Makefile delegates to `core/Makefile`. From `core/`:

- `make build` - sync the QML tree into the binary and build
- `make test`, `make cover`, `make fmt`, `make vet`
- `DFILES_ENABLE_HOTRELOAD=1 go run ./cmd/dfiles run -c ../quickshell` - run against the repo QML with hot reload

## Commit Messages

Commit messages start with the part of the system they touched, followed by a short lowercase explanation of the work:

ipc: add ui.open for directories
theme: read display font from settings

The title should be concise. Description should explain the work in more detail (only if required) while still being concise. Use simple language, do not try to sound smart.
