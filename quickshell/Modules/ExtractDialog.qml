import QtQuick
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var archives: []
    property var entries: []
    property bool intoFolder: true

    readonly property bool partial: entries.length > 0
    readonly property string archiveName: FilePaths.baseName(archives[0] ?? "")
    readonly property string layout: {
        if (!intoFolder)
            return "flat";
        return partial ? "folder" : "auto";
    }

    signal submitted(var archives, var entries, string dest, string layout)
    signal browseRequested(string start)

    function ask(targets, members, dest) {
        archives = targets;
        entries = members;
        intoFolder = members.length === 0;
        destField.text = dest;
        open();
        destField.forceActiveFocus();
    }

    function setDestination(path) {
        destField.text = path;
    }

    function submit() {
        const dest = FilePaths.normalize(destField.text);
        if (dest === "" || archives.length === 0)
            return;
        submitted(archives, entries, dest, layout);
        close();
    }

    title: I18n.tr("Extract", "extract dialog header")
    supportingText: {
        if (partial)
            return I18n.tr("%1 from %2", "extract dialog summary, %1 is a count of items and %2 the archive file name").arg(FileFormat.count(entries.length)).arg(archiveName);
        return archives.length === 1 ? archiveName : FileFormat.count(archives.length);
    }
    onAccepted: submit()

    DTextField {
        id: destField

        width: parent.width
        outlined: true
        labelText: I18n.tr("Extract to", "folder picker title for unpacking archives")
        onAccepted: root.submit()
        trailingContent: Component {
            DActionButton {
                focusPolicy: Qt.TabFocus
                buttonSize: destField.accessorySize
                iconName: "folder_open"
                tooltipText: I18n.tr("Choose a folder", "tooltip on the button opening a folder picker")
                onClicked: root.browseRequested(FilePaths.normalize(destField.text))
            }
        }
    }

    DToggle {
        width: parent.width
        text: I18n.tr("Extract into a new folder", "extract dialog option unpacking into a folder named after the archive")
        description: I18n.tr("Named after the archive. Turn off to put the contents straight into the chosen folder.", "extract dialog option explanation")
        checked: root.intoFolder
        onToggled: checked => root.intoFolder = checked
    }

    actions: [
        DButton {
            text: I18n.tr("Extract", "extract dialog header")
            onClicked: root.submit()
        }
    ]
}
