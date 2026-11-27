pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services
import qs.DCommon.FileBrowser

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var started: []
    property var cancelled: []
    property var startResponse: ({})
    property int nextId: 0

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function entry(name) {
        return {
            "name": name,
            "path": "/tmp/" + name,
            "isDir": false,
            "size": 1,
            "mtimeMs": 1000,
            "mime": "text/plain",
            "thumbnailable": name.endsWith(".png"),
            "thumbnail": ""
        };
    }

    function names() {
        const out = [];
        for (let i = 0; i < model.entries.count; i++)
            out.push(model.entries.get(i).name);
        return out;
    }

    readonly property QtObject fakeBackend: QtObject {
        property bool connected: true

        signal searchEvent(var data)

        function search(root, query, options, callback) {
            fixture.nextId++;
            fixture.started = fixture.started.concat([
                {
                    "root": root,
                    "query": query,
                    "options": options
                }
            ]);
            callback(Object.assign({
                "searchId": "s" + fixture.nextId,
                "topic": "search:s" + fixture.nextId,
                "limit": 10000
            }, fixture.startResponse));
        }

        function cancelSearch(searchId) {
            fixture.cancelled = fixture.cancelled.concat([searchId]);
        }

        function thumbnails(paths, size, watchId, callback) {
            callback({
                "results": paths.map(path => ({
                            "path": path,
                            "thumbnail": "/cache/" + path.split("/").pop()
                        }))
            });
        }
    }

    readonly property SearchModel model: SearchModel {
        backend: fixture.fakeBackend
    }

    Component.onCompleted: {
        let finished = 0;
        model.finished.connect(() => finished++);

        model.rootPath = "/tmp";
        model.query = "rep";
        model.options = {
            "types": ["documents"]
        };
        model.start();
        check("start sends root, query and options", started[0], {
            "root": "/tmp",
            "query": "rep",
            "options": {
                "types": ["documents"]
            }
        });
        check("active and loading", [model.active, model.loading, model.searchId], [true, true, "s1"]);

        fakeBackend.searchEvent({
            "searchId": "s1",
            "kind": "results",
            "entries": [entry("report.txt"), entry("Reports")],
            "count": 2
        });
        check("results append", names(), ["report.txt", "Reports"]);
        check("rows carry the match roles from the first one on", [model.entries.get(0).line, model.entries.get(0).snippet], [0, ""]);
        check("count follows the daemon", model.total, 2);

        fakeBackend.searchEvent({
            "searchId": "s9",
            "kind": "results",
            "entries": [entry("other.txt")],
            "count": 1
        });
        check("another search's events are ignored", names(), ["report.txt", "Reports"]);

        fakeBackend.searchEvent({
            "searchId": "s1",
            "kind": "results",
            "entries": [entry("pic.png")],
            "count": 3
        });
        model.requestThumbnails(0, 2);
        check("thumbnails only for thumbnailable entries", [model.entries.get(0).thumbnail, model.entries.get(2).thumbnail], ["", "/cache/pic.png"]);

        fakeBackend.searchEvent({
            "searchId": "s1",
            "kind": "done",
            "count": 3,
            "truncated": true,
            "unreadable": 2
        });
        check("done ends loading", [model.loading, model.truncated, model.unreadable, finished], [false, true, 2, 1]);
        check("done keeps the results", names().length, 3);
        check("a finished search is not cancelled", cancelled, []);

        model.start();
        check("restart clears the list", names(), []);
        fakeBackend.searchEvent({
            "searchId": "s2",
            "kind": "results",
            "entries": [entry("a")],
            "count": 1
        });
        model.cancel();
        check("cancel tells the backend", cancelled, ["s2"]);
        check("cancel keeps what arrived", names(), ["a"]);
        check("cancel ends loading but stays active", [model.loading, model.active], [false, true]);

        fakeBackend.searchEvent({
            "searchId": "s2",
            "kind": "done",
            "count": 1
        });
        check("late events after cancel are dropped", finished, 1);

        model.clear();
        check("clear leaves results mode", [model.active, names()], [false, []]);

        model.start();
        fakeBackend.searchEvent({
            "searchId": "s3",
            "kind": "backend",
            "backend": "index",
            "phase": "syncing",
            "depthLimit": 4
        });
        check("backend event is exposed", [model.servedBy, model.indexPhase, model.depthLimit], ["index", "syncing", 4]);
        const scored = (name, score, size, mtimeMs) => Object.assign(entry(name), {
                "score": score,
                "size": size,
                "mtimeMs": mtimeMs
            });
        fakeBackend.searchEvent({
            "searchId": "s3",
            "kind": "results",
            "entries": [scored("b.txt", 600, 30, 3), scored("a.txt", 800, 10, 1)],
            "count": 2
        });
        fakeBackend.searchEvent({
            "searchId": "s3",
            "kind": "results",
            "entries": [scored("c.txt", 700, 20, 2), scored("d.txt", 600, 40, 4)],
            "count": 4
        });
        check("batches insert by score, ties by arrival", names(), ["a.txt", "c.txt", "b.txt", "d.txt"]);
        model.setSort("name");
        check("name sort", names(), ["a.txt", "b.txt", "c.txt", "d.txt"]);
        model.setSort("name");
        check("same key flips direction", [model.sortDescending, names()], [true, ["d.txt", "c.txt", "b.txt", "a.txt"]]);
        model.setSort("size");
        check("size sort resets direction", [model.sortDescending, names()], [false, ["a.txt", "c.txt", "b.txt", "d.txt"]]);
        fakeBackend.searchEvent({
            "searchId": "s3",
            "kind": "results",
            "entries": [scored("e.txt", 900, 25, 5)],
            "count": 5
        });
        check("late batches respect the current sort", names(), ["a.txt", "c.txt", "e.txt", "b.txt", "d.txt"]);
        model.setSort("mtime");
        model.sortDescending = true;
        check("modified sort", names(), ["e.txt", "d.txt", "b.txt", "c.txt", "a.txt"]);
        model.setSort("");
        check("relevance again", [model.sortDescending, names()], [false, ["e.txt", "a.txt", "c.txt", "b.txt", "d.txt"]]);
        fakeBackend.searchEvent({
            "searchId": "s3",
            "kind": "done",
            "count": 5,
            "partial": true
        });
        check("done reports partial", [model.partial, model.loading], [true, false]);
        check("path lookup follows the sorted rows", [model.indexOfPath("/tmp/c.txt"), model.indexOfPath("/tmp/d.txt"), model.indexOfPath("/tmp/nope")], [2, 4, -1]);

        startResponse = {
            "error": "not a directory",
            "code": "ENOTDIR"
        };
        model.start();
        check("a failed start reports the code", [model.errorCode, model.loading], ["ENOTDIR", false]);

        startResponse = ({});
        model.start();
        check("contents are not offered before a backend says so", model.contentsAvailable, false);
        fakeBackend.searchEvent({
            "searchId": "s5",
            "kind": "backend",
            "backend": "rg",
            "contents": true
        });
        fakeBackend.searchEvent({
            "searchId": "s5",
            "kind": "results",
            "entries": [Object.assign(entry("hit.txt"), {
                    "line": 12,
                    "snippet": "the needle"
                })],
            "count": 1
        });
        check("content hits keep their line", [model.servedBy, model.entries.get(0).line, model.entries.get(0).snippet], ["rg", 12, "the needle"]);
        fakeBackend.searchEvent({
            "searchId": "s5",
            "kind": "done",
            "count": 1
        });
        model.start();
        check("contents stay offered while the next search starts", [model.contentsAvailable, model.servedBy], [true, ""]);
        fakeBackend.searchEvent({
            "searchId": "s6",
            "kind": "backend",
            "backend": "walk"
        });
        check("a backend without contents withdraws the offer", model.contentsAvailable, false);
        fakeBackend.searchEvent({
            "searchId": "s6",
            "kind": "done",
            "count": 0,
            "code": "EINVAL",
            "error": "regex parse error"
        });
        check("a search that ends in an error reports it", [model.errorCode, model.error, model.loading], ["EINVAL", "regex parse error", false]);

        log.info(failures === 0 ? "FIXTURE PASS" : `FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
