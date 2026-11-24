pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    readonly property bool connected: transport.connected

    property var jobs: []
    property string lastDestination: ""

    readonly property int activeCount: jobs.filter(job => job.state !== "done" && job.state !== "failed").length

    readonly property int historyDuration: Timing.timeout
    property var historyDeadlines: ({})
    property bool historyHeld: false
    property real _heldAt: 0

    property var elevation: ({
            "available": false,
            "route": "",
            "reason": ""
        })
    readonly property bool canElevate: elevation.available === true

    signal requestFailed(string message)
    signal jobFinished(var job)

    property var _byId: ({})

    readonly property DaemonTopic _jobs: DaemonTopic {
        transport: root.transport
        topic: "jobs"
        handler: data => root._onEvent(data)
        onConnected: {
            root.refresh();
            root.refreshElevation();
        }
    }

    function copy(paths, dest, scope) {
        _scoped("files.op.copy", {
            "paths": paths,
            "dest": dest
        }, scope);
    }

    function move(paths, dest, scope) {
        _scoped("files.op.move", {
            "paths": paths,
            "dest": dest
        }, scope);
    }

    function rename(path, name, scope) {
        _scoped("files.op.rename", {
            "path": path,
            "name": name
        }, scope);
    }

    function newFolder(dir, name, scope, unique) {
        _scoped("files.op.newFolder", {
            "path": dir,
            "name": name,
            "unique": unique === true
        }, scope);
    }

    function newFile(dir, name, template, scope, unique) {
        _scoped("files.op.newFile", {
            "path": dir,
            "name": name,
            "template": template || "",
            "unique": unique === true
        }, scope);
    }

    function link(paths, dest, scope) {
        _scoped("files.op.link", {
            "paths": paths,
            "dest": dest
        }, scope);
    }

    function trash(paths, scope) {
        _scoped("files.op.trash", {
            "paths": paths
        }, scope);
    }

    function restore(names, trashDir, scope) {
        _scoped("files.op.restore", {
            "names": names,
            "trashDir": trashDir
        }, scope);
    }

    function deletePermanently(paths) {
        _start("files.op.delete", {
            "paths": paths
        });
    }

    function emptyTrash() {
        _start("files.op.emptyTrash", null);
    }

    function compress(paths, dest, name, format, password, scope) {
        _scoped("files.op.compress", {
            "paths": paths,
            "dest": dest,
            "name": name,
            "format": format || "zip",
            "password": password || ""
        }, scope);
    }

    function extract(paths, dest, scope) {
        _scoped("files.op.extract", {
            "paths": paths,
            "dest": dest
        }, scope);
    }

    function setPermissions(paths, mode, recursive) {
        _start("files.op.setPermissions", {
            "paths": paths,
            "mode": mode,
            "recursive": recursive === true
        });
    }

    function setOwner(paths, owner, group, recursive) {
        _start("files.op.setOwner", {
            "paths": paths,
            "owner": owner || "",
            "group": group || "",
            "recursive": recursive === true
        });
    }

    function respond(jobId, answerId, action, applyToAll) {
        _start("files.op.respond", {
            "jobId": jobId,
            "id": answerId,
            "action": action,
            "applyToAll": applyToAll === true
        });
    }

    function unlock(jobId, answerId, password) {
        _start("files.op.respond", {
            "jobId": jobId,
            "id": answerId,
            "action": "unlock",
            "password": password
        });
    }

    function cancel(jobId) {
        _start("files.op.cancel", {
            "jobId": jobId
        });
    }

    function undo(scope) {
        _scoped("files.op.undo", {}, scope);
    }

    function forget(scope) {
        transport.call("files.op.forget", {
            "scope": scope || ""
        }, null);
    }

    function redo(scope) {
        _scoped("files.op.redo", {}, scope);
    }

    function duplicate(paths, scope) {
        _scoped("files.op.duplicate", {
            "paths": paths
        }, scope);
    }

    function newFolderWith(paths, dir, name, scope, unique) {
        _scoped("files.op.newFolderWith", {
            "paths": paths,
            "path": dir,
            "name": name,
            "unique": unique === true
        }, scope);
    }

    function pasteImage(dir, name, scope) {
        _scoped("files.op.pasteImage", {
            "path": dir,
            "name": name
        }, scope);
    }

    function refresh() {
        transport.call("files.op.jobs", null, result => {
            if (result.error)
                return;
            _byId = ({});
            historyDeadlines = ({});
            for (const job of result.jobs || [])
                _byId[job.id] = job;
            _rebuild();
            _expire(jobs.filter(job => _finished(job)));
        });
    }

    function refreshElevation() {
        transport.call("system.elevation", null, result => {
            if (result.error)
                return;
            elevation = {
                "available": result.available === true,
                "route": result.route || "",
                "reason": result.reason || ""
            };
        });
    }

    function _scoped(method, params, scope) {
        params.scope = scope || "";
        _start(method, params);
    }

    function _start(method, params) {
        transport.call(method, params, result => {
            if (result.error) {
                root.requestFailed(result.error);
                return;
            }
            if (result.job && !root._byId[result.job.id])
                root._store(result.job);
        });
    }

    function _onEvent(data) {
        switch (data.kind) {
        case "job":
            _onJob(data.job);
            return;
        case "progress":
            _onProgress(data.jobId, data.progress);
            return;
        }
    }

    function _onJob(job) {
        if (!job)
            return;
        const finished = job.state === "done" || job.state === "failed";
        _store(job);
        if (!finished)
            return;
        jobFinished(job);
        _expire([job]);
    }

    readonly property Timer _historyTimeout: Timer {
        onTriggered: root._dropExpired()
    }

    function _finished(job) {
        return job.state === "done" || job.state === "failed";
    }

    function historyRemaining(jobId) {
        const deadline = historyDeadlines[jobId];
        if (deadline === undefined)
            return 0;
        const now = historyHeld ? _heldAt : Date.now();
        return Math.min(1, Math.max(0, deadline - now) / historyDuration);
    }

    function dismiss(jobId) {
        const job = _byId[jobId];
        if (!job || !_finished(job))
            return;
        _drop([jobId]);
    }

    function holdHistory(held) {
        if (held === historyHeld)
            return;
        const now = Date.now();
        if (held) {
            _heldAt = now;
            historyHeld = true;
            _historyTimeout.stop();
            return;
        }
        const resumed = ({});
        for (const id in historyDeadlines)
            resumed[id] = historyDeadlines[id] + now - _heldAt;
        historyDeadlines = resumed;
        historyHeld = false;
        _scheduleExpiry();
    }

    function _expire(finished) {
        if (finished.length === 0)
            return;
        const start = historyHeld ? _heldAt : Date.now();
        const deadlines = Object.assign({}, historyDeadlines);
        for (const job of finished)
            deadlines[job.id] = start + historyDuration;
        historyDeadlines = deadlines;
        _scheduleExpiry();
    }

    function _scheduleExpiry() {
        const pending = Object.values(historyDeadlines);
        if (historyHeld || pending.length === 0) {
            _historyTimeout.stop();
            return;
        }
        _historyTimeout.interval = Math.max(1, Math.min(...pending) - Date.now());
        _historyTimeout.restart();
    }

    function _dropExpired() {
        const now = Date.now();
        _drop(Object.keys(historyDeadlines).filter(id => historyDeadlines[id] <= now));
    }

    function _drop(ids) {
        const deadlines = Object.assign({}, historyDeadlines);
        for (const id of ids) {
            delete deadlines[id];
            delete _byId[id];
        }
        historyDeadlines = deadlines;
        _rebuild();
        _scheduleExpiry();
    }

    function _onProgress(jobId, progress) {
        const job = _byId[jobId];
        if (!job)
            return;
        job.progress = progress;
        _rebuild();
    }

    function _store(job) {
        _byId[job.id] = job;
        _rebuild();
    }

    function _rebuild() {
        const list = [];
        for (const id in _byId)
            list.push(_byId[id]);
        list.sort((a, b) => {
            if (_finished(a) !== _finished(b))
                return _finished(a) ? 1 : -1;
            if (_finished(a))
                return (b.endedMs || 0) - (a.endedMs || 0);
            return b.startedMs - a.startedMs;
        });
        jobs = list;
    }
}
