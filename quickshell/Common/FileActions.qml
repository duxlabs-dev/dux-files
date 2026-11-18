pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    readonly property var everywhere: ["folder", "results", "recent", "trash", "picker"]
    readonly property var browsing: ["folder", "results"]
    readonly property var browsingAndRecent: ["folder", "results", "recent"]
    readonly property var browsingAndPicker: ["folder", "results", "picker"]
    readonly property var browsingRecentPicker: ["folder", "results", "recent", "picker"]

    readonly property var catalog: ({
            "open": {
                "icon": "open_in_new",
                "label": I18n.tr("Open", "context menu entry opening a file"),
                "contexts": browsingRecentPicker
            },
            "openWith": {
                "icon": "apps",
                "label": I18n.tr("Open with", "open with dialog header"),
                "contexts": browsingAndRecent
            },
            "openWithMenu": {
                "icon": "apps",
                "label": I18n.tr("Open with", "open with dialog header"),
                "contexts": browsingAndRecent,
                "submenu": true
            },
            "openFolderWith": {
                "icon": "apps",
                "label": I18n.tr("Open with", "open with dialog header"),
                "contexts": browsing,
                "submenu": true
            },
            "openInNewTab": {
                "icon": "tab",
                "label": I18n.tr("Open in new tab", "context menu entry opening a folder in a new tab"),
                "contexts": browsingAndRecent
            },
            "openInNewWindow": {
                "icon": "open_in_new",
                "label": I18n.tr("Open in new window", "context menu entry opening a folder in another window"),
                "contexts": browsingAndRecent
            },
            "openInPane": {
                "icon": "splitscreen_vertical_add",
                "label": I18n.tr("Open in the other pane", "context menu entry opening a folder beside the current one"),
                "contexts": browsingAndRecent
            },
            "openContainingFolder": {
                "icon": "folder_open",
                "label": I18n.tr("Open the containing folder", "keyboard shortcut"),
                "contexts": browsingAndRecent
            },
            "openTerminal": {
                "icon": "terminal",
                "label": I18n.tr("Open in terminal", "context menu entry opening a terminal"),
                "contexts": browsing
            },
            "bookmark": {
                "icon": "bookmark",
                "label": I18n.tr("Bookmark this folder", "tooltip on the bookmark button"),
                "contexts": browsingAndPicker
            },
            "bookmarkSelected": {
                "icon": "bookmark",
                "label": I18n.tr("Add to bookmarks", "context menu entry pinning the selected folder to the sidebar"),
                "contexts": browsingAndRecent
            },
            "removeBookmark": {
                "icon": "bookmark_remove",
                "label": I18n.tr("Remove bookmark", "tooltip on the bookmark button"),
                "contexts": browsingAndRecent
            },
            "cut": {
                "icon": "content_cut",
                "label": I18n.tr("Cut", "context menu entry marking the selection for a move"),
                "contexts": browsingRecentPicker
            },
            "copy": {
                "icon": "content_copy",
                "label": I18n.tr("Copy", "context menu entry copying the selection"),
                "contexts": browsingRecentPicker
            },
            "moveTo": {
                "icon": "drive_file_move",
                "label": I18n.tr("Move to…", "context menu entry moving the selection into a chosen folder"),
                "contexts": browsingAndRecent
            },
            "copyTo": {
                "icon": "file_copy",
                "label": I18n.tr("Copy to…", "context menu entry copying the selection into a chosen folder"),
                "contexts": browsingAndRecent
            },
            "paste": {
                "icon": "content_paste",
                "label": I18n.tr("Paste", "context menu entry pasting the clipboard here"),
                "contexts": browsingAndPicker
            },
            "pasteInto": {
                "icon": "content_paste",
                "label": I18n.tr("Paste into folder", "context menu entry pasting the clipboard into the selected folder"),
                "contexts": browsingRecentPicker
            },
            "pasteLink": {
                "icon": "add_link",
                "label": I18n.tr("Paste as link", "context menu entry creating symlinks from the clipboard"),
                "contexts": browsing
            },
            "rename": {
                "icon": "edit",
                "label": I18n.tr("Rename", "rename dialog header"),
                "contexts": browsingAndPicker
            },
            "duplicate": {
                "icon": "difference",
                "label": I18n.tr("Duplicate", "context menu entry copying the selection beside itself"),
                "contexts": browsing
            },
            "newFolder": {
                "icon": "create_new_folder",
                "label": I18n.tr("New folder", "new folder dialog header"),
                "contexts": browsingAndPicker
            },
            "newFolderWith": {
                "icon": "create_new_folder",
                "label": I18n.tr("New folder with selection", "context menu entry moving the selection into a new folder"),
                "contexts": browsing
            },
            "newDocument": {
                "icon": "note_add",
                "label": I18n.tr("New document", "context menu entry creating a file from a template"),
                "contexts": browsingAndPicker,
                "submenu": true
            },
            "compress": {
                "icon": "folder_zip",
                "label": I18n.tr("Compress…", "context menu entry packing the selection into an archive"),
                "contexts": browsing
            },
            "extract": {
                "icon": "unarchive",
                "label": I18n.tr("Extract here", "context menu entry unpacking the selected archives"),
                "contexts": browsing
            },
            "extractTo": {
                "icon": "drive_file_move",
                "label": I18n.tr("Extract to…", "context menu entry unpacking the selected archives into a chosen folder"),
                "contexts": browsingAndRecent
            },
            "trash": {
                "icon": "delete",
                "label": I18n.tr("Move to Trash", "context menu entry trashing the selection"),
                "contexts": everywhere
            },
            "deletePermanently": {
                "icon": "delete_forever",
                "label": I18n.tr("Delete permanently", "context menu entry deleting the selection for good"),
                "contexts": ["folder", "results", "recent", "trash"],
                "danger": true
            },
            "restore": {
                "icon": "restore_from_trash",
                "label": I18n.tr("Restore", "trash action returning items to where they were deleted from"),
                "contexts": ["trash"]
            },
            "emptyTrash": {
                "icon": "delete_forever",
                "label": I18n.tr("Empty Trash", "action deleting everything in the trash"),
                "contexts": ["trash"],
                "danger": true
            },
            "removeFromRecent": {
                "icon": "delete_history",
                "label": I18n.tr("Remove from Recent", "action dropping the selected files from the recently used list without touching the files"),
                "contexts": ["recent"]
            },
            "clearRecent": {
                "icon": "delete_sweep",
                "label": I18n.tr("Clear Recent", "action emptying the recently used list without touching the files"),
                "contexts": ["recent"]
            },
            "selectAll": {
                "icon": "select_all",
                "label": I18n.tr("Select all", "menu entry selecting every item in the folder"),
                "contexts": ["folder", "results", "recent", "trash"]
            },
            "showHidden": {
                "icon": "visibility",
                "label": I18n.tr("Show hidden files", "menu entry listing dot files"),
                "contexts": browsingAndPicker
            },
            "reload": {
                "icon": "refresh",
                "label": I18n.tr("Reload", "menu entry re-reading the current folder"),
                "contexts": everywhere
            },
            "copyPath": {
                "icon": "link",
                "label": I18n.tr("Copy location path", "menu entry copying the current path to the clipboard"),
                "contexts": browsingRecentPicker
            },
            "copyName": {
                "icon": "text_fields",
                "label": I18n.tr("Copy name", "context menu entry copying the selected item names to the clipboard"),
                "contexts": browsingAndRecent
            },
            "customizeFolder": {
                "icon": "palette",
                "label": I18n.tr("Folder appearance", "context menu entry opening the folder appearance dialog"),
                "contexts": browsing
            },
            "properties": {
                "icon": "info",
                "label": I18n.tr("Properties", "menu entry showing details about the current folder"),
                "contexts": browsingAndRecent
            },
            "newWindow": {
                "icon": "open_in_new",
                "label": I18n.tr("New window", "menu entry opening another file manager window"),
                "contexts": everywhere
            },
            "shortcuts": {
                "icon": "keyboard",
                "label": I18n.tr("Keyboard shortcuts", "overlay listing every keyboard shortcut"),
                "contexts": everywhere
            },
            "settings": {
                "icon": "settings",
                "label": I18n.tr("Settings", "settings window header title"),
                "contexts": everywhere
            }
        })

    function icon(id) {
        return catalog[id]?.icon ?? "";
    }

    function label(id) {
        return catalog[id]?.label ?? id;
    }

    function danger(id) {
        return catalog[id]?.danger === true;
    }

    function submenu(id) {
        return catalog[id]?.submenu === true;
    }

    function available(id, context) {
        const entry = catalog[id];
        if (!entry)
            return true;
        return entry.contexts.includes(context);
    }

    function row(id, fields) {
        return Object.assign({
            "id": id,
            "icon": icon(id),
            "label": label(id),
            "danger": danger(id),
            "submenu": submenu(id),
            "enabled": true
        }, fields ?? {});
    }
}
