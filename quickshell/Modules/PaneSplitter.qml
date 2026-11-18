pragma ComponentBehavior: Bound

import QtQuick
import qs.Common

Item {
    id: splitter

    signal moved(real delta)
    signal reset

    width: FilesMetrics.splitterWidth

    Rectangle {
        anchors.centerIn: parent
        width: Theme.spacingXXS
        height: FilesMetrics.splitterHandleHeight
        radius: Theme.cornerRadiusFull
        color: handle.containsMouse || handle.pressed ? Theme.primary : Theme.outlineVariant
        opacity: handle.containsMouse || handle.pressed ? 1 : Theme.pendingOpacity

        Behavior on color {
            enabled: Theme.animationsEnabled
            ColorAnimation {
                duration: Theme.expressiveDurations.expressiveFastEffects
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveFastEffects
            }
        }
    }

    MouseArea {
        id: handle

        property real pressX: 0

        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.SplitHCursor
        acceptedButtons: Qt.LeftButton
        onPressed: mouse => pressX = mouse.x
        onPositionChanged: mouse => {
            if (!pressed)
                return;
            splitter.moved(I18n.isRtl ? pressX - mouse.x : mouse.x - pressX);
        }
        onDoubleClicked: splitter.reset()
    }
}
