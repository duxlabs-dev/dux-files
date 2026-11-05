import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

PausedJobDialog {
    id: root

    readonly property bool wrong: prompt ? prompt.wrong === true : false
    readonly property string archiveName: prompt ? FilePaths.baseName(prompt.path) : ""

    function submit() {
        if (!current || !prompt || passwordField.text === "")
            return;
        FileOpsService.unlock(current.id, prompt.id, passwordField.text);
    }

    onPromptChanged: {
        passwordField.text = "";
        if (prompt)
            passwordField.forceActiveFocus();
    }

    pausedState: "paused-password"
    promptKey: "passwordPrompt"
    title: I18n.tr("Password required", "archive password dialog header")
    supportingText: prompt ? I18n.tr("%1 is password protected.", "archive password dialog explanation").arg(archiveName) : ""
    onAccepted: submit()

    PasswordField {
        id: passwordField

        isError: root.wrong
        supportingText: root.wrong ? I18n.tr("Wrong password", "archive password dialog error after a failed attempt") : ""
        onAccepted: root.submit()
    }

    actions: [
        DButton {
            visible: root.offers("skip")
            text: I18n.tr("Skip", "file conflict dialog button to leave the item alone")
            variant: "tonal"
            onClicked: root.answer("skip")
        },
        DButton {
            text: I18n.tr("Unlock", "unlock dialog header for an encrypted device")
            onClicked: root.submit()
        }
    ]
}
