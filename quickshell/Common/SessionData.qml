pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Services

Singleton {
    id: root

    readonly property var log: Log.scoped("SessionData")

    readonly property string sessionPath: Paths.strip(Paths.stateDir) + "/session.json"

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

    property bool _loaded: false
    property bool _applying: false
    property bool _selfWrite: false

    function lastUsed(key, fallback) {
        return browsing?.[key] ?? fallback;
    }

    function remember(values) {
        if (Object.keys(values).every(key => browsing[key] === values[key]))
            return;
        browsing = Object.assign({}, browsing, values);
    }

    onPersistedStateChanged: {
        if (!_loaded || _applying)
            return;
        writeDebounce.restart();
    }

    function _apply(text) {
        if (text === "") {
            _loaded = true;
            return;
        }

        let data = null;
        try {
            data = JSON.parse(text);
        } catch (e) {
            log.warn(`ignoring unreadable session state: ${e}`);
        }

        if (Array.isArray(data?.recentDirs))
            legacyRecentDirs = data.recentDirs;
        _applying = true;
        for (const key in persistedState) {
            if (!data || data[key] === undefined)
                continue;
            root[key] = data[key];
        }
        _applying = false;
        _loaded = true;
    }

    // blocking read so the first window opens with the saved state
    Component.onCompleted: sessionFile.text()

    Timer {
        id: writeDebounce

        interval: 400
        onTriggered: {
            if (!Paths.ready) {
                restart();
                return;
            }
            root._selfWrite = true;
            sessionFile.setText(JSON.stringify(root.persistedState, null, 1));
        }
    }

    FileView {
        id: sessionFile

        path: root.sessionPath
        blockLoading: true
        atomicWrites: true
        watchChanges: true
        printErrors: false

        onLoaded: root._apply(text())
        onLoadFailed: root._loaded = true
        onFileChanged: {
            if (root._selfWrite) {
                root._selfWrite = false;
                return;
            }
            reload();
        }
    }
}
