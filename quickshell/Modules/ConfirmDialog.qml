import QtQuick
import qs.Common
import qs.DCommon.Widgets

OverlayDialog {
    id: root

    property string message: ""
    property string subject: ""
    property string confirmText: I18n.tr("Delete", "confirm dialog default confirm button")
    property bool danger: true

    signal confirmed

    function confirm() {
        confirmed();
        close();
    }

    supportingText: message
    onAccepted: confirm()

    actions: [
        DButton {
            text: root.confirmText
            variant: root.danger ? "tonal" : "filled"
            danger: root.danger
            onClicked: root.confirm()
        }
    ]
}
