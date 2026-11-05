import QtQuick
import qs.Common
import qs.DCommon.Widgets

OverlayDialog {
    id: root

    property string acceptText: I18n.tr("Save", "dialog button to confirm a name")

    signal submitted(string name)

    function ask(name) {
        nameField.text = name || "";
        open();
        nameField.forceActiveFocus();
    }

    function submit() {
        const trimmed = nameField.text.trim();
        if (trimmed === "")
            return;
        submitted(trimmed);
        close();
    }

    onAccepted: submit()

    DTextField {
        id: nameField

        width: parent.width
        outlined: true
        labelText: I18n.tr("Name", "label for a file name input")
        onAccepted: root.submit()
    }

    actions: [
        DButton {
            text: root.acceptText
            onClicked: root.submit()
        }
    ]
}
