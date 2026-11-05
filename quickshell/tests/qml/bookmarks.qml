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
    property var handler: null
    property var listResponse: ({})
    property string nextError: ""
    property var errors: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function last() {
        return sent[sent.length - 1];
    }

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: true

        function subscribeTopic(topic, cb) {
            if (topic === "bookmarks")
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
            switch (method) {
            case "bookmarks.list":
                callback({
                    "result": fixture.listResponse
                });
                return;
            case "bookmarks.importGtk":
                callback({
                    "result": {
                        "bookmarks": [
                            {
                                "path": "/a",
                                "label": "a",
                                "icon": ""
                            }
                        ],
                        "added": 1
                    }
                });
                return;
            default:
                callback({
                    "result": {
                        "bookmarks": (params.paths ?? [params.path]).map(path => ({
                                    "path": path,
                                    "label": params.label || path.split("/").pop(),
                                    "icon": ""
                                }))
                    }
                });
            }
        }
    }

    Component.onCompleted: {
        listResponse = {
            "bookmarks": [
                {
                    "path": "/srv/media",
                    "label": "Media",
                    "icon": ""
                }
            ],
            "gtkImportAvailable": true
        };
        BookmarksService.requestFailed.connect(message => fixture.errors.push(message));
        BookmarksService.transport = fakeTransport;
        check("list request on attach", last().method, "bookmarks.list");
        check("entries from the list", BookmarksService.paths, ["/srv/media"]);
        check("label lookup", BookmarksService.labelFor("/srv/media"), "Media");
        check("has", [BookmarksService.has("/srv/media"), BookmarksService.has("/nope")], [true, false]);
        check("gtk import is offered once", BookmarksService.importOffer, true);

        BookmarksService.add("/srv/media", "");
        check("adding a known path sends nothing", last().method, "bookmarks.list");

        BookmarksService.toggle("/home/me/Work");
        check("toggle adds", last(), {
            "method": "bookmarks.add",
            "params": {
                "path": "/home/me/Work",
                "label": ""
            }
        });
        check("response replaces the list", BookmarksService.paths, ["/home/me/Work"]);

        BookmarksService.toggle("/home/me/Work");
        check("toggle removes", last().method, "bookmarks.remove");

        BookmarksService.rename("/home/me/Work", "Work stuff");
        check("rename params", last().params, {
            "path": "/home/me/Work",
            "label": "Work stuff"
        });

        BookmarksService.reorder(["/b", "/a"]);
        check("reorder params", last().params.paths, ["/b", "/a"]);

        fixture.handler({
            "kind": "changed",
            "bookmarks": [
                {
                    "path": "/x",
                    "label": "X",
                    "icon": "folder-code"
                }
            ]
        });
        check("event replaces the list", BookmarksService.entries, [
            {
                "path": "/x",
                "label": "X",
                "icon": "folder-code"
            }
        ]);

        nextError = "disk full";
        BookmarksService.remove("/x");
        check("errors surface through requestFailed", errors, ["disk full"]);
        check("a failed call keeps the list", BookmarksService.paths, ["/x"]);

        let added = -1;
        BookmarksService.importGtk(count => added = count);
        check("import marks the offer as handled", SettingsData.gtkBookmarksOffered, true);
        check("import reports the count", added, 1);
        check("import replaces the list", BookmarksService.paths, ["/a"]);
        check("no second offer", BookmarksService.importOffer, false);

        listResponse = {
            "bookmarks": [],
            "seeded": false
        };
        DirectoryService.userDirs = [];
        BookmarksService.refresh();
        check("an unseeded list waits for the user dirs", last().method, "bookmarks.list");
        DirectoryService.userDirs = [
            {
                "key": "home",
                "path": "/home/me"
            },
            {
                "key": "download",
                "path": "/home/me/Downloads"
            }
        ];
        check("the user dirs seed the places once", last(), {
            "method": "bookmarks.seed",
            "params": {
                "paths": ["recent:///", "/home/me", "/home/me/Downloads", "trash:///"]
            }
        });
        check("the seed response replaces the list", BookmarksService.paths, ["recent:///", "/home/me", "/home/me/Downloads", "trash:///"]);
        listResponse = {
            "bookmarks": [],
            "seeded": true
        };
        BookmarksService.refresh();
        DirectoryService.userDirs = [];
        DirectoryService.userDirs = [
            {
                "key": "home",
                "path": "/home/me"
            }
        ];
        check("a seeded list is never seeded again", last().method, "bookmarks.list");

        log.info(failures === 0 ? "FIXTURE PASS" : `FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
