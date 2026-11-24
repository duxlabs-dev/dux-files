pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import qs.Common
import qs.Services
import qs.DCommon.Common as DC
import qs.DCommon.Widgets

Popup {
    id: root

    width: FilesMetrics.popoverWidth
    margins: 0
    padding: Theme.spacingL
    modal: false
    focus: false
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutsideParent

    background: Item {
        DC.ElevationShadow {
            anchors.fill: parent
            level: Theme.elevationLevel2
            fallbackOffset: Theme.spacingXS
            targetRadius: Theme.cornerRadiusL
            targetColor: Theme.cardSurface
            borderColor: Theme.outlineMedium
            borderWidth: Theme.layerOutlineWidth
            shadowEnabled: Theme.elevationEnabled && SettingsData.popoutElevationEnabled
        }
    }

    contentItem: Loader {
        active: root.visible
        sourceComponent: Column {
            spacing: Theme.spacingM

            Component.onDestruction: FileOpsService.holdHistory(false)

            HoverHandler {
                onHoveredChanged: FileOpsService.holdHistory(hovered)
            }

            StyledText {
                text: I18n.tr("File operations", "jobs popover header")
                color: Theme.primary
                font.pixelSize: Theme.fontSizeMedium
                font.weight: Theme.fontWeightMedium
            }

            Repeater {
                model: FileOpsService.jobs

                JobRow {
                    required property var modelData
                    width: root.availableWidth
                    job: modelData
                }
            }

            StyledText {
                visible: FileOpsService.jobs.length === 0
                text: I18n.tr("Nothing is running", "jobs popover empty state")
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
            }
        }
    }
}
