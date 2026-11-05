pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    readonly property bool connected: DuxFilesService.connected
    property var userDirs: []
    property var _watchTopics: ({})
    property var _searchTopics: ({})
    readonly property var _eventHandler: data => root.watchEvent(data)
    readonly property var _searchHandler: data => root._onSearchEvent(data)
    readonly property var _sizeHandler: data => root.sizeEvent(data)

    signal watchEvent(var data)
    signal searchEvent(var data)
    signal sizeEvent(var data)

    readonly property string templatesDir: userDirs.find(dir => dir.key === "templates")?.path ?? ""

    function refreshUserDirs() {
        DuxFilesService.call("files.userDirs", {}, result => {
            if (result.error)
                return;
            root.userDirs = result.dirs || [];
        });
    }

    function watch(path, options, callback) {
        DuxFilesService.call("files.watch", Object.assign({
            "path": path
        }, _options(options)), result => {
            if (result.topic)
                root._track(root._watchTopics, result.watchId, result.topic, root._eventHandler);
            callback?.(result);
        });
    }

    function unwatch(watchId) {
        if (!watchId)
            return;
        _forget(_watchTopics, watchId, _eventHandler);
        DuxFilesService.call("files.unwatch", {
            "watchId": watchId
        }, null);
    }

    function list(path, options, callback) {
        DuxFilesService.call("files.list", Object.assign({
            "path": path
        }, _options(options)), callback);
    }

    function resolveLocation(text, cwd, callback) {
        DuxFilesService.call("location.resolve", {
            "text": text,
            "cwd": cwd
        }, callback);
    }

    function missingPaths(paths, callback) {
        DuxFilesService.call("location.missing", {
            "paths": paths
        }, result => callback(result.missing ?? []));
    }

    function page(watchId, options, callback) {
        DuxFilesService.call("files.list", Object.assign({
            "watchId": watchId
        }, _options(options)), callback);
    }

    function stat(path, callback) {
        DuxFilesService.call("files.stat", {
            "path": path
        }, callback);
    }

    function preview(path, maxBytes, callback) {
        DuxFilesService.call("files.preview", {
            "path": path,
            "maxBytes": maxBytes
        }, callback);
    }

    function imageSize(path, callback) {
        DuxFilesService.call("files.imageSize", {
            "path": path
        }, callback);
    }

    function count(paths, includeHidden, callback) {
        DuxFilesService.call("files.count", {
            "paths": paths,
            "includeHidden": includeHidden === true
        }, callback);
    }

    function scanSize(paths, callback) {
        DuxFilesService.call("files.size", {
            "paths": paths
        }, callback);
    }

    function cancelScan(scanId) {
        if (!scanId)
            return;
        DuxFilesService.call("files.sizeCancel", {
            "scanId": scanId
        }, null);
    }

    function thumbnails(paths, size, watchId, callback) {
        const params = {
            "paths": paths,
            "size": size || "normal"
        };
        if (watchId)
            params.watchId = watchId;
        DuxFilesService.call("files.thumbnail", params, callback);
    }

    function search(rootPath, query, options, callback) {
        const params = Object.assign({
            "root": rootPath,
            "query": query
        }, options || {});
        DuxFilesService.call("search.start", params, result => {
            if (result.topic)
                root._track(root._searchTopics, result.searchId, result.topic, root._searchHandler);
            callback?.(result);
        });
    }

    function cancelSearch(searchId) {
        if (!searchId)
            return;
        _forget(_searchTopics, searchId, _searchHandler);
        DuxFilesService.call("search.cancel", {
            "searchId": searchId
        }, null);
    }

    function _onSearchEvent(data) {
        searchEvent(data);
        if (data.kind === "done")
            _forget(_searchTopics, data.searchId, _searchHandler);
    }

    function _track(topics, id, topic, handler) {
        topics[id] = topic;
        DuxFilesService.subscribeTopic(topic, handler);
    }

    function _forget(topics, id, handler) {
        const topic = topics[id];
        if (!topic)
            return;
        delete topics[id];
        DuxFilesService.unsubscribeTopic(topic, handler);
    }

    function _forgetAll(topics, handler) {
        for (const id in topics)
            _forget(topics, id, handler);
    }

    function statAll(paths, callback) {
        const entries = [];
        let pending = paths.length;
        if (pending === 0) {
            callback([], []);
            return;
        }
        const failures = [];
        paths.forEach((path, index) => stat(path, result => {
                if (result.error)
                    failures.push(result);
                entries[index] = result.entry ?? null;
                pending--;
                if (pending > 0)
                    return;
                callback(entries.filter(entry => entry), failures);
            }));
    }

    function mkdir(path, callback) {
        DuxFilesService.call("files.mkdir", {
            "path": path
        }, callback);
    }

    function rename(path, name, callback) {
        DuxFilesService.call("files.rename", {
            "path": path,
            "name": name
        }, callback);
    }

    function trash(paths, callback) {
        DuxFilesService.call("files.trash", {
            "paths": paths
        }, callback);
    }

    function _options(options) {
        const opts = options || {};
        const params = {};
        if (opts.includeHidden !== undefined)
            params.includeHidden = opts.includeHidden;
        if (opts.filters !== undefined)
            params.filters = opts.filters;
        if (opts.sortKey)
            params.sort = opts.sortKey;
        if (opts.sortDesc !== undefined)
            params.desc = opts.sortDesc;
        if (opts.dirsFirst !== undefined)
            params.dirsFirst = opts.dirsFirst;
        if (opts.hiddenLast !== undefined)
            params.hiddenLast = opts.hiddenLast;
        if (opts.limit)
            params.limit = opts.limit;
        if (opts.cursor)
            params.cursor = opts.cursor;
        return params;
    }

    Component.onCompleted: {
        DuxFilesService.subscribeTopic("sizes", _sizeHandler);
        refreshUserDirs();
    }

    Connections {
        target: DuxFilesService

        function onConnectedChanged() {
            root._forgetAll(root._watchTopics, root._eventHandler);
            root._forgetAll(root._searchTopics, root._searchHandler);
            if (DuxFilesService.connected)
                root.refreshUserDirs();
        }
    }
}
