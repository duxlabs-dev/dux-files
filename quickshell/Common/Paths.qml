pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import QtCore
import qs.Services

Singleton {
    id: root

    readonly property url home: StandardPaths.standardLocations(StandardPaths.HomeLocation)[0]
    readonly property url xdgCache: StandardPaths.standardLocations(StandardPaths.GenericCacheLocation)[0]
    readonly property url cache: `${xdgCache}/dfiles`
    readonly property url imagecache: `${cache}/imagecache`
    readonly property url stateDir: `${StandardPaths.writableLocation(StandardPaths.GenericStateLocation)}/dfiles`
    readonly property string configDir: {
        const xdg = Quickshell.env("XDG_CONFIG_HOME");
        const base = (xdg && xdg !== "") ? xdg : Quickshell.env("HOME") + "/.config";
        return base + "/dfiles";
    }
    property bool ready: false

    function stringify(path: url): string {
        return path.toString().replace(/%20/g, " ");
    }

    function strip(path: url): string {
        return stringify(path).replace("file://", "");
    }

    function shortenHome(path: string): string {
        const home = strip(root.home);
        if (path === home)
            return "~";
        if (path.startsWith(home + "/"))
            return "~" + path.slice(home.length);
        return path;
    }

    property var _iconPaths: ({})
    property var themePaths: ({})
    property string iconThemeSetting: "auto"
    property string iconThemeEffective: ""
    property bool _themePathsLoaded: false

    function applyIconTheme(result) {
        const changed = iconThemeEffective !== (result.effective || "");
        iconThemeSetting = result.setting || "auto";
        iconThemeEffective = result.effective || "";
        if (changed) {
            _iconPaths = ({});
            themePaths = ({});
            _themePathsLoaded = false;
            loadThemePaths();
        }
    }

    function refreshIconTheme() {
        DuxFilesService.sendRequest("files.iconThemes", null, response => {
            if (response.error)
                return;
            applyIconTheme(response.result || {});
            loadThemePaths();
        });
    }

    function loadThemePaths() {
        if (_themePathsLoaded)
            return;
        _themePathsLoaded = true;
        const expectedTheme = iconThemeEffective;
        DuxFilesService.sendRequest("files.iconThemePaths", null, response => {
            if (expectedTheme !== iconThemeEffective)
                return;
            if (response.error) {
                _themePathsLoaded = false;
                return;
            }
            themePaths = response.result || ({});
        });
    }

    Connections {
        target: DuxFilesService
        function onConnectedChanged() {
            if (DuxFilesService.connected)
                root.refreshIconTheme();
        }
        function onIconThemeUpdate(data) { root.applyIconTheme(data || {}); }
    }

    Component.onCompleted: {
        if (DuxFilesService.connected)
            refreshIconTheme();
    }

    function resolveIconPath(iconName: string): string {
        if (!iconName)
            return "";
        const literal = literalIconSource(iconName);
        if (literal)
            return literal;
        const themed = themePaths[iconName];
        if (themed)
            return Qt.resolvedUrl("file://" + themed).toString();
        const cached = _iconPaths[iconName];
        if (cached !== undefined)
            return cached;
        const resolved = literalIconSource(iconName) || Quickshell.iconPath(iconName, true) || bundledIconPath(iconName);
        if (resolved !== "")
            _iconPaths[iconName] = resolved;
        return resolved;
    }

    function literalIconSource(source: string): string {
        switch (true) {
        case source.startsWith("/"):
            return "file://" + source;
        case /^https?:\/\//.test(source):
            return source;
        default:
            return "";
        }
    }

    function bundledIconPath(iconName: string): string {
        switch (iconName) {
        case "folder":
        case "folder-documents":
        case "folder-download":
        case "folder-music":
        case "folder-pictures":
        case "folder-publicshare":
        case "folder-remote":
        case "folder-templates":
        case "folder-videos":
            return Qt.resolvedUrl(`../assets/icons/${iconName}.svg`).toString();
        case "user-home":
        case "user-desktop":
        case "inode-directory":
            return Qt.resolvedUrl("../assets/icons/folder.svg").toString();
        default:
            return "";
        }
    }

    function copyPathToClipboard(path: string): void {
        Quickshell.clipboardText = path;
    }

    Process {
        command: ["mkdir", "-p", root.strip(root.imagecache), root.configDir, root.strip(root.stateDir)]
        running: true
        onExited: code => {
            if (code !== 0)
                Log.warn(`mkdir of the app dirs exited with ${code}`);
            root.ready = true;
        }
    }
}
