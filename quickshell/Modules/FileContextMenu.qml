pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FileMenu {
    id: menu

    property int selectionCount: 0
    property bool hasDirectory: false
    property bool hasClipboard: false
    property bool forFolder: false
    property bool forPicker: false
    property bool selectedBookmarked: false
    property bool extractable: false
    property bool inResults: false
    property bool inTrash: false
    property bool inRecent: false
    property int trashCount: 0
    property int recentCount: 0

    signal actionTriggered(string id)

    readonly property bool crossFolder: inResults || inRecent
    readonly property bool singleFolder: hasDirectory && selectionCount === 1
    readonly property string context: {
        if (forPicker)
            return "picker";
        if (inTrash)
            return "trash";
        if (inRecent)
            return "recent";
        return inResults ? "results" : "folder";
    }
    readonly property var entries: collapse((forFolder ? backgroundRows : itemRows).filter(row => row.visible !== false && (row.id === "separator" || FileActions.available(row.id, context))))

    readonly property var separator: ({
            "id": "separator"
        })

    readonly property var itemRows: [FileActions.row("restore", {
            "enabled": selectionCount > 0
        }), FileActions.row("open", {
            "enabled": selectionCount > 0
        }), FileActions.row("openWithMenu", {
            "enabled": selectionCount > 0
        }), FileActions.row("openInNewTab", {
            "visible": singleFolder
        }), FileActions.row("openInNewWindow", {
            "visible": singleFolder
        }), FileActions.row("openInPane", {
            "visible": singleFolder
        }), FileActions.row("bookmarkSelected", {
            "icon": FileActions.icon(selectedBookmarked ? "removeBookmark" : "bookmarkSelected"),
            "label": FileActions.label(selectedBookmarked ? "removeBookmark" : "bookmarkSelected"),
            "visible": singleFolder
        }), separator, FileActions.row("cut", {
            "enabled": selectionCount > 0
        }), FileActions.row("copy", {
            "enabled": selectionCount > 0
        }), FileActions.row("moveTo", {
            "enabled": selectionCount > 0
        }), FileActions.row("copyTo", {
            "enabled": selectionCount > 0
        }), FileActions.row("pasteInto", {
            "enabled": hasClipboard,
            "visible": singleFolder
        }), FileActions.row("rename", {
            "visible": selectionCount === 1
        }), FileActions.row("duplicate", {
            "enabled": selectionCount > 0,
            "visible": !crossFolder
        }), FileActions.row("newFolderWith", {
            "enabled": selectionCount > 1,
            "visible": selectionCount > 1 && !crossFolder
        }), separator, FileActions.row("compress", {
            "enabled": selectionCount > 0,
            "visible": !crossFolder
        }), FileActions.row("extract", {
            "enabled": selectionCount > 0,
            "visible": extractable && !crossFolder
        }), FileActions.row("extractTo", {
            "enabled": selectionCount > 0,
            "visible": extractable
        }), FileActions.row("openContainingFolder", {
            "enabled": selectionCount === 1,
            "visible": crossFolder
        }), separator, FileActions.row("removeFromRecent", {
            "enabled": selectionCount > 0
        }), separator, FileActions.row("trash", {
            "enabled": selectionCount > 0,
            "visible": !inTrash
        }), FileActions.row("deletePermanently", {
            "enabled": selectionCount > 0
        }), separator, FileActions.row("copyPath", {
            "enabled": selectionCount > 0
        }), FileActions.row("copyName", {
            "enabled": selectionCount > 0
        }), FileActions.row("customizeFolder", {
            "visible": singleFolder && !crossFolder
        }), FileActions.row("properties")]

    readonly property var backgroundRows: [FileActions.row("newFolder"), FileActions.row("newDocument"), FileActions.row("paste", {
            "enabled": hasClipboard
        }), FileActions.row("pasteLink", {
            "enabled": hasClipboard
        }), separator, FileActions.row("selectAll"), FileActions.row("showHidden"), FileActions.row("reload"), FileActions.row("openTerminal", {
            "visible": SandboxService.available("terminal")
        }), FileActions.row("openFolderWith"), separator, FileActions.row("emptyTrash", {
            "enabled": trashCount > 0
        }), FileActions.row("clearRecent", {
            "enabled": recentCount > 0
        }), FileActions.row("properties", {
            "visible": !inRecent
        })]

    function collapse(rows) {
        const out = [];
        for (const row of rows) {
            const isSeparator = row.id === "separator";
            if (isSeparator && (out.length === 0 || out[out.length - 1].id === "separator"))
                continue;
            out.push(row);
        }
        if (out.length > 0 && out[out.length - 1].id === "separator")
            out.pop();
        return out;
    }

    Repeater {
        model: menu.entries

        Item {
            id: slot

            required property var modelData

            readonly property bool separator: modelData.id === "separator"

            width: parent?.width ?? 0
            height: separator ? FileBrowserMetrics.menuDividerHeight : Theme.menuItemHeight

            FileMenuDivider {
                visible: slot.separator
            }

            FileMenuItem {
                anchors.fill: parent
                visible: !slot.separator
                text: slot.separator ? "" : slot.modelData.label
                iconName: slot.separator ? "" : slot.modelData.icon
                danger: slot.modelData.danger === true
                submenu: slot.modelData.submenu === true
                enabled: !slot.separator && slot.modelData.enabled
                onClicked: {
                    menu.close();
                    menu.actionTriggered(slot.modelData.id);
                }
            }
        }
    }
}
