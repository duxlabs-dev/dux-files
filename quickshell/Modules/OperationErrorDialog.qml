import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

PausedJobDialog {
    id: root

    readonly property var failure: prompt
    readonly property bool elevationOffered: offers("retryElevated")
    readonly property bool deniedUnelevated: failure !== null && failure.code === "EACCES" && !elevationOffered

    pausedState: "paused-error"
    promptKey: "failure"
    title: failure ? FileFormat.operationError(failure.code, failure.message) : ""
    supportingText: failure ? failure.path : ""

    ColumnLayout {
        width: parent.width
        spacing: Theme.spacingM

        StyledText {
            Layout.fillWidth: true
            visible: root.failure !== null
            text: root.failure ? root.failure.message : ""
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
            wrapMode: Text.WordWrap
        }

        StyledText {
            Layout.fillWidth: true
            visible: root.deniedUnelevated
            text: FileOpsService.canElevate ? I18n.tr("This step cannot be retried as administrator", "file operation error dialog explanation") : FilesFormat.elevationUnavailable(FileOpsService.elevation.reason)
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
            wrapMode: Text.WordWrap
        }

        DToggle {
            Layout.fillWidth: true
            text: I18n.tr("Do this for the rest", "file conflict dialog option to reuse the answer")
            checked: root.applyToAll
            onToggled: checked => root.applyToAll = checked
        }
    }

    actions: [
        DButton {
            visible: root.offers("skip")
            text: I18n.tr("Skip", "file conflict dialog button to leave the item alone")
            variant: "tonal"
            onClicked: root.answer("skip")
        },
        DButton {
            visible: root.offers("retry")
            text: I18n.tr("Try again", "file operation error dialog button to retry")
            variant: root.elevationOffered ? "tonal" : "filled"
            onClicked: root.answer("retry")
        },
        DButton {
            visible: root.elevationOffered
            text: I18n.tr("Retry as administrator", "file operation error dialog button to retry with elevated rights")
            onClicked: root.answer("retryElevated")
        }
    ]
}
