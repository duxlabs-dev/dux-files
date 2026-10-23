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
    property bool shiftHeld: false
    property bool forFolder: false
    property bool selectedBookmarked: false
    property bool extractable: false
    property bool inResults: false
    property bool inTrash: false
    property bool inRecent: false
    property int trashCount: 0
    property int recentCount: 0

    signal actionTriggered(string id)

    readonly property bool crossFolder: inResults || inRecent
    readonly property var entries: {
        if (forFolder)
            return inTrash ? trashBackgroundRows : inRecent ? recentBackgroundRows : backgroundRows;
        return inTrash ? trashItemRows : itemRows;
    }

    readonly property var recentBackgroundRows: [
        {
            "id": "selectAll",
            "icon": "select_all",
            "label": I18n.tr("Select all", "menu entry selecting every item in the folder"),
            "enabled": true
        },
        {
            "id": "reload",
            "icon": "refresh",
            "label": I18n.tr("Reload", "menu entry re-reading the current folder"),
            "enabled": true
        },
        {
            "id": "separator"
        },
        {
            "id": "clearRecent",
            "icon": "delete_sweep",
            "label": I18n.tr("Clear Recent", "action emptying the recently used list without touching the files"),
            "enabled": recentCount > 0
        }
    ]

    readonly property var trashItemRows: [
        {
            "id": "restore",
            "icon": "restore_from_trash",
            "label": I18n.tr("Restore", "trash action returning items to where they were deleted from"),
            "enabled": selectionCount > 0
        },
        {
            "id": "separator"
        },
        {
            "id": "deletePermanently",
            "icon": "delete_forever",
            "label": I18n.tr("Delete permanently", "context menu entry deleting the selection for good"),
            "enabled": selectionCount > 0,
            "danger": true
        }
    ]

    readonly property var trashBackgroundRows: [
        {
            "id": "selectAll",
            "icon": "select_all",
            "label": I18n.tr("Select all", "menu entry selecting every item in the folder"),
            "enabled": true
        },
        {
            "id": "reload",
            "icon": "refresh",
            "label": I18n.tr("Reload", "menu entry re-reading the current folder"),
            "enabled": true
        },
        {
            "id": "separator"
        },
        {
            "id": "emptyTrash",
            "icon": "delete_forever",
            "label": I18n.tr("Empty Trash", "action deleting everything in the trash"),
            "enabled": trashCount > 0,
            "danger": true
        }
    ]

    readonly property var itemRows: [
        {
            "id": "open",
            "icon": "open_in_new",
            "label": I18n.tr("Open", "context menu entry opening a file"),
            "enabled": selectionCount > 0
        },
        {
            "id": "openWithMenu",
            "icon": "apps",
            "label": I18n.tr("Open with", "open with dialog header"),
            "enabled": selectionCount > 0,
            "submenu": true
        },
        {
            "id": "openInNewTab",
            "icon": "tab",
            "label": I18n.tr("Open in new tab", "context menu entry opening a folder in a new tab"),
            "enabled": hasDirectory && selectionCount === 1
        },
        {
            "id": "openInNewWindow",
            "icon": "open_in_new",
            "label": I18n.tr("Open in new window", "context menu entry opening a folder in another window"),
            "enabled": hasDirectory && selectionCount === 1
        },
        {
            "id": "openInPane",
            "icon": "splitscreen_vertical_add",
            "label": I18n.tr("Open in the other pane", "context menu entry opening a folder beside the current one"),
            "enabled": hasDirectory && selectionCount === 1
        },
        {
            "id": "bookmarkSelected",
            "icon": selectedBookmarked ? "bookmark_remove" : "bookmark",
            "label": selectedBookmarked ? I18n.tr("Remove bookmark", "tooltip on the bookmark button") : I18n.tr("Add to bookmarks", "context menu entry pinning the selected folder to the sidebar"),
            "enabled": hasDirectory && selectionCount === 1
        },
        {
            "id": "separator"
        },
        {
            "id": "cut",
            "icon": "content_cut",
            "label": I18n.tr("Cut", "context menu entry marking the selection for a move"),
            "enabled": selectionCount > 0
        },
        {
            "id": "copy",
            "icon": "content_copy",
            "label": I18n.tr("Copy", "context menu entry copying the selection"),
            "enabled": selectionCount > 0
        },
        {
            "id": "moveTo",
            "icon": "drive_file_move",
            "label": I18n.tr("Move to…", "context menu entry moving the selection into a chosen folder"),
            "enabled": selectionCount > 0
        },
        {
            "id": "copyTo",
            "icon": "file_copy",
            "label": I18n.tr("Copy to…", "context menu entry copying the selection into a chosen folder"),
            "enabled": selectionCount > 0
        },
        {
            "id": "pasteInto",
            "icon": "content_paste",
            "label": I18n.tr("Paste into folder", "context menu entry pasting the clipboard into the selected folder"),
            "enabled": hasDirectory && selectionCount === 1 && hasClipboard
        },
        {
            "id": "rename",
            "icon": "edit",
            "label": I18n.tr("Rename", "rename dialog header"),
            "enabled": selectionCount === 1,
            "visible": !inRecent
        },
        {
            "id": "duplicate",
            "icon": "difference",
            "label": I18n.tr("Duplicate", "context menu entry copying the selection beside itself"),
            "enabled": selectionCount > 0,
            "visible": !crossFolder
        },
        {
            "id": "newFolderWith",
            "icon": "create_new_folder",
            "label": I18n.tr("New folder with selection", "context menu entry moving the selection into a new folder"),
            "enabled": selectionCount > 1,
            "visible": selectionCount > 1 && !crossFolder
        },
        {
            "id": "separator"
        },
        {
            "id": "compress",
            "icon": "folder_zip",
            "label": I18n.tr("Compress…", "context menu entry packing the selection into an archive"),
            "enabled": selectionCount > 0,
            "visible": !crossFolder
        },
        {
            "id": "extract",
            "icon": "unarchive",
            "label": I18n.tr("Extract here", "context menu entry unpacking the selected archives"),
            "enabled": selectionCount > 0,
            "visible": extractable && !crossFolder
        },
        {
            "id": "extractTo",
            "icon": "drive_file_move",
            "label": I18n.tr("Extract to…", "context menu entry unpacking the selected archives into a chosen folder"),
            "enabled": selectionCount > 0,
            "visible": extractable
        },
        {
            "id": "openContainingFolder",
            "icon": "folder_open",
            "label": I18n.tr("Open the containing folder", "keyboard shortcut"),
            "enabled": selectionCount === 1,
            "visible": crossFolder
        },
        {
            "id": "separator",
            "visible": inRecent
        },
        {
            "id": "removeFromRecent",
            "icon": "delete_history",
            "label": I18n.tr("Remove from Recent", "action dropping the selected files from the recently used list without touching the files"),
            "enabled": selectionCount > 0,
            "visible": inRecent
        },
        {
            "id": "separator"
        },
        {
            "id": "trash",
            "icon": "delete",
            "label": I18n.tr("Move to Trash", "context menu entry trashing the selection"),
            "enabled": selectionCount > 0,
            "visible": !shiftHeld
        },
        {
            "id": "deletePermanently",
            "icon": "delete_forever",
            "label": I18n.tr("Delete permanently", "context menu entry deleting the selection for good"),
            "enabled": selectionCount > 0,
            "visible": shiftHeld,
            "danger": true
        },
        {
            "id": "separator"
        },
        {
            "id": "copyPath",
            "icon": "link",
            "label": I18n.tr("Copy location path", "menu entry copying the current path to the clipboard"),
            "enabled": selectionCount > 0
        },
        {
            "id": "copyName",
            "icon": "text_fields",
            "label": I18n.tr("Copy name", "context menu entry copying the selected item names to the clipboard"),
            "enabled": selectionCount > 0
        },
        {
            "id": "customizeFolder",
            "icon": "palette",
            "label": I18n.tr("Folder appearance", "context menu entry opening the folder appearance dialog"),
            "enabled": selectionCount === 1 && hasDirectory,
            "visible": selectionCount === 1 && hasDirectory && !crossFolder
        },
        {
            "id": "properties",
            "icon": "info",
            "label": I18n.tr("Properties", "menu entry showing details about the current folder"),
            "enabled": true
        }
    ]

    readonly property var backgroundRows: [
        {
            "id": "newFolder",
            "icon": "create_new_folder",
            "label": I18n.tr("New folder", "new folder dialog header"),
            "enabled": true
        },
        {
            "id": "newDocument",
            "icon": "note_add",
            "label": I18n.tr("New document", "context menu entry creating a file from a template"),
            "enabled": true,
            "submenu": true
        },
        {
            "id": "paste",
            "icon": "content_paste",
            "label": I18n.tr("Paste", "context menu entry pasting the clipboard here"),
            "enabled": hasClipboard
        },
        {
            "id": "pasteLink",
            "icon": "add_link",
            "label": I18n.tr("Paste as link", "context menu entry creating symlinks from the clipboard"),
            "enabled": hasClipboard
        },
        {
            "id": "separator"
        },
        {
            "id": "selectAll",
            "icon": "select_all",
            "label": I18n.tr("Select all", "menu entry selecting every item in the folder"),
            "enabled": true
        },
        {
            "id": "showHidden",
            "icon": "visibility",
            "label": I18n.tr("Show hidden files", "menu entry listing dot files"),
            "enabled": true
        },
        {
            "id": "reload",
            "icon": "refresh",
            "label": I18n.tr("Reload", "menu entry re-reading the current folder"),
            "enabled": true
        },
        {
            "id": "openTerminal",
            "icon": "terminal",
            "label": I18n.tr("Open in terminal", "context menu entry opening a terminal"),
            "enabled": true,
            "visible": SandboxService.available("terminal")
        },
        {
            "id": "openFolderWith",
            "icon": "apps",
            "label": I18n.tr("Open with", "open with dialog header"),
            "enabled": true,
            "submenu": true
        },
        {
            "id": "separator"
        },
        {
            "id": "properties",
            "icon": "info",
            "label": I18n.tr("Properties", "menu entry showing details about the current folder"),
            "enabled": true
        }
    ]

    Repeater {
        model: menu.entries

        Item {
            id: slot

            required property var modelData

            readonly property bool separator: modelData.id === "separator"

            width: parent?.width ?? 0
            visible: modelData.visible !== false
            height: visible ? (separator ? FileBrowserMetrics.menuDividerHeight : Theme.menuItemHeight) : 0

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
