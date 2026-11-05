pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    property var devices: []
    property bool available: false
    property string reason: ""
    property var usage: ({})

    readonly property var systemDevice: devices.find(device => device.system) ?? null

    signal requestFailed(string message, string code)

    readonly property DaemonTopic _devices: DaemonTopic {
        transport: root.transport
        topic: "devices"
        handler: data => root._onEvent(data)
        onConnected: root.refresh()
    }

    function refresh() {
        transport.call("devices.list", null, result => {
            if (result.error)
                return;
            root._onEvent(result);
        });
    }

    function refreshUsage(force) {
        const paths = [];
        for (const device of devices) {
            const mounts = device.mountPoints?.length ? device.mountPoints : device.mounted ? [device.mountPoint] : [];
            for (const path of mounts) {
                if (!paths.includes(path))
                    paths.push(path);
            }
        }
        if (!paths.includes("/"))
            paths.unshift("/");
        transport.call("devices.usage", {
            "paths": paths,
            "force": force === true
        }, result => {
            if (result.error)
                return;
            const next = ({});
            for (const entry of result.usage || [])
                next[entry.mountPoint] = entry;
            root.usage = next;
        });
    }

    function usageRatio(mountPoint) {
        const entry = usage[mountPoint];
        if (!entry || !entry.total)
            return -1;
        return entry.used / entry.total;
    }

    function mount(id, callback) {
        transport.callReporting("devices.mount", {
            "id": id
        }, root.requestFailed, callback);
    }

    function unmount(id, force, callback) {
        transport.callReporting("devices.unmount", {
            "id": id,
            "force": force === true
        }, root.requestFailed, callback);
    }

    function eject(id, callback) {
        transport.callReporting("devices.eject", {
            "id": id
        }, root.requestFailed, callback);
    }

    function powerOff(id, callback) {
        transport.callReporting("devices.powerOff", {
            "id": id
        }, root.requestFailed, callback);
    }

    function unlock(id, passphrase, callback) {
        transport.callReporting("devices.unlock", {
            "id": id,
            "passphrase": passphrase,
            "mount": true
        }, root.requestFailed, callback);
    }

    function lock(id, callback) {
        transport.callReporting("devices.lock", {
            "id": id
        }, root.requestFailed, callback);
    }

    function _onEvent(data) {
        available = data.available === true;
        reason = data.reason || "";
        devices = data.devices || [];
        refreshUsage(false);
    }
}
