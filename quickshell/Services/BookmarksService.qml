pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    property var entries: []
    property bool gtkImportAvailable: false
    property bool importOffer: false
    property bool seedPending: false

    readonly property var paths: entries.map(entry => entry.path)

    signal requestFailed(string message)

    readonly property DaemonTopic _bookmarks: DaemonTopic {
        transport: root.transport
        topic: "bookmarks"
        handler: data => root._onEvent(data)
        onConnected: root.refresh()
    }

    function has(path) {
        return paths.includes(path);
    }

    function labelFor(path) {
        return entries.find(entry => entry.path === path)?.label ?? "";
    }

    function refresh() {
        transport.call("bookmarks.list", null, result => {
            if (result.error)
                return;
            root.entries = result.bookmarks || [];
            root.gtkImportAvailable = result.gtkImportAvailable === true;
            root.importOffer = root.gtkImportAvailable && !SettingsData.gtkBookmarksOffered;
            root.seedPending = result.seeded === false;
            root.seed();
        });
    }

    function seed() {
        if (!seedPending || DirectoryService.userDirs.length === 0)
            return;
        seedPending = false;
        _call("bookmarks.seed", {
            "paths": [RecentService.path].concat(DirectoryService.userDirs.map(dir => dir.path), [TrashService.path])
        });
    }

    function add(path, label) {
        if (path === "" || has(path))
            return;
        _call("bookmarks.add", {
            "path": path,
            "label": label || ""
        });
    }

    function remove(path) {
        _call("bookmarks.remove", {
            "path": path
        });
    }

    function toggle(path) {
        if (path === "")
            return;
        if (has(path)) {
            remove(path);
            return;
        }
        add(path, "");
    }

    function rename(path, label) {
        _call("bookmarks.rename", {
            "path": path,
            "label": label
        });
    }

    function reorder(orderedPaths) {
        _call("bookmarks.reorder", {
            "paths": orderedPaths
        });
    }

    function importGtk(callback) {
        declineImport();
        transport.call("bookmarks.importGtk", null, result => {
            if (result.error) {
                root.requestFailed(result.error);
                callback?.(0);
                return;
            }
            root.entries = result.bookmarks || [];
            root.gtkImportAvailable = false;
            callback?.(result.added || 0);
        });
    }

    function declineImport() {
        importOffer = false;
        SettingsData.gtkBookmarksOffered = true;
    }

    function _call(method, params) {
        transport.call(method, params, result => {
            if (result.error) {
                root.requestFailed(result.error);
                return;
            }
            root.entries = result.bookmarks || [];
            root.gtkImportAvailable = false;
        });
    }

    readonly property Connections _userDirs: Connections {
        target: DirectoryService
        function onUserDirsChanged() {
            root.seed();
        }
    }

    function _onEvent(data) {
        if (data.kind !== "changed")
            return;
        entries = data.bookmarks || [];
    }
}
