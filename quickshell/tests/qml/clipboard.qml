pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var sent: []
    property var nextResult: ({})
    property string nextError: ""
    property var reads: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    readonly property QtObject transport: QtObject {
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
                callback({
                    "error": fixture.nextError
                });
                return;
            }
            callback({
                "result": fixture.nextResult
            });
        }
    }

    Component.onCompleted: {
        ClipboardService.transport = transport;

        ClipboardService.copy(["/home/me/a.txt", "/home/me/b.txt"]);
        check("copy goes to the daemon", sent[0].method, "clipboard.copy");
        check("copy sends the paths and the cut flag", sent[0].params, {
            "paths": ["/home/me/a.txt", "/home/me/b.txt"],
            "cut": false
        });
        check("a successful copy is remembered", [ClipboardService.paths.length, ClipboardService.cut, ClipboardService.hasFiles], [2, false, true]);

        ClipboardService.cutPaths(["/home/me/a.txt"]);
        check("cut sets the flag", sent[1].params.cut, true);
        check("cut is remembered", ClipboardService.cut, true);

        ClipboardService.copy([]);
        check("an empty selection sends nothing", sent.length, 2);

        nextResult = {
            "paths": ["/tmp/one", "/tmp/two"],
            "cut": true,
            "source": "dfiles"
        };
        ClipboardService.read(result => fixture.reads = fixture.reads.concat([result]));
        check("read asks the daemon", sent[2].method, "clipboard.paths");
        check("read returns what the daemon owns", reads[0], {
            "paths": ["/tmp/one", "/tmp/two"],
            "cut": true,
            "source": "dfiles"
        });

        nextResult = ({});
        ClipboardService.read(result => fixture.reads = fixture.reads.concat([result]));
        check("an empty clipboard reads as no paths", reads[1], {
            "paths": [],
            "cut": false,
            "source": ""
        });

        nextError = "no clipboard data";
        ClipboardService.read(result => fixture.reads = fixture.reads.concat([result]));
        check("a failed read still answers the caller", reads[2], {
            "paths": [],
            "cut": false
        });

        nextError = "compositor has no data-control";
        ClipboardService.copy(["/home/me/c.txt"]);
        check("a failed copy marks the clipboard unavailable", ClipboardService.available, false);
        check("a failed copy does not overwrite what was remembered", ClipboardService.paths, ["/home/me/a.txt"]);

        nextError = "";
        ClipboardService.clear();
        check("clear tells the daemon", sent[sent.length - 1].method, "clipboard.clear");
        check("clear empties the local copy", [ClipboardService.paths.length, ClipboardService.cut], [0, false]);

        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }
}
