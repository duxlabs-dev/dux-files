import QtQuick
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var paths: []
    property string format: "zip"
    property bool protect: false

    readonly property bool canProtect: format === "zip"

    readonly property var formats: [
        {
            "value": "zip",
            "label": ".zip"
        },
        {
            "value": "tar.gz",
            "label": ".tar.gz"
        },
        {
            "value": "tar.xz",
            "label": ".tar.xz"
        },
        {
            "value": "tar.zst",
            "label": ".tar.zst"
        }
    ]

    signal submitted(var paths, string name, string format, string password)

    function ask(targets) {
        paths = targets;
        protect = false;
        passwordField.text = "";
        const base = FilePaths.baseName(targets[0] ?? "");
        nameField.text = targets.length === 1 ? base.replace(/(\.tar)?\.[^.]+$/, "") || base : I18n.tr("Archive", "default name for a new archive");
        open();
        nameField.forceActiveFocus();
        nameField.selectAll();
    }

    function submit() {
        const trimmed = nameField.text.trim();
        if (trimmed === "" || paths.length === 0)
            return;
        const password = canProtect && protect ? passwordField.text : "";
        if (canProtect && protect && password === "")
            return;
        submitted(paths, trimmed, format, password);
        close();
    }

    title: I18n.tr("Compress", "compress dialog header")
    supportingText: paths.length === 1 ? FilePaths.baseName(paths[0]) : FileFormat.count(paths.length)
    onAccepted: submit()

    DTextField {
        id: nameField

        width: parent.width
        outlined: true
        labelText: I18n.tr("Name", "label for a file name input")
        onAccepted: root.submit()
    }

    DFilterChips {
        width: parent.width
        model: root.formats
        currentIndex: Math.max(0, root.formats.findIndex(entry => entry.value === root.format))
        showCounts: false
        onSelectionChanged: index => root.format = root.formats[index].value
    }

    DToggle {
        width: parent.width
        visible: root.canProtect
        text: I18n.tr("Protect with a password", "compress dialog option encrypting the archive")
        checked: root.protect
        onToggled: checked => {
            root.protect = checked;
            if (checked)
                passwordField.forceActiveFocus();
        }
    }

    PasswordField {
        id: passwordField

        visible: root.canProtect && root.protect
        onAccepted: root.submit()
    }

    actions: [
        DButton {
            text: I18n.tr("Create", "dialog button creating an item")
            onClicked: root.submit()
        }
    ]
}
