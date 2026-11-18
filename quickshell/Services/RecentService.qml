pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Singleton {
    id: root

    readonly property string path: "recent:///"
    readonly property bool shown: SettingsData.recentFiles
    property string stamp: ""
    property int count: -1
    property int listeners: 0

    signal changed(var entries)
    signal requestFailed(string message, string code)

    readonly property SnapshotSource _source: SnapshotSource {
        method: "recent.entries"
        finishedOnly: true
        affects: job => root.affects(job)
        onCheck: root._check()
        onReconnected: root.invalidate()
        onAffected: root.invalidate()
        onFetched: result => {
            if (result.error)
                return;
            root.stamp = result.stamp ?? "";
            root.count = (result.entries || []).length;
        }
    }

    function refresh() {
        _source.refresh();
    }

    function invalidate() {
        stamp = "";
        count = -1;
        refresh();
    }

    function fetch(callback) {
        _source.fetch({}, callback);
    }

    function remove(paths) {
        if (paths.length === 0)
            return;
        DuxFilesService.call("recent.remove", {
            "paths": paths
        }, result => root._edited(result));
    }

    function clear() {
        DuxFilesService.call("recent.clear", {}, result => root._edited(result));
    }

    function affects(job) {
        switch (job.kind) {
        case "move":
        case "rename":
        case "newFolderWith":
        case "trash":
        case "delete":
        case "restore":
        case "emptyTrash":
            return true;
        default:
            return false;
        }
    }

    function _edited(result) {
        if (result.error) {
            requestFailed(result.error, result.code ?? "");
            return;
        }
        invalidate();
    }

    function _check() {
        if (!DuxFilesService.connected || listeners === 0)
            return;
        DuxFilesService.call("recent.entries", {
            "stamp": stamp
        }, result => {
            if (result.error || result.unchanged)
                return;
            root.stamp = result.stamp ?? "";
            root.count = (result.entries || []).length;
            root.changed(result.entries || []);
        });
    }
}
