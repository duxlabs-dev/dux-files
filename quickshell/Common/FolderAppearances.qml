pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Services
import "FolderAppearance.js" as Look

Singleton {
    id: root

    readonly property var log: Log.scoped("FolderAppearances")
    readonly property string filePath: Paths.strip(Paths.stateDir) + "/folder-appearances.json"
    readonly property int cap: 1000
    readonly property var colors: ["", "#448aff", "#66bb6a", "#ffb74d", "#ef5350", "#ab47bc", "#26a69a", "#ec407a"]
    readonly property var badges: _catalog.badges ?? []
    readonly property var icons: _catalog.icons ?? []

    property var entries: ({})
    property bool _loaded: false
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
        DuxFilesService.sendRequest("files.iconCatalog", null, response => {
            if (response.error)
                return;
            _catalog = response.result ?? ({});
        });
    }

    function _commit(next, key) {
        entries = next;
        writeDebounce.restart();
        changed(key);
    }

    function _apply(text) {
        _loaded = true;
        if (text === "")
            return;
        try {
            const data = JSON.parse(text);
            if (data && typeof data === "object" && !Array.isArray(data))
                entries = data;
        } catch (e) {
            log.warn(`ignoring unreadable folder appearances: ${e}`);
        }
    }

    function _write() {
        if (!_loaded || !Paths.ready) {
            writeDebounce.restart();
            return;
        }
        file.setText(JSON.stringify(_pruned(entries), null, 1));
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

    Component.onCompleted: _apply(file.text() ?? "")

    Timer {
        id: writeDebounce

        interval: 400
        onTriggered: root._write()
    }

    FileView {
        id: file

        path: root.filePath
        blockLoading: true
        atomicWrites: true
        printErrors: false
    }
}
