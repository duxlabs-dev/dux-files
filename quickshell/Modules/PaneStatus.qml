pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Row {
    id: status

    property int total: 0
    property int hidden: 0
    property bool compact: false
    property bool showClipboard: true
    property string hint: ""
    property real textRoom: Number.POSITIVE_INFINITY

    readonly property int clipboardCount: ClipboardService.paths.length
    readonly property string countText: {
        const items = FileFormat.count(total);
        if (hidden <= 0)
            return items;
        return I18n.tr("%1 · %2", "folder status: item count then hidden count").arg(items).arg(I18n.tr("%1 hidden", "count of hidden items left out of a folder listing").arg(hidden));
    }
    readonly property string clipboardText: {
        switch (true) {
        case ClipboardService.cut && clipboardCount === 1:
            return I18n.tr("1 item cut", "clipboard indicator for a single cut item");
        case ClipboardService.cut:
            return I18n.tr("%1 items cut", "clipboard indicator for cut items").arg(clipboardCount);
        case clipboardCount === 1:
            return I18n.tr("1 item copied", "clipboard indicator for a single copied item");
        default:
            return I18n.tr("%1 items copied", "clipboard indicator for copied items").arg(clipboardCount);
        }
    }

    spacing: Theme.spacingS

    StyledText {
        anchors.verticalCenter: parent.verticalCenter
        visible: !status.compact && (status.hint !== "" || status.total > 0 || status.hidden > 0)
        width: Math.min(implicitWidth, Math.max(0, status.textRoom))
        text: status.hint !== "" ? status.hint : status.countText
        color: Theme.surfaceVariantText
        font.pixelSize: Theme.fontSizeSmall
        elide: Text.ElideMiddle
    }

    Rectangle {
        anchors.verticalCenter: parent.verticalCenter
        visible: status.showClipboard && status.clipboardCount > 0
        width: chip.implicitWidth + FileBrowserMetrics.pathPillPadding + Theme.spacingXXS
        height: FileBrowserMetrics.pathPillHeight
        radius: FileBrowserMetrics.pathPillRadius
        color: Theme.chipSurface
        border.width: Theme.outlineWidth
        border.color: Theme.outlineVariant
        Accessible.role: Accessible.StaticText
        Accessible.name: status.clipboardText

        Row {
            id: chip

            anchors.left: parent.left
            anchors.leftMargin: FileBrowserMetrics.pathPillPadding
            anchors.verticalCenter: parent.verticalCenter
            spacing: FileBrowserMetrics.pathPillSpacing

            DIcon {
                anchors.verticalCenter: parent.verticalCenter
                name: ClipboardService.cut ? "content_cut" : "content_copy"
                size: FileBrowserMetrics.pathIconSize
                color: Theme.primary
            }

            StyledText {
                anchors.verticalCenter: parent.verticalCenter
                text: status.compact ? String(status.clipboardCount) : status.clipboardText
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeMedium
                font.weight: Theme.fontWeightMedium
            }

            DActionButton {
                anchors.verticalCenter: parent.verticalCenter
                buttonSize: FileBrowserMetrics.pathPillHeight - Theme.spacingXXS * 2
                iconName: "close"
                iconSize: Theme.iconSizeSmall
                iconColor: Theme.surfaceVariantText
                tooltipText: I18n.tr("Clear the clipboard", "tooltip on the button dropping the copied or cut items")
                onClicked: ClipboardService.clear()
            }
        }
    }
}
