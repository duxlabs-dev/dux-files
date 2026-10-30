pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var sent: []
    property var handlers: ({})
    property int subscriptions: 0
    property var nextError: null
    property var responses: ({})
    property var asks: []
    property var failedRequests: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function lastRequest() {
        return sent[sent.length - 1];
    }

    function device(id, extra) {
        return Object.assign({
            "id": id,
            "kind": "removable",
            "name": id,
            "device": "/dev/sdb1",
            "drive": "/drives/stick",
            "size": 1000,
            "removable": true,
            "ejectable": true,
            "canPowerOff": true,
            "encrypted": false,
            "unlocked": false,
            "mounted": false,
            "iconName": "drive-removable-media",
            "system": false
        }, extra || {});
    }

    function mount(id, kind, extra) {
        return Object.assign({
            "id": id,
            "name": id,
            "uri": "ftp://" + id + "/",
            "path": "/run/user/1000/gvfs/" + id,
            "scheme": "ftp",
            "kind": kind,
            "iconName": "folder-remote"
        }, extra || {});
    }

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: true

        function subscribeTopic(topic, cb) {
            fixture.subscriptions++;
            const list = fixture.handlers[topic] || [];
            list.push(cb);
            fixture.handlers[topic] = list;
        }

        function unsubscribeTopic(topic, cb) {
        }

        function call(method, params, callback) {
            sendRequest(method, params, response => callback?.(DuxFilesService.unwrap(response)));
        }

        function sendRequest(method, params, callback) {
            fixture.sent = fixture.sent.concat([
                {
                    "method": method,
                    "params": params
                }
            ]);
            if (!callback)
                return;
            if (fixture.nextError) {
                const error = fixture.nextError;
                fixture.nextError = null;
                callback(error);
                return;
            }
            callback({
                "result": fixture.responses[method] || {}
            });
        }
    }

    function emit(topic, data) {
        for (const handler of (handlers[topic] || []).slice())
            handler(data);
    }

    readonly property Connections devicesLink: Connections {
        target: DevicesService
        function onRequestFailed(message, code) {
            fixture.failedRequests = fixture.failedRequests.concat([`${code}:${message}`]);
        }
    }

    readonly property Connections networkLink: Connections {
        target: NetworkService
        function onAskRequested(ask) {
            fixture.asks = fixture.asks.concat([ask.id]);
        }
    }

    Component.onCompleted: {
        DevicesService.transport = fakeTransport;
        NetworkService.transport = fakeTransport;
        check("one subscription per service", subscriptions, 2);
        check("a transport already connected loads at once", sent.map(request => request.method), ["devices.list", "devices.usage", "network.mounts", "network.recent"]);
        sent = [];

        responses["devices.list"] = {
            "available": true,
            "devices": [device("/block/root", {
                    "system": true,
                    "kind": "system",
                    "mounted": true,
                    "mountPoint": "/",
                    "removable": false,
                    "ejectable": false
                }), device("/block/stick")]
        };
        responses["devices.usage"] = {
            "usage": [
                {
                    "mountPoint": "/",
                    "total": 200,
                    "used": 50,
                    "free": 150
                }
            ]
        };
        DevicesService.refresh();
        check("list then usage", sent.map(request => request.method), ["devices.list", "devices.usage"]);
        check("devices stored", DevicesService.devices.length, 2);
        check("system device found", DevicesService.systemDevice.id, "/block/root");
        check("usage ratio", DevicesService.usageRatio("/"), 0.25);
        check("usage ratio for an unknown mount", DevicesService.usageRatio("/nope"), -1);
        DevicesService.devices = [device("/block/root", {
                "mounted": true,
                "mountPoint": "/",
                "mountPoints": ["/", "/home"]
            })];
        DevicesService.refreshUsage(false);
        check("usage includes all mounts", lastRequest().params.paths, ["/", "/home"]);

        emit("devices", {
            "available": false,
            "reason": "udisks2 is not running",
            "devices": []
        });
        check("event replaces the snapshot", [DevicesService.available, DevicesService.reason, DevicesService.devices.length], [false, "udisks2 is not running", 0]);

        DevicesService.mount("/block/stick", null);
        check("mount request", lastRequest(), {
            "method": "devices.mount",
            "params": {
                "id": "/block/stick"
            }
        });

        DevicesService.unmount("/block/stick", true, null);
        check("unmount carries force", lastRequest().params, {
            "id": "/block/stick",
            "force": true
        });

        DevicesService.unlock("/block/luks", "hunter2", null);
        check("unlock mounts after unlocking", lastRequest().params, {
            "id": "/block/luks",
            "passphrase": "hunter2",
            "mount": true
        });

        nextError = {
            "error": "target is busy",
            "code": "EBUSY"
        };
        DevicesService.eject("/block/stick", null);
        check("device errors are reported", failedRequests, ["EBUSY:target is busy"]);

        DevicesService.refreshUsage(true);
        check("forced usage refresh", lastRequest().params, {
            "paths": ["/"],
            "force": true
        });

        responses["network.mounts"] = {
            "available": true,
            "schemes": ["sftp", "smb"],
            "mounts": [mount("share", "network"), mount("phone", "device")],
            "portables": [
                {
                    "id": "mtp-1",
                    "monitor": "org.gtk.vfs.MTPVolumeMonitor",
                    "name": "Pixel",
                    "iconName": "multimedia-player",
                    "kind": "device",
                    "mounted": false
                }
            ]
        };
        responses["network.recent"] = {
            "servers": [
                {
                    "uri": "sftp://one/"
                }
            ]
        };
        sent = [];
        NetworkService.refresh();
        check("mounts split by kind", [NetworkService.networkMounts.length, NetworkService.deviceMounts.length], [1, 1]);
        check("portables stored", NetworkService.portables.length, 1);
        check("installed schemes stored", NetworkService.schemes, ["sftp", "smb"]);
        check("recent servers stored", NetworkService.servers.length, 1);

        emit("network", {
            "kind": "ask",
            "id": "/mountop/1/1",
            "type": "password",
            "message": "Enter password",
            "needPassword": true
        });
        check("ask surfaced", asks, ["/mountop/1/1"]);

        NetworkService.answer("/mountop/1/1", {
            "password": "secret",
            "save": 2
        });
        check("answer request", lastRequest(), {
            "method": "network.answer",
            "params": {
                "id": "/mountop/1/1",
                "password": "secret",
                "save": 2
            }
        });

        NetworkService.abort("/mountop/1/1");
        check("abort request", lastRequest().params, {
            "id": "/mountop/1/1",
            "abort": true
        });

        responses["network.connect"] = {
            "mount": mount("share", "network")
        };
        NetworkService.connectTo("ftp://share/", null);
        check("connect clears the busy flag", NetworkService.connecting, false);
        check("connect refreshes recent servers", lastRequest().method, "network.recent");

        responses["network.connect"] = {
            "server": "smb://nas/",
            "shares": [
                {
                    "name": "media",
                    "uri": "smb://nas/media"
                }
            ]
        };
        let browsed = null;
        NetworkService.connectTo("smb://nas/", result => browsed = result.shares);
        check("browsing a server hands back its shares", browsed, [
            {
                "name": "media",
                "uri": "smb://nas/media"
            }
        ]);

        emit("network", {
            "kind": "mounts",
            "available": true,
            "mounts": []
        });
        check("mount events replace the list", NetworkService.mounts.length, 0);
        check("portables survive a mounts event", NetworkService.portables.length, 1);

        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }
}
