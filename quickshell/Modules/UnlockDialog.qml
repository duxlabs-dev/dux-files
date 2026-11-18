import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var device: null
    property string errorText: ""

    signal unlocked(string mountPoint)

    function ask(target) {
        device = target;
        errorText = "";
        passphraseField.text = "";
        open();
        passphraseField.forceActiveFocus();
    }

    function submit() {
        if (passphraseField.text === "")
            return;
        DevicesService.unlock(device.id, passphraseField.text, result => {
            passphraseField.text = "";
            if (result.error) {
                root.errorText = FileFormat.operationError(result.code, result.error);
                return;
            }
            if (result.mountError)
                ToastService.info(FileFormat.operationError(result.mountCode, result.mountError));
            root.unlocked(result.mountPoint || "");
            root.close();
        });
    }

    title: I18n.tr("Unlock", "unlock dialog header for an encrypted device")
    supportingText: device ? I18n.tr("%1 is encrypted. Enter its passphrase to unlock it.", "unlock dialog explanation").arg(device.name) : ""
    onAccepted: submit()

    PasswordField {
        id: passphraseField

        isError: root.errorText !== ""
        supportingText: root.errorText
        labelText: I18n.tr("Passphrase", "label for an encrypted device passphrase input")
        onAccepted: root.submit()
    }

    actions: [
        DButton {
            text: I18n.tr("Unlock", "unlock dialog header for an encrypted device")
            onClicked: root.submit()
        }
    ]
}
