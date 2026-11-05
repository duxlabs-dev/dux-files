import QtQuick
import QtQuick.Controls
import qs.Common
import qs.DCommon.Widgets

Popup {
    id: root

    property bool takesFocus: true
    property bool showCancel: true
    property string cancelText: I18n.tr("Cancel", "dialog button to cancel")
    property var cancelAction: () => root.close()
    property alias title: dialog.title
    property alias supportingText: dialog.supportingText
    property alias maximumWidth: dialog.maximumWidth
    property alias acceptEnabled: dialog.acceptEnabled
    property alias closeEnabled: dialog.closeEnabled
    default property alias content: dialog.content
    property alias actions: dialog.actions

    readonly property bool fieldFocused: dialog.Window.activeFocusItem instanceof TextInput

    signal accepted
    signal rejected

    parent: Overlay.overlay
    width: parent?.width ?? 0
    height: parent?.height ?? 0
    padding: 0
    modal: true
    dim: false
    focus: takesFocus
    closePolicy: Popup.NoAutoClose
    background: null

    actions: TextButton {
        visible: root.showCancel
        text: root.cancelText
        onClicked: root.cancelAction()
    }

    onAboutToShow: dialog.opened = true
    onAboutToHide: dialog.opened = false
    onRejected: close()

    exit: Transition {
        PauseAnimation {
            duration: Theme.animationsEnabled ? Theme.expressiveDurations.expressiveEffects : 0
        }
    }

    contentItem: DDialog {
        id: dialog
        embedded: false
        opened: false
        focus: root.takesFocus
        onAccepted: {
            if (root.fieldFocused)
                return;
            root.accepted();
        }
        onRejected: root.rejected()
    }
}
