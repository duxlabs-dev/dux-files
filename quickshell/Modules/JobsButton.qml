import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

DActionButton {
    id: button

    property bool hosted: true

    visible: hosted && FileOpsService.jobs.length > 0
    iconName: FileOpsService.activeCount > 0 ? "sync" : "check_circle"
    buttonSize: Theme.buttonHeightXXS
    iconSize: Theme.iconSizeSmall
    iconColor: Theme.surfaceText
    tooltipText: I18n.tr("File operations", "jobs popover header")
    onClicked: popover.visible ? popover.close() : popover.open()

    JobsPopover {
        id: popover

        y: button.height + Theme.spacingXS
        x: button.width - width
    }
}
