pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    readonly property int cap: 1000
    readonly property int touchGranularityMs: 3600000
    readonly property var valueKeys: ["sortKey", "sortDescending", "viewMode"]

    property var entries: ({})

    signal changed(string key)

    function keyFor(path, devices) {
        let best = null;
        for (const device of devices ?? []) {
            if (device.system || !device.mounted || !device.uuid)
                continue;
            const mounts = device.mountPoints?.length ? device.mountPoints : [device.mountPoint];
            for (const mount of mounts) {
                if (!mount || mount === "/")
                    continue;
                const prefix = mount.endsWith("/") ? mount : mount + "/";
                if (path !== mount && !path.startsWith(prefix))
                    continue;
                if (best && mount.length <= best.mount.length)
                    continue;
                best = {
                    "mount": mount,
                    "uuid": device.uuid
                };
            }
        }
        if (!best)
            return path;
        const inside = path === best.mount ? "" : path.slice(best.mount.replace(/\/$/, "").length);
        return "uuid:" + best.uuid + ":" + inside;
    }

    function resolve(entry, lastUsed, defaults) {
        const pick = name => entry?.[name] ?? lastUsed?.[name] ?? defaults[name];
        const folderView = entry?.viewMode === "columns" ? undefined : entry?.viewMode;
        return {
            "sortKey": pick("sortKey"),
            "sortDescending": pick("sortDescending"),
            "viewMode": folderView ?? lastUsed?.viewMode ?? defaults.viewMode
        };
    }

    function prune(table, limit) {
        const keys = Object.keys(table);
        if (keys.length <= limit)
            return table;
        keys.sort((a, b) => (table[a].used ?? 0) - (table[b].used ?? 0));
        const next = Object.assign({}, table);
        for (const key of keys.slice(0, keys.length - limit))
            delete next[key];
        return next;
    }

    function hasKeysUnder(table, from) {
        const prefix = from + "/";
        for (const key in table) {
            if (key === from || key.startsWith(prefix))
                return true;
        }
        return false;
    }

    function renameKeys(table, from, to) {
        const next = ({});
        const prefix = from + "/";
        const under = key => key === from || key.startsWith(prefix);
        for (const key in table) {
            if (!under(key))
                next[key] = table[key];
        }
        for (const key in table) {
            if (under(key))
                next[to + key.slice(from.length)] = table[key];
        }
        return next;
    }

    function stripMatching(table, values) {
        const next = ({});
        for (const key in table) {
            const entry = Object.assign({}, table[key]);
            if (entry.sortKey === values.sortKey && entry.sortDescending === values.sortDescending) {
                delete entry.sortKey;
                delete entry.sortDescending;
            }
            if (entry.viewMode === values.viewMode)
                delete entry.viewMode;
            if (entry.sortKey === undefined && entry.viewMode === undefined)
                continue;
            next[key] = entry;
        }
        return next;
    }

    function key(path) {
        return keyFor(path, DevicesService.devices);
    }

    function lookup(path) {
        return entries[key(path)] ?? null;
    }

    function set(path, values) {
        const target = key(path);
        const entry = Object.assign({}, entries[target], values, {
            "used": Date.now()
        });
        const next = Object.assign({}, entries);
        next[target] = entry;
        _commit(prune(next, cap), target);
    }

    function reset(path) {
        const target = key(path);
        if (!entries[target])
            return;
        const next = Object.assign({}, entries);
        delete next[target];
        _commit(next, target);
    }

    function touch(path) {
        const target = key(path);
        const entry = entries[target];
        if (!entry || Date.now() - (entry.used ?? 0) < touchGranularityMs)
            return;
        const next = Object.assign({}, entries);
        next[target] = Object.assign({}, entry, {
            "used": Date.now()
        });
        entries = next;
        touchDebounce.restart();
    }

    function dropMatching(values) {
        _commit(stripMatching(entries, values), "");
    }

    function clear() {
        if (Object.keys(entries).length === 0)
            return;
        _commit({}, "");
    }

    function moved(pairs) {
        let next = entries;
        for (const pair of pairs) {
            const from = key(pair.from);
            if (!hasKeysUnder(next, from))
                continue;
            next = renameKeys(next, from, key(pair.to));
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

    Timer {
        id: touchDebounce

        interval: 60000
        onTriggered: store.write()
    }

    JsonStore {
        id: store

        name: "folder views"
        path: Paths.strip(Paths.stateDir) + "/folder-views.json"
        serialize: () => JSON.stringify(root.entries, null, 1)
        parse: text => root.entries = objectFrom(text) ?? root.entries
        onWritten: touchDebounce.stop()
    }
}
