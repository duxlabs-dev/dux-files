pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    property var paths: []
    property bool cut: false
    property bool available: true

    readonly property bool hasFiles: paths.length > 0

    signal requestFailed(string message)

    function copy(selection) {
        _publish(selection, false);
    }

    function cutPaths(selection) {
        _publish(selection, true);
    }

    function read(callback) {
        transport.call("clipboard.paths", null, result => {
            if (result.error) {
                root.requestFailed(result.error);
                callback({
                    "paths": [],
                    "cut": false
                });
                return;
            }
            callback({
                "paths": result.paths || [],
                "cut": result.cut === true,
                "source": result.source || ""
            });
        });
    }

    function refresh() {
        if (paths.length === 0)
            return;
        transport.call("clipboard.paths", null, result => {
            const source = result.source || "";
            if (result.error || source === "" || source === "dfiles")
                return;
            root.clear();
        });
    }

    function clear() {
        paths = [];
        cut = false;
        transport.call("clipboard.clear", null, null);
    }

    function _publish(selection, asCut) {
        if (selection.length === 0)
            return;
        transport.call("clipboard.copy", {
            "paths": selection,
            "cut": asCut
        }, result => {
            if (result.error) {
                root.available = false;
                root.requestFailed(result.error);
                return;
            }
            root.available = true;
            root.paths = selection.slice();
            root.cut = asCut;
        });
    }
}
