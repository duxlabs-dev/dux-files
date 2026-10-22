pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    readonly property var log: Log.scoped("PdfService")
    readonly property bool available: _probe?.status === Component.Ready
    readonly property bool unavailable: _probe?.status === Component.Error
    property Component _probe: null

    function ensureProbed() {
        if (_probe)
            return;
        _probe = Qt.createComponent(Qt.resolvedUrl("PdfProbe.qml"), Component.PreferSynchronous, root);
        if (unavailable)
            log.warn("QtQuick.Pdf not available");
    }
}
