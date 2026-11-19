pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services
import qs.DCommon.FileBrowser

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var pageResponse: ({})
    property var watchResponse: ({})
    property var unwatched: []
    property bool holdPages: false
    property var heldPage: null
    property var watchOptions: ({})
    property var thumbnailSizes: []

    property int addedSeen: 0

    Connections {
        target: model
        function onEntriesAdded() {
            fixture.addedSeen++;
        }
    }

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function entry(name, extra) {
        return Object.assign({
            "name": name,
            "path": "/tmp/" + name,
            "isDir": false,
            "isSymlink": false,
            "symlinkTarget": "",
            "symlinkBroken": false,
            "size": 1,
            "mtimeMs": 1000,
            "ctimeMs": 1000,
            "mode": "-rw-r--r--",
            "owner": "me",
            "group": "me",
            "hidden": false,
            "isExecutable": false,
            "extension": "txt",
            "mime": "text/plain",
            "iconName": "text-plain",
            "thumbnail": "",
            "thumbnailable": false,
            "unreadable": false
        }, extra || {});
    }

    function names() {
        const out = [];
        for (let i = 0; i < model.entries.count; i++)
            out.push(model.entries.get(i).name);
        return out;
    }

    readonly property QtObject fakeService: QtObject {
        property bool connected: true

        signal watchEvent(var data)

        function watch(path, options, callback) {
            fixture.watchOptions = options;
            callback(fixture.watchResponse);
        }

        function page(watchId, options, callback) {
            if (fixture.holdPages) {
                fixture.heldPage = () => callback(fixture.pageResponse);
                return;
            }
            callback(fixture.pageResponse);
        }

        function unwatch(watchId) {
            fixture.unwatched.push(watchId);
        }

        function thumbnails(paths, size, watchId, callback) {
            fixture.thumbnailSizes.push(size);
            callback({
                "results": paths.map(path => ({
                            "path": path,
                            "thumbnail": "/cache/" + path.split("/").pop() + ".png"
                        }))
            });
        }

        function icons(paths, callback) {
            callback({});
        }
    }

    readonly property DirectoryModel model: DirectoryModel {
        backend: fixture.fakeService
    }

    Component.onCompleted: {
        watchResponse = {
            "watchId": "w1",
            "topic": "files:w1",
            "watching": true,
            "pollOnFocus": false,
            "path": "/tmp",
            "total": 3,
            "cursor": "",
            "entries": [entry("a.txt"), entry("c.txt"), entry("d.txt")]
        };
        model.path = "/tmp";

        check("snapshot entries", names(), ["a.txt", "c.txt", "d.txt"]);
        check("snapshot total", model.total, 3);
        check("watch id", model.watchId, "w1");

        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "batch",
            "seq": 1,
            "added": [entry("b.txt")],
            "addedAt": [1]
        });
        check("insert at index", names(), ["a.txt", "b.txt", "c.txt", "d.txt"]);
        check("a watcher insert announces itself", fixture.addedSeen, 1);
        check("total after insert", model.total, 4);

        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "batch",
            "seq": 2,
            "removed": ["c.txt"],
            "changed": [entry("a.txt", {
                    "size": 42
                })]
        });
        check("remove by name", names(), ["a.txt", "b.txt", "d.txt"]);
        check("changed entry", model.entries.get(0).size, 42);
        check("total after remove", model.total, 3);

        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "thumbnails",
            "seq": 3,
            "thumbnails": [entry("b.txt", {
                    "thumbnail": "/cache/b.png"
                })]
        });
        check("thumbnail applied", model.entries.get(1).thumbnail, "/cache/b.png");

        fakeService.watchEvent({
            "watchId": "w2",
            "kind": "batch",
            "seq": 4,
            "removed": ["a.txt"]
        });
        check("other watch ignored", names(), ["a.txt", "b.txt", "d.txt"]);

        pageResponse = {
            "path": "/tmp",
            "total": 1,
            "cursor": "",
            "seq": 9,
            "entries": [entry("resynced.txt")]
        };
        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "batch",
            "seq": 9,
            "removed": ["b.txt"]
        });
        check("seq gap re-lists", names(), ["resynced.txt"]);

        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "resync",
            "seq": 10
        });
        check("resync re-lists", names(), ["resynced.txt"]);

        model.requestThumbnails(0, 0);
        check("thumbnail request skips non-thumbnailable", model.entries.get(0).thumbnail, "");

        pageResponse = {
            "total": 1,
            "cursor": "",
            "seq": 10,
            "entries": [entry("pic.png", {
                    "thumbnailable": true,
                    "mime": "image/png"
                })]
        };
        model.setSort("size", true);
        check("sort re-pages", names(), ["pic.png"]);
        model.requestThumbnails(0, 0);
        check("thumbnail request fills cache path", model.entries.get(0).thumbnail, "/cache/pic.png.png");

        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "batch",
            "seq": 11,
            "changed": [entry("pic.png", {
                    "thumbnailable": true,
                    "mime": "image/png"
                })]
        });
        check("late thumbnailable schedules a retry", model._thumbnailRetry.running, true);

        holdPages = true;
        model.setSort("name", false);
        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "batch",
            "seq": 12,
            "added": [entry("late.txt")],
            "addedAt": [1]
        });
        pageResponse = {
            "total": 1,
            "cursor": "",
            "seq": 11,
            "entries": [entry("pic.png")]
        };
        holdPages = false;
        heldPage();
        check("event during a page request applies after it", names(), ["pic.png", "late.txt"]);

        let goneSeen = false;
        model.gone.connect(() => goneSeen = true);
        fakeService.watchEvent({
            "watchId": "w1",
            "kind": "gone",
            "seq": 12
        });
        check("gone signal", goneSeen, true);
        check("watch released", model.watchId, "");
        check("unwatched ids", unwatched, ["w1"]);

        watchResponse = {
            "watchId": "w3",
            "topic": "files:w3",
            "watching": true,
            "pollOnFocus": false,
            "path": "/tmp/filtered",
            "total": 4,
            "cursor": "",
            "entries": [entry("Alpha.txt"), entry("beta.txt"), entry("gamma.txt"), entry("delta.txt")]
        };
        model.path = "/tmp/filtered";
        model.nameFilter = "ta";
        check("filter keeps case-insensitive substring matches", names(), ["beta.txt", "delta.txt"]);
        check("filter leaves the daemon total alone", model.total, 4);
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "batch",
            "seq": 1,
            "added": [entry("Tango.txt"), entry("zeta.txt")],
            "addedAt": [4, 5]
        });
        check("inserts land at the filtered index", names(), ["beta.txt", "delta.txt", "Tango.txt", "zeta.txt"]);
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "batch",
            "seq": 2,
            "added": [entry("aa-ta.txt")],
            "addedAt": [1]
        });
        check("an insert before hidden entries counts only visible ones", names(), ["aa-ta.txt", "beta.txt", "delta.txt", "Tango.txt", "zeta.txt"]);
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "batch",
            "seq": 3,
            "removed": ["gamma.txt", "delta.txt"]
        });
        check("removals hit hidden and visible entries", [names(), model.total], [["aa-ta.txt", "beta.txt", "Tango.txt", "zeta.txt"], 5]);
        model.nameFilter = "";
        check("clearing the filter restores the daemon order", names(), ["Alpha.txt", "aa-ta.txt", "beta.txt", "Tango.txt", "zeta.txt"]);

        model.thumbnailsEnabled = false;
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "batch",
            "seq": 4,
            "added": [entry("shot.png", {
                    "thumbnailable": true,
                    "mime": "image/png"
                })],
            "addedAt": [0]
        });
        model.requestThumbnails(0, 0);
        check("thumbnails off skips the request", model.entries.get(0).thumbnail, "");
        model.thumbnailsEnabled = true;
        model.thumbnailSizeLimit = 0;
        model.requestThumbnails(0, 0);
        check("size cap skips large images", model.entries.get(0).thumbnail, "");
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "batch",
            "seq": 5,
            "added": [entry("clip.mp4", {
                    "thumbnailable": true,
                    "mime": "video/mp4",
                    "size": 900000000
                })],
            "addedAt": [1]
        });
        model.requestThumbnails(0, 1);
        check("size cap leaves videos alone", [model.entries.get(0).thumbnail, model.entries.get(1).thumbnail], ["", "/cache/clip.mp4.png"]);
        model.thumbnailSizeLimit = -1;
        model.requestThumbnails(0, 0);
        check("thumbnails resume", model.entries.get(0).thumbnail, "/cache/shot.png.png");

        model.thumbnailSize = "large";
        thumbnailSizes = [];
        fakeService.watchEvent({
            "watchId": "w3",
            "kind": "thumbnails",
            "seq": 6,
            "thumbnails": [entry("shot.png", {
                    "thumbnailable": true,
                    "mime": "image/png",
                    "thumbnail": "/cache/thumbnails/normal/shot.png"
                }), entry("clip.mp4", {
                    "thumbnailable": true,
                    "mime": "video/mp4",
                    "thumbnail": "/cache/thumbnails/x-large/clip.png"
                })]
        });
        model.requestThumbnails(0, 1);
        check("a thumbnail smaller than the tile is replaced, a larger one is kept", [model.entries.get(0).thumbnail, model.entries.get(1).thumbnail, thumbnailSizes], ["/cache/shot.png.png", "/cache/thumbnails/x-large/clip.png", ["large"]]);
        model.path = "/tmp/zoomed";
        check("a new listing asks for the tile's thumbnail size", watchOptions.thumbnailSize, "large");

        watchResponse = {
            "error": "no such directory",
            "code": "ENOENT"
        };
        model.path = "/tmp/missing";
        check("error code", model.errorCode, "ENOENT");
        check("error clears entries", names(), []);

        log.info(failures === 0 ? "FIXTURE PASS" : `FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
