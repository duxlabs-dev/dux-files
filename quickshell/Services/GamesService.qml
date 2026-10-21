pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    property var groups: []

    signal requestFailed(string message, string code)

    readonly property DaemonTopic _devices: DaemonTopic {
        transport: root.transport
        topic: "devices"
        handler: () => root.refresh()
        onConnected: root.refresh()
    }

    function refresh() {
        transport.call("games.list", null, result => {
            if (result.error) {
                root.requestFailed(result.error, result.code ?? "");
                return;
            }
            root.groups = result.groups || [];
        });
    }
}
