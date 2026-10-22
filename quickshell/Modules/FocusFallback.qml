import QtQuick
import QtQuick.Controls

Item {
    id: fallback

    readonly property Item focusItem: Window.activeFocusItem
    // Qt 6.8 leaves active focus on the root item after a focused Popup closes
    readonly property bool dropped: Window.active && focusItem !== null && focusItem.parent === null && Overlay.overlay.children.length === 0

    signal triggered

    onDroppedChanged: {
        if (dropped)
            Qt.callLater(fallback.fire);
    }

    function fire() {
        if (dropped)
            triggered();
    }
}
