pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    property var mounts: []
    property var portables: []
    property var servers: []
    property var schemes: []
    property bool available: false
    property string reason: ""
    property bool connecting: false

    readonly property var networkMounts: mounts.filter(mount => mount.kind === "network")
    readonly property var deviceMounts: mounts.filter(mount => mount.kind === "device")

    signal askRequested(var ask)
    signal requestFailed(string message, string code)
    signal connected(var mount)

    readonly property DaemonTopic _network: DaemonTopic {
        transport: root.transport
        topic: "network"
        handler: data => root._onEvent(data)
        onConnected: root.refresh()
    }

    function refresh() {
        transport.call("network.mounts", null, result => {
            if (result.error)
                return;
            root._applyMounts(result);
        });
        refreshRecent();
    }

    function refreshRecent() {
        transport.call("network.recent", null, result => {
            if (result.error)
                return;
            root.servers = result.servers || [];
        });
    }

    // Blocks in the daemon until gvfs is done, credential round trips included.
    function connectTo(uri, callback, password) {
        connecting = true;
        const params = {
            "uri": uri
        };
        if (password)
            params.password = password;
        transport.callReporting("network.connect", params, root.requestFailed, result => {
            root.connecting = false;
            if (!result.error && result.mount) {
                if (!root.mounts.some(mount => mount.id === result.mount.id))
                    root.mounts = root.mounts.concat([result.mount]);
                root.connected(result.mount);
                root.refreshRecent();
            }
            if (callback)
                callback(result);
        });
    }

    function answer(id, params) {
        const request = Object.assign({
            "id": id
        }, params || {});
        transport.callReporting("network.answer", request, root.requestFailed, null);
    }

    function abort(id) {
        answer(id, {
            "abort": true
        });
    }

    function disconnect(id, callback) {
        transport.callReporting("network.disconnect", {
            "id": id
        }, root.requestFailed, callback);
    }

    function forget(uri) {
        transport.call("network.forget", {
            "uri": uri
        }, result => {
            if (result.error)
                return;
            root.servers = result.servers || [];
        });
    }

    function mountPortable(monitor, id, callback) {
        transport.callReporting("network.mountPortable", {
            "monitor": monitor,
            "id": id
        }, root.requestFailed, callback);
    }

    function _onEvent(data) {
        switch (data.kind) {
        case "ask":
            askRequested(data);
            return;
        case "mounts":
            _applyMounts(data);
            return;
        }
    }

    function _applyMounts(data) {
        available = data.available === true;
        reason = data.reason || "";
        schemes = data.schemes || [];
        mounts = data.mounts || [];
        if (data.portables !== undefined)
            portables = data.portables || [];
    }
}
