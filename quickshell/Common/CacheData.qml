pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property alias fileBrowserSettings: adapter.fileBrowserSettings
    property alias sidebarWidth: adapter.sidebarWidth
    property alias previewWidth: adapter.previewWidth

    function saveCache() {
        store.save();
    }

    JsonStore {
        id: store

        name: "cache"
        path: Paths.strip(Paths.cache) + "/cache.json"

        adapter: JsonAdapter {
            id: adapter

            property var fileBrowserSettings: ({})
            property int sidebarWidth: FilesMetrics.sidebarWidth
            property int previewWidth: 0
        }
    }
}
