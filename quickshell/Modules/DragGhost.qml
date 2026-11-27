pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Rectangle {
    id: ghost

    property int count: 0
    property string iconName: ""
    property string thumbnail: ""
    property string mime: ""
    property string path: ""
    property bool isDir: false

    implicitWidth: body.implicitWidth + Theme.spacingS * 2
    implicitHeight: FilesMetrics.dragGhostSize + Theme.spacingS * 2
    radius: Theme.cornerRadiusL
    color: Theme.chipSurface
    border.width: Theme.outlineWidth
    border.color: Theme.primary

    Row {
        id: body

        anchors.centerIn: parent

        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: FilesMetrics.dragGhostSize
            height: FilesMetrics.dragGhostSize

            FileIcon {
                anchors.fill: parent
                iconName: ghost.iconName
                thumbnail: ghost.thumbnail !== "" ? ghost.thumbnail : ghost.mime === "image/svg+xml" ? ghost.path : ""
                size: FilesMetrics.dragGhostSize
                color: ghost.isDir ? Theme.primary : Theme.surfaceText
                asynchronous: false
            }

            Rectangle {
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                visible: ghost.count > 1
                width: Math.max(height, badge.implicitWidth + Theme.spacingXS)
                height: Theme.iconSizeSmall
                radius: Theme.fullRadius(width, height)
                color: Theme.primary

                StyledText {
                    id: badge

                    anchors.centerIn: parent
                    text: ghost.count
                    color: Theme.onPrimary
                    font.pixelSize: Theme.fontSizeSmall
                    font.weight: Theme.fontWeightMedium
                }
            }
        }
    }
}
