import QtQuick
import qs.Common

Rectangle {
    property bool first: false
    property bool last: false

    readonly property real topRadius: first ? FilesMetrics.paneRadius : Theme.groupedListInnerRadius
    readonly property real bottomRadius: last ? FilesMetrics.paneRadius : Theme.groupedListInnerRadius

    anchors.left: parent.left
    anchors.right: parent.right
    color: Theme.foregroundColor(Theme.cardSurface, true)
    topLeftRadius: topRadius
    topRightRadius: topRadius
    bottomLeftRadius: bottomRadius
    bottomRightRadius: bottomRadius
}
