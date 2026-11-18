pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Rectangle {
    id: chip

    property string action: ""

    readonly property string iconName: {
        switch (action) {
        case "copy":
            return "content_copy";
        case "link":
            return "add_link";
        case "ask":
            return "more_horiz";
        case "trash":
            return "delete";
        default:
            return "drive_file_move";
        }
    }
    readonly property string label: {
        switch (action) {
        case "copy":
            return I18n.tr("Copy", "context menu entry copying the selection");
        case "link":
            return I18n.tr("Link", "drag label for creating a symlink");
        case "ask":
            return I18n.tr("Ask", "drag label when the drop opens a copy, move or link menu");
        case "trash":
            return I18n.tr("Move to Trash", "context menu entry trashing the selection");
        default:
            return I18n.tr("Move", "drag label for moving files");
        }
    }

    implicitWidth: row.implicitWidth + FileBrowserMetrics.pathPillPadding * 2
    implicitHeight: FileBrowserMetrics.pathPillHeight
    radius: FileBrowserMetrics.pathPillRadius
    color: Theme.chipSurface
    border.width: Theme.outlineWidth
    border.color: Theme.primary

    Row {
        id: row

        anchors.centerIn: parent
        spacing: FileBrowserMetrics.pathPillSpacing

        DIcon {
            anchors.verticalCenter: parent.verticalCenter
            name: chip.iconName
            size: FileBrowserMetrics.pathIconSize
            color: Theme.primary
        }

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            text: chip.label
            color: Theme.surfaceText
            font.pixelSize: Theme.fontSizeMedium
            font.weight: Theme.fontWeightMedium
        }
    }
}
