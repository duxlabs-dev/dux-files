pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services
import "FolderAppearance.js" as Look

Singleton {
    id: root

    readonly property int cap: 1000
    readonly property var colors: ["", "#448aff", "#66bb6a", "#ffb74d", "#ef5350", "#ab47bc", "#26a69a", "#ec407a"]
    readonly property var badges: _catalog.badges ?? []
    readonly property var icons: _catalog.icons ?? []

    property var entries: ({})
    property var _catalog: ({})

    signal changed(string key)

    // Accessors read entries, so bound rows repaint when a look changes.
    function pathKey(path) {
        return FolderViews.keyFor(path, DevicesService.devices);
    }

    function lookup(path) {
        if (!path)
            return null;
        return entries[pathKey(path)] ?? null;
    }

    function tintFor(path, fallback) {
        return Look.tintFor(lookup(path), fallback);
    }

    function emblemFor(path) {
        return Look.emblemFor(lookup(path));
    }

    function iconFor(path, fallback) {
        return Look.iconFor(lookup(path), fallback);
    }

    function set(path, values) {
        if (!path)
            return;
        const look = Look.normalize(values);
        const target = pathKey(path);
        const next = Object.assign({}, entries);
        if (Look.isEmpty(look))
            delete next[target];
        else
            next[target] = look;
        _commit(next, target);
    }

    function reset(path) {
        set(path, {});
    }

    function refreshCatalog() {
        if (_catalog.badges)
            return;
        DuxFilesService.call("files.iconCatalog", null, result => {
            if (result.error)
                return;
            _catalog = result;
        });
    }

    function moved(pairs) {
        let next = entries;
        for (const pair of pairs) {
            const from = pathKey(pair.from);
            if (!FolderViews.hasKeysUnder(next, from))
                continue;
            next = FolderViews.renameKeys(next, from, pathKey(pair.to));
        }
        if (next === entries)
            return;
        _commit(next, "");
    }

    function _commit(next, key) {
        entries = next;
        store.save();
        changed(key);
    }

    function _pruned(table) {
        const keys = Object.keys(table);
        if (keys.length <= cap)
            return table;
        const next = Object.assign({}, table);
        for (const key of keys.slice(0, keys.length - cap))
            delete next[key];
        return next;
    }

    Component.onCompleted: store.read()

    Connections {
        target: FileOpsService

        function onJobFinished(job) {
            if (job.state !== "done")
                return;
            if (job.moved?.length)
                root.moved(job.moved);
        }
    }

    JsonStore {
        id: store

        name: "folder appearances"
        path: Paths.strip(Paths.stateDir) + "/folder-appearances.json"
        serialize: () => JSON.stringify(root._pruned(root.entries), null, 1)
        parse: text => root.entries = objectFrom(text) ?? root.entries
    }
}
