pragma ComponentBehavior: Bound

import QtQuick
import qs.Services
import "../DCommon/FileBrowser/FilePreview.js" as FilePreview
import "../DCommon/FileBrowser/NameFilter.js" as NameFilter

QtObject {
    id: root

    property string path: ""
    property string sortKey: "name"
    property bool sortDescending: false
    property var descendingKeys: []
    property string nameFilter: ""
    property bool thumbnailsEnabled: true
    property real thumbnailSizeLimit: -1
    property string thumbnailSize: "normal"

    property bool loading: false
    property string error: ""
    property string errorCode: ""
    property int total: 0

    readonly property var backend: DirectoryService
    readonly property ListModel entries: ListModel {}
    readonly property int count: entries.count
    readonly property bool pollOnFocus: false

    property var _all: []
    property var _shown: []
    property var _positions: new Map()
    property var _life: ({
            "stale": false
        })
    property int _request: 0
    property bool _loaded: false

    signal listed
    signal pathReset
    signal gone

    onNameFilterChanged: _refilter()
    Component.onCompleted: refresh()
    Component.onDestruction: _life.stale = true

    function load(callback) {
        callback({
            "entries": []
        });
    }

    function compare(key, a, b) {
        return key === "size" ? a.size - b.size : 0;
    }

    function rowChanged(current, next) {
        return current.mtimeMs !== next.mtimeMs || current.size !== next.size;
    }

    function refresh() {
        const life = _life;
        const request = ++_request;
        loading = !_loaded;
        load(result => {
            if (life.stale || request !== root._request)
                return;
            root.apply(result);
        });
    }

    function apply(result) {
        _loaded = true;
        loading = false;
        error = result.error ?? "";
        errorCode = result.code ?? "";
        _all = sorted(Array.from(result.entries || []), sortKey, sortDescending);
        total = _all.length;
        _shown = syncInto(entries, _shown, filtered(_all, nameFilter));
        _index();
        listed();
    }

    function setSort(key, descending) {
        sortKey = key;
        sortDescending = descending;
        _all = sorted(_all, key, descending);
        _refilter();
    }

    function sortBy(key) {
        setSort(key, key === sortKey ? !sortDescending : descendingKeys.includes(key));
    }

    function loadMore() {
    }

    function indexOfPath(target) {
        return _positions.get(target) ?? -1;
    }

    function requestThumbnails(first, last) {
        if (first < 0 || last < first || !thumbnailsEnabled)
            return;
        const tooLarge = entry => thumbnailSizeLimit >= 0 && entry.size > thumbnailSizeLimit && entry.mime.startsWith("image/");
        const paths = _shown.slice(first, last + 1).filter(entry => entry.thumbnailable && !FilePreview.thumbnailCovers(entry.thumbnail, thumbnailSize) && !tooLarge(entry)).map(entry => entry.path);
        if (paths.length === 0)
            return;
        const life = _life;
        backend.thumbnails(paths, thumbnailSize, "", result => {
            if (life.stale || result.error)
                return;
            for (const item of result.results || []) {
                const index = root._positions.get(item.path);
                if (!item.thumbnail || index === undefined)
                    continue;
                root._shown[index].thumbnail = item.thumbnail;
                root.entries.setProperty(index, "thumbnail", item.thumbnail);
            }
        });
    }

    function sorted(items, key, descending) {
        const direction = descending ? -1 : 1;
        const byName = (a, b) => a.name.localeCompare(b.name);
        return items.slice().sort((a, b) => direction * root.compare(key, a, b) || byName(a, b));
    }

    function filtered(entries, filter) {
        if (filter === "")
            return entries.slice();
        const matcher = NameFilter.compile(filter);
        return entries.filter(entry => NameFilter.entryMatches(matcher, entry));
    }

    function syncInto(list, shown, next) {
        if (next.length === 0 || shown.length === 0) {
            list.clear();
            list.append(next);
            return next;
        }
        const mirror = shown.slice();
        const kept = new Set(next.map(entry => entry.path));
        for (let i = mirror.length - 1; i >= 0; i--) {
            if (kept.has(mirror[i].path))
                continue;
            list.remove(i);
            mirror.splice(i, 1);
        }
        for (let i = 0; i < next.length; i++) {
            const current = mirror[i];
            if (current?.path !== next[i].path) {
                list.insert(i, next[i]);
                mirror.splice(i, 0, next[i]);
                continue;
            }
            if (next[i].thumbnail === "")
                next[i].thumbnail = current.thumbnail;
            if (root.rowChanged(current, next[i]))
                list.set(i, next[i]);
            else if (current.thumbnail !== next[i].thumbnail)
                list.setProperty(i, "thumbnail", next[i].thumbnail);
            mirror[i] = next[i];
        }
        if (mirror.length > next.length)
            list.remove(next.length, mirror.length - next.length);
        return next;
    }

    function _refilter() {
        _shown = syncInto(entries, _shown, filtered(_all, nameFilter));
        _index();
        listed();
    }

    function _index() {
        _positions = new Map(_shown.map((entry, index) => [entry.path, index]));
    }
}
