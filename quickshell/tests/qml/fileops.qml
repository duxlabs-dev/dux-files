pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var sent: []
    property var handler: null
    property var jobsResponse: ({})
    property var elevationResponse: ({})
    property var nextError: ""
    property var finished: []
    property var requestErrors: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function job(id, extra) {
        return Object.assign({
            "id": id,
            "kind": "copy",
            "state": "running",
            "scope": "w1",
            "sources": ["/tmp/a.txt"],
            "dest": "/tmp/dest",
            "progress": {
                "filesDone": 0,
                "filesTotal": 2,
                "bytesDone": 0,
                "bytesTotal": 10,
                "currentPath": "",
                "bytesPerSec": 0,
                "estimating": false
            },
            "skipped": 0,
            "warnings": 0,
            "cancelled": false,
            "undoable": true,
            "startedMs": 1000
        }, extra || {});
    }

    property int subscriptions: 0

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: true

        function subscribeTopic(topic, cb) {
            if (topic !== "jobs")
                return;
            fixture.subscriptions++;
            fixture.handler = cb;
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
                    "error": message
                });
                return;
            }
            if (method === "files.op.jobs") {
                callback({
                    "result": fixture.jobsResponse
                });
                return;
            }
            if (method === "system.elevation") {
                callback({
                    "result": fixture.elevationResponse
                });
                return;
            }
            if (method === "files.op.pasteImage") {
                callback({
                    "result": {
                        "image": false
                    }
                });
                return;
            }
            callback({
                "result": {
                    "job": fixture.job("j1")
                }
            });
        }
    }

    readonly property Connections opsLink: Connections {
        target: FileOpsService

        function onJobFinished(job) {
            fixture.finished = fixture.finished.concat([job.id]);
        }

        function onRequestFailed(message) {
            fixture.requestErrors = fixture.requestErrors.concat([message]);
        }
    }

    function states() {
        return FileOpsService.jobs.map(job => `${job.id}:${job.state}`);
    }

    Component.onCompleted: {
        FileOpsService.transport = fakeTransport;

        check("subscribed once", subscriptions, 1);

        FileOpsService.copy(["/tmp/a.txt"], "/tmp/dest", "w1");
        check("copy request", sent[sent.length - 1].method, "files.op.copy");
        check("copy params", sent[sent.length - 1].params, {
            "paths": ["/tmp/a.txt"],
            "dest": "/tmp/dest",
            "scope": "w1"
        });
        check("job stored from the response", states(), ["j1:running"]);

        handler({
            "kind": "progress",
            "jobId": "j1",
            "progress": {
                "filesDone": 1,
                "filesTotal": 2,
                "bytesDone": 5,
                "bytesTotal": 10,
                "currentPath": "/tmp/dest/a.txt",
                "bytesPerSec": 5,
                "estimating": false
            }
        });
        check("progress applied", FileOpsService.jobs[0].progress.bytesDone, 5);
        check("no finish yet", finished, []);

        handler({
            "kind": "job",
            "job": job("j1", {
                "state": "paused-conflict",
                "conflict": {
                    "id": "c1",
                    "actions": ["skip", "replace", "keepBoth", "cancel"]
                }
            })
        });
        check("paused state", states(), ["j1:paused-conflict"]);
        check("conflict carried on the job", FileOpsService.jobs[0].conflict.id, "c1");
        check("active count", FileOpsService.activeCount, 1);

        FileOpsService.respond("j1", "c1", "keepBoth", true);
        check("respond params", sent[sent.length - 1].params, {
            "jobId": "j1",
            "id": "c1",
            "action": "keepBoth",
            "applyToAll": true
        });

        handler({
            "kind": "job",
            "job": job("j1", {
                "state": "done"
            })
        });
        check("finished signal", finished, ["j1"]);
        check("no conflict left", FileOpsService.jobs[0].conflict, undefined);
        check("idle", FileOpsService.activeCount, 0);

        handler({
            "kind": "job",
            "job": job("j2", {
                "state": "running",
                "startedMs": 2000
            })
        });
        check("ordered by start", states(), ["j1:done", "j2:running"]);

        nextError = "not connected to dfiles socket";
        FileOpsService.trash(["/tmp/a.txt"], "w1");
        check("request error surfaced", requestErrors, ["not connected to dfiles socket"]);

        nextError = "unknown job: j404";
        FileOpsService.cancel("j404");
        check("cancel error surfaced", requestErrors[requestErrors.length - 1], "unknown job: j404");

        nextError = "nothing to undo";
        FileOpsService.undo("w1");
        check("undo error surfaced", requestErrors[requestErrors.length - 1], "nothing to undo");

        jobsResponse = {
            "jobs": [job("j9", {
                    "state": "queued",
                    "startedMs": 0
                })]
        };
        FileOpsService.refresh();
        check("relist replaces the store", states(), ["j9:queued"]);

        check("no elevation before asking", FileOpsService.canElevate, false);
        elevationResponse = {
            "available": false,
            "route": "pkexec",
            "reason": "NOPOLICY"
        };
        FileOpsService.refreshElevation();
        check("elevation reason kept", FileOpsService.elevation.reason, "NOPOLICY");
        check("elevation unavailable", FileOpsService.canElevate, false);
        elevationResponse = {
            "available": true,
            "route": "pkexec"
        };
        FileOpsService.refreshElevation();
        check("elevation available", FileOpsService.canElevate, true);
        check("elevation reason cleared", FileOpsService.elevation.reason, "");

        FileOpsService.respond("j9", "e1", "retryElevated", false);
        check("retry elevated respond params", sent[sent.length - 1].params, {
            "jobId": "j9",
            "id": "e1",
            "action": "retryElevated",
            "applyToAll": false
        });

        FileOpsService.setOwner(["/tmp/a.txt"], "root", "", true);
        check("set owner request", sent[sent.length - 1].method, "files.op.setOwner");
        check("set owner params", sent[sent.length - 1].params, {
            "paths": ["/tmp/a.txt"],
            "owner": "root",
            "group": "",
            "recursive": true
        });

        FileOpsService.newFolderWith(["/tmp/a.txt", "/tmp/b.txt"], "/tmp", "Untitled folder", "w1", true);
        check("new folder with selection params", sent[sent.length - 1].params, {
            "paths": ["/tmp/a.txt", "/tmp/b.txt"],
            "path": "/tmp",
            "name": "Untitled folder",
            "unique": true,
            "scope": "w1"
        });

        const before = states();
        FileOpsService.pasteImage("/tmp", "Pasted image", "w1");
        check("paste image request", sent[sent.length - 1].method, "files.op.pasteImage");
        check("no image stores no job", states(), before);
        check("no image is not an error", requestErrors.length, 3);

        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }
}
