pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Item {
    id: jobs

    property real maximumHeight: Number.POSITIVE_INFINITY

    readonly property bool rail: width < FileBrowserMetrics.sidebarIconOnlyBelow
    readonly property string title: I18n.tr("File operations", "jobs popover header")

    visible: FileOpsService.jobs.length > 0
    height: {
        if (!visible)
            return 0;
        return rail ? button.height : panel.height;
    }
    onRailChanged: popover.close()

    FilePlaceRow {
        id: button

        visible: jobs.rail
        label: jobs.title
        glyph: FileOpsService.activeCount > 0 ? "sync" : "check_circle"
        onClicked: popover.visible ? popover.close() : popover.open()

        JobsPopover {
            id: popover

            x: I18n.isRtl ? -width - Theme.spacingS : button.width + Theme.spacingS
            y: button.height - height
        }
    }

    Rectangle {
        id: panel

        visible: !jobs.rail
        width: parent.width
        height: Math.min(jobs.maximumHeight, list.height + Theme.spacingM * 2)
        radius: FileBrowserMetrics.sidebarRowRadius
        color: Theme.chipLayer

        HoverHandler {
            onHoveredChanged: FileOpsService.holdHistory(hovered)
        }

        DFlickable {
            anchors.fill: parent
            anchors.margins: Theme.spacingM
            contentHeight: list.height
            clip: true

            Column {
                id: list

                width: parent.width
                spacing: Theme.spacingM

                StyledText {
                    text: jobs.title
                    color: Theme.surfaceVariantText
                    font.pixelSize: Theme.fontSizeSmall
                    font.weight: Theme.fontWeightMedium
                }

                Repeater {
                    model: jobs.visible && !jobs.rail ? FileOpsService.jobs : []

                    JobRow {
                        required property var modelData

                        width: list.width
                        job: modelData
                    }
                }
            }
        }
    }
}
