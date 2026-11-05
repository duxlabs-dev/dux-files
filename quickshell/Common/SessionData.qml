pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    property real windowWidth: 960
    property real windowHeight: 640
    property bool maximized: false
    property bool splitView: false
    property real splitRatio: 0.5
    property int currentTab: 0
    property var tabs: []
    property int gridZoom: FilesMetrics.defaultGridZoom
    property int listZoom: FilesMetrics.defaultListZoom
    property int columnZoom: FilesMetrics.defaultColumnZoom
    property real columnPaneWidth: 0
    property var listColumns: ["size", "modified", "type"]
    property var columnWidths: ({})
    property var browsing: ({})

    readonly property var persistedState: ({
            "windowWidth": Math.round(windowWidth),
            "windowHeight": Math.round(windowHeight),
            "maximized": maximized,
            "splitView": splitView,
            "splitRatio": splitRatio,
            "currentTab": currentTab,
            "tabs": tabs,
            "gridZoom": gridZoom,
            "listZoom": listZoom,
            "columnZoom": columnZoom,
            "columnPaneWidth": Math.round(columnPaneWidth),
            "listColumns": listColumns,
            "columnWidths": columnWidths,
            "browsing": browsing
        })

    property var legacyRecentDirs: []
    property bool _applying: false

    function lastUsed(key, fallback) {
        return browsing?.[key] ?? fallback;
    }

    function remember(values) {
        if (Object.keys(values).every(key => browsing[key] === values[key]))
            return;
        browsing = Object.assign({}, browsing, values);
    }

    onPersistedStateChanged: {
        if (_applying)
            return;
        store.save();
    }

    function _apply(text) {
        const data = store.objectFrom(text);
        if (!data)
            return;
        if (Array.isArray(data.recentDirs))
            legacyRecentDirs = data.recentDirs;
        _applying = true;
        for (const key in persistedState) {
            if (data[key] === undefined)
                continue;
            root[key] = data[key];
        }
        _applying = false;
    }

    // blocking read so the first window opens with the saved state
    Component.onCompleted: store.read()

    JsonStore {
        id: store

        name: "session state"
        path: Paths.strip(Paths.stateDir) + "/session.json"
        watchChanges: true
        serialize: () => JSON.stringify(root.persistedState, null, 1)
        parse: text => root._apply(text)
    }
}
