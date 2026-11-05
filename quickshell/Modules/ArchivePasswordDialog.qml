import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property string opScope: ""
    property bool catchUnscoped: false

    readonly property var paused: FileOpsService.jobs.filter(job => job.state === "paused-password" && (job.scope === opScope || (catchUnscoped && job.scope === "")))
    readonly property var current: paused.length > 0 ? paused[0] : null
    readonly property var prompt: current ? current.passwordPrompt : null
    readonly property bool wrong: prompt ? prompt.wrong === true : false
    readonly property string archiveName: prompt ? FilePaths.baseName(prompt.path) : ""

    function answer(action) {
        if (!current || !prompt)
            return;
        FileOpsService.respond(current.id, prompt.id, action, false);
    }

    function submit() {
        if (!current || !prompt || passwordField.text === "")
            return;
        FileOpsService.unlock(current.id, prompt.id, passwordField.text);
    }

    function offers(action) {
        return prompt ? (prompt.actions || []).includes(action) : false;
    }

    onPromptChanged: {
        passwordField.text = "";
        if (!prompt) {
            close();
            return;
        }
        open();
        passwordField.forceActiveFocus();
    }

    title: I18n.tr("Password required", "archive password dialog header")
    supportingText: prompt ? I18n.tr("%1 is password protected.", "archive password dialog explanation").arg(archiveName) : ""
    closeEnabled: false
    onAccepted: submit()

    DTextField {
        id: passwordField

        width: parent.width
        outlined: true
        echoMode: TextInput.Password
        showPasswordToggle: true
        isError: root.wrong
        supportingText: root.wrong ? I18n.tr("Wrong password", "archive password dialog error after a failed attempt") : ""
        labelText: I18n.tr("Password", "label for a password input")
        onAccepted: root.submit()
    }

    actions: [
        TextButton {
            text: I18n.tr("Cancel", "dialog button to cancel")
            onClicked: root.answer("cancel")
        },
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
