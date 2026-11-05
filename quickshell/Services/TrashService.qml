pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Singleton {
    id: root

    readonly property string path: "trash:///"
    property string filesPath: localFilesPath()
    property int count: 0
    property string stamp: ""

    signal changed

    readonly property SnapshotSource _source: SnapshotSource {
        method: "trash.entries"
        affects: job => root.affects(job)
        onCheck: root._check()
        onReconnected: {
            root.stamp = "";
            root.resolvePath();
            root.refresh();
        }
        onAffected: root.refresh()
    }

    function localFilesPath() {
        const dataHome = String(Quickshell.env("XDG_DATA_HOME") || "").replace(/\/+$/, "");
        const base = dataHome.startsWith("/") ? dataHome : Paths.strip(Paths.home) + "/.local/share";
        return base + "/Trash/files";
    }

    function refresh() {
        _source.refresh();
    }

    function resolvePath() {
        DirectoryService.resolveLocation(root.path, "", result => {
            if (result.error)
                return;
            root.filesPath = result.location?.path ?? root.filesPath;
        });
    }

    function entries(callback) {
        _source.fetch({}, callback);
    }

    function holds(path) {
        return filesPath !== "" && (path === filesPath || FilePaths.parentOf(path) === filesPath);
    }

    function isTrashed(path) {
        const parent = FilePaths.parentOf(path);
        return parent === filesPath || /\/\.Trash(-\d+|\/\d+)\/files$/.test(parent);
    }

    function affects(job) {
        switch (job.kind) {
        case "trash":
        case "restore":
        case "emptyTrash":
            return true;
        case "delete":
        case "move":
            return (job.sources ?? []).some(path => isTrashed(path)) || holds(job.dest ?? "");
        default:
            return false;
        }
    }

    function _check() {
        if (!DuxFilesService.connected)
            return;
        DuxFilesService.call("trash.count", {}, result => {
            if (result.error)
                return;
            root.count = result.count ?? 0;
            if (result.stamp === root.stamp)
                return;
            root.stamp = result.stamp ?? "";
            root.changed();
        });
    }

    Component.onCompleted: {
        resolvePath();
        refresh();
    }
}
