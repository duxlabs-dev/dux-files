pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var response: ({})
    property int requests: 0

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: false

        function call(method, params, callback) {
            fixture.requests++;
            fixture.check("method " + fixture.requests, method, "system.capabilities");
            callback(DuxFilesService.unwrap(fixture.response));
        }
    }

    Component.onCompleted: {
        SandboxService.transport = fakeTransport;

        response = {
            "error": "not connected"
        };
        SandboxService.refresh();
        check("unknown feature is available before an answer", SandboxService.available("terminal"), true);
        check("no reason before an answer", SandboxService.reason("terminal"), "");
        check("not loaded on error", SandboxService.loaded, false);

        response = {
            "result": {
                "sandboxed": true,
                "appId": "dev.duxlabs.files",
                "hostFilesystem": false,
                "hostOsRoot": "/run/host",
                "portals": {
                    "Trash": 1
                },
                "features": {
                    "terminal": {
                        "available": false,
                        "reason": "NOHOSTSPAWN"
                    },
                    "openWith": {
                        "available": true,
                        "degraded": true,
                        "reason": "PORTALCHOOSER"
                    },
                    "trash": {
                        "available": true
                    }
                },
                "thumbnailers": ["video"]
            }
        };
        fakeTransport.connected = true;
        check("connect refetches", requests, 2);
        check("loaded", SandboxService.loaded, true);
        check("missing feature", SandboxService.available("terminal"), false);
        check("missing reason", SandboxService.reason("terminal"), "NOHOSTSPAWN");
        check("degraded feature stays available", SandboxService.available("openWith"), true);
        check("degraded flag", SandboxService.degraded("openWith"), true);
        check("plain feature", SandboxService.degraded("trash"), false);
        check("unlisted feature defaults to available", SandboxService.available("devices"), true);

        response = {
            "error": "not connected"
        };
        fakeTransport.connected = false;
        check("disconnect does not refetch", requests, 2);
        fakeTransport.connected = true;
        check("reconnect refetches", requests, 3);
        check("failed refetch keeps the last table", SandboxService.available("terminal"), false);
        check("failed refetch keeps loaded", SandboxService.loaded, true);

        if (failures === 0)
            log.info("FIXTURE PASS");
        else
            log.error(`FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
