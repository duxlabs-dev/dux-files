pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets

Item {
    id: row

    property var tabs: []
    property int currentIndex: 0
    property int draggingIndex: -1
    property var order: []
    property var positions: []
    property real dragX: 0
    property real pressOffset: 0

    signal tabSelected(int index)
    signal tabClosed(int index)
    signal tabAdded
    signal tabDropHovered(int index)
    signal tabDropped(int index, var drop)
    signal tabMoved(int from, int to)
    signal tabMenuRequested(int index, real pointX, real pointY)

    implicitHeight: FilesMetrics.tabHeight
    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    function identityOrder() {
        return Array.from({
            "length": tabRepeater.count
        }, (_, index) => index);
    }

    function layout() {
        let cursor = 0;
        const next = [];
        for (const index of draggingIndex >= 0 ? order : identityOrder()) {
            const item = tabRepeater.itemAt(index);
            if (!item)
                continue;
            next[index] = cursor;
            cursor += item.width + FilesMetrics.tabSpacing;
        }
        positions = next;
        strapline.width = Math.max(0, cursor - FilesMetrics.tabSpacing);
    }

    function reset() {
        draggingIndex = -1;
        order = identityOrder();
        layout();
    }

    function physicalX(logical, width) {
        return row.LayoutMirroring.enabled ? strapline.width - logical - width : logical;
    }

    function logicalX(physical) {
        return row.LayoutMirroring.enabled ? strapline.width - physical : physical;
    }

    function begin(index, pointerX) {
        const item = tabRepeater.itemAt(index);
        if (!item || tabRepeater.count < 2)
            return;
        order = identityOrder();
        draggingIndex = index;
        pressOffset = pointerX - (positions[index] ?? 0);
        dragX = positions[index] ?? 0;
    }

    function dragTo(pointerX) {
        const item = tabRepeater.itemAt(draggingIndex);
        if (!item)
            return;
        dragX = Math.max(0, Math.min(strapline.width - item.width, pointerX - pressOffset));
        const from = order.indexOf(draggingIndex);
        let target = from;
        for (let slot = 0; slot < order.length; slot++) {
            if (slot === from)
                continue;
            const other = tabRepeater.itemAt(order[slot]);
            if (!other)
                continue;
            const middle = positions[order[slot]] + other.width / 2;
            if (slot < from && dragX < middle)
                target = Math.min(target, slot);
            if (slot > from && dragX + item.width > middle)
                target = Math.max(target, slot);
        }
        if (target === from)
            return;
        const next = order.slice();
        next.splice(from, 1);
        next.splice(target, 0, draggingIndex);
        order = next;
        layout();
    }

    function finish() {
        const from = draggingIndex;
        if (from < 0)
            return;
        const to = order.indexOf(from);
        reset();
        if (to !== from)
            tabMoved(from, to);
    }

    Timer {
        id: relayout

        interval: 0
        onTriggered: row.reset()
    }

    DFlickable {
        id: strip

        anchors.left: parent.left
        anchors.right: addButton.left
        anchors.rightMargin: FilesMetrics.tabSpacing
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        wheelEnabled: false
        interactive: false
        clip: true
        flickableDirection: Flickable.HorizontalFlick
        contentWidth: strapline.width
        contentHeight: height

        WheelHandler {
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                strip.contentX = Math.max(0, Math.min(Math.max(0, strip.contentWidth - strip.width), strip.contentX - delta));
            }
        }

        Item {
            id: strapline

            height: strip.height

            Repeater {
                id: tabRepeater

                model: row.tabs.length
                onItemAdded: relayout.restart()
                onItemRemoved: relayout.restart()

                FilesTab {
                    id: tab

                    required property int index

                    x: row.physicalX(dragging ? row.dragX : (row.positions[index] ?? 0), width)
                    z: dragging ? 1 : 0
                    dragging: row.draggingIndex === index
                    label: row.tabs[index] ?? ""
                    current: index === row.currentIndex
                    onWidthChanged: row.layout()
                    onClicked: row.tabSelected(index)
                    onCloseRequested: row.tabClosed(index)
                    onDropHovered: row.tabDropHovered(index)
                    onDropped: drop => row.tabDropped(index, drop)
                    onMenuRequested: (pointX, pointY) => {
                        const point = tab.mapToItem(row, pointX, pointY);
                        row.tabMenuRequested(index, point.x, point.y);
                    }

                    Behavior on x {
                        enabled: row.draggingIndex >= 0 && !tab.dragging && Theme.animationsEnabled
                        NumberAnimation {
                            duration: Theme.expressiveDurations.expressiveFastSpatial
                            easing.type: Easing.BezierSpline
                            easing.bezierCurve: Theme.expressiveCurves.expressiveFastSpatial
                        }
                    }

                    DragHandler {
                        target: null
                        yAxis.enabled: false
                        enabled: row.tabs.length > 1
                        onGrabChanged: (transition, point) => {
                            switch (transition) {
                            case PointerDevice.GrabExclusive:
                                row.begin(tab.index, row.logicalX(strapline.mapFromItem(null, point.scenePressPosition.x, 0).x));
                                row.dragTo(row.logicalX(strapline.mapFromItem(null, point.scenePosition.x, 0).x));
                                return;
                            case PointerDevice.UngrabExclusive:
                                row.finish();
                                return;
                            case PointerDevice.CancelGrabExclusive:
                                row.reset();
                                return;
                            }
                        }
                        onCentroidChanged: {
                            if (!active || !tab.dragging)
                                return;
                            row.dragTo(row.logicalX(strapline.mapFromItem(null, centroid.scenePosition.x, 0).x));
                        }
                    }
                }
            }
        }
    }

    DActionButton {
        id: addButton

        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        focusPolicy: Qt.TabFocus
        buttonSize: Theme.buttonHeightXS
        iconName: "add"
        iconSize: Theme.iconSizeMedium
        iconColor: Theme.onSurfaceVariant
        tooltipText: I18n.tr("New tab", "tooltip on the button opening a new tab")
        onClicked: row.tabAdded()
    }

}
