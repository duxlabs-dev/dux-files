pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var sent: []
    property var nextResult: ({})
    property var nextError: ""
    property var replies: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function lastSent() {
        return sent[sent.length - 1];
    }

    readonly property QtObject fakeTransport: QtObject {
        readonly property bool connected: true

        function subscribeTopic(topic, cb) {
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
            if (fixture.nextError !== "") {
                const message = fixture.nextError;
                fixture.nextError = "";
                callback({
                    "error": message,
                    "code": "EACCES"
                });
                return;
            }
            callback({
                "result": fixture.nextResult
            });
        }
    }

    Component.onCompleted: {
        AppsService.transport = fakeTransport;

        AppsService.open(["/tmp/a.txt"], null);
        check("open request", lastSent().method, "files.open");
        check("open params", lastSent().params, {
            "paths": ["/tmp/a.txt"]
        });

        SettingsData.opener = "xdg-open";
        AppsService.open(["/tmp/a.txt"], null);
        check("opener preference is passed through", lastSent().params, {
            "paths": ["/tmp/a.txt"],
            "opener": "xdg-open"
        });
        AppsService.openWith(["/tmp/a.txt"], "editor.desktop", null);
        check("a chosen app bypasses the opener", lastSent().params, {
            "paths": ["/tmp/a.txt"],
            "desktopId": "editor.desktop"
        });
        SettingsData.opener = "";

        SettingsData.terminal = "ghostty";
        AppsService.openTerminal("/tmp", null);
        check("terminal preference is passed through", lastSent().params, {
            "path": "/tmp",
            "terminal": "ghostty"
        });
        SettingsData.terminal = "";

        AppsService.openWith(["/tmp/a.txt"], "editor.desktop", null);
        check("openWith params", lastSent().params, {
            "paths": ["/tmp/a.txt"],
            "desktopId": "editor.desktop"
        });

        AppsService.appsForMime("text/plain", "/tmp/a.txt", null);
        check("forMime params", lastSent().params, {
            "mime": "text/plain",
            "path": "/tmp/a.txt"
        });

        AppsService.sections(["text/plain"], ["/tmp/a.txt"], null);
        check("sections prefers given types over paths", lastSent().params, {
            "mimes": ["text/plain"]
        });
        AppsService.sections([], ["/tmp/a.txt"], null);
        check("sections types the paths when no type is given", lastSent().params, {
            "paths": ["/tmp/a.txt"]
        });

        AppsService.setDefault("text/plain", "editor.desktop", null);
        check("setDefault params", lastSent().params, {
            "mime": "text/plain",
            "desktopId": "editor.desktop"
        });

        nextResult = {
            "apps": [
                {
                    "id": "editor.desktop",
                    "name": "Editor"
                }
            ]
        };
        AppsService.allApps(response => fixture.replies = fixture.replies.concat([response]));
        check("result is unwrapped", replies[0].apps[0].id, "editor.desktop");

        nextError = "denied";
        AppsService.run("/tmp/a.sh", response => fixture.replies = fixture.replies.concat([response]));
        check("error carries its code", replies[1], {
            "error": "denied",
            "code": "EACCES"
        });

        check("a program asks before running", FilesFormat.isRunnable("application/x-executable", true), true);
        check("a script asks before running", FilesFormat.isRunnable("application/x-shellscript", true), true);
        check("an unknown executable asks", FilesFormat.isRunnable("", true), true);
        check("a text file with the bit asks", FilesFormat.isRunnable("text/x-python", true), true);
        check("an image never asks", FilesFormat.isRunnable("image/png", true), false);
        check("without the bit nothing asks", FilesFormat.isRunnable("application/x-executable", false), false);

        log.info(failures === 0 ? "FIXTURE PASS" : `FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
