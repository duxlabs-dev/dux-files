pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    // "view" and "sidebar" keys are handled by the focused item so they never fire inside a text field.
    readonly property var entries: [
        {
            "id": "open",
            "keys": ["Return", "Enter"],
            "group": "navigation",
            "scope": "view",
            "label": I18n.tr("Open", "context menu entry opening a file"),
            "vim": ["l"]
        },
        {
            "id": "openInNewTab",
            "keys": ["Shift+Return"],
            "group": "navigation",
            "scope": "selection",
            "label": I18n.tr("Open in new tab", "context menu entry opening a folder in a new tab")
        },
        {
            "id": "openInPane",
            "keys": ["Ctrl+Shift+Return"],
            "group": "navigation",
            "scope": "selection",
            "label": I18n.tr("Open in the other pane", "context menu entry opening a folder beside the current one")
        },
        {
            "id": "openContainingFolder",
            "keys": ["Ctrl+Return"],
            "group": "navigation",
            "scope": "selection",
            "label": I18n.tr("Open the containing folder", "keyboard shortcut")
        },
        {
            "id": "openWith",
            "keys": ["Ctrl+Shift+O"],
            "group": "navigation",
            "scope": "selection",
            "label": I18n.tr("Open with", "open with dialog header")
        },
        {
            "id": "openTerminal",
            "keys": ["Ctrl+Alt+T"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Open in terminal", "context menu entry opening a terminal")
        },
        {
            "id": "moveCursor",
            "keys": ["Up", "Down", "Left", "Right"],
            "group": "navigation",
            "scope": "view",
            "label": I18n.tr("Move through the folder", "keyboard shortcut"),
            "vim": ["j", "k"]
        },
        {
            "id": "cursorEdge",
            "keys": ["Home", "End", "PgUp", "PgDown"],
            "group": "navigation",
            "scope": "view",
            "label": I18n.tr("Jump to the start, the end or the next screen", "keyboard shortcut"),
            "vim": ["gg", "G"]
        },
        {
            "id": "typeAhead",
            "keys": ["A-Z"],
            "group": "navigation",
            "scope": "view",
            "label": I18n.tr("Jump to a name or filter the folder with what you type", "keyboard shortcut"),
            "plainOnly": true
        },
        {
            "id": "up",
            "keys": ["Alt+Up", "Backspace"],
            "group": "navigation",
            "scope": "view",
            "label": I18n.tr("Go to the parent folder", "keyboard shortcut"),
            "vim": ["h"]
        },
        {
            "id": "home",
            "keys": ["Alt+Home"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Go to the home folder", "keyboard shortcut"),
            "vim": ["gh"]
        },
        {
            "id": "back",
            "keys": ["Alt+Left"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Back", "tooltip on the button going to the previous location"),
            "vim": ["H"]
        },
        {
            "id": "forward",
            "keys": ["Alt+Right"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Forward", "tooltip on the button going to the next location"),
            "vim": ["L"]
        },
        {
            "id": "bookmark",
            "keys": ["Ctrl+D"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Bookmark this folder", "tooltip on the bookmark button")
        },
        {
            "id": "editPath",
            "keys": ["Ctrl+L"],
            "group": "navigation",
            "scope": "path",
            "label": I18n.tr("Type a location", "keyboard shortcut")
        },
        {
            "id": "jump",
            "keys": ["Ctrl+Shift+K"],
            "group": "navigation",
            "scope": "path",
            "label": I18n.tr("Jump to a folder you visited", "keyboard shortcut")
        },
        {
            "id": "search",
            "keys": ["Ctrl+F", "/"],
            "group": "navigation",
            "scope": "path",
            "label": I18n.tr("Search in Files", "placeholder in the search field")
        },
        {
            "id": "searchHome",
            "keys": ["Ctrl+Shift+F"],
            "group": "navigation",
            "scope": "path",
            "label": I18n.tr("Search your home folder", "keyboard shortcut opening search with the Home scope")
        },
        {
            "id": "searchFilters",
            "keys": ["Alt+F"],
            "group": "navigation",
            "scope": "path",
            "label": I18n.tr("Move to the search filters", "keyboard shortcut focusing the chips above search results")
        },
        {
            "id": "newWindow",
            "keys": ["Ctrl+N"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("New window", "menu entry opening another file manager window")
        },
        {
            "id": "newTab",
            "keys": ["Ctrl+T"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("New tab", "context menu entry opening a folder in a new tab"),
            "vim": ["t"]
        },
        {
            "id": "closeTab",
            "keys": ["Ctrl+W"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Close tab", "tooltip on the button closing a tab")
        },
        {
            "id": "reopenTab",
            "keys": ["Ctrl+Shift+T"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Reopen the closed tab", "keyboard shortcut")
        },
        {
            "id": "nextTab",
            "keys": ["Ctrl+Tab", "Ctrl+PgDown"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Next tab", "keyboard shortcut"),
            "vim": ["gt"]
        },
        {
            "id": "previousTab",
            "keys": ["Ctrl+Shift+Tab", "Ctrl+PgUp"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Previous tab", "keyboard shortcut"),
            "vim": ["gT"]
        },
        {
            "id": "tabAt",
            "keys": ["Alt+1-9"],
            "group": "navigation",
            "scope": "tabs",
            "label": I18n.tr("Go to that tab", "keyboard shortcut")
        },
        {
            "id": "moveTabLeft",
            "keys": ["Ctrl+Shift+PgUp"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Move the tab left", "keyboard shortcut moving the tab toward the start of the tab row")
        },
        {
            "id": "moveTabRight",
            "keys": ["Ctrl+Shift+PgDown"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Move the tab right", "keyboard shortcut moving the tab toward the end of the tab row")
        },
        {
            "id": "splitView",
            "keys": ["F3"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Split view", "tooltip on the button showing a second pane")
        },
        {
            "id": "swapPane",
            "keys": ["F6"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Switch pane", "keyboard shortcut")
        },
        {
            "id": "toggleSidebar",
            "keys": ["F9"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Toggle sidebar", "tooltip on the button showing or hiding the sidebar")
        },
        {
            "id": "focusSidebar",
            "keys": ["Ctrl+B"],
            "group": "navigation",
            "scope": "window",
            "label": I18n.tr("Focus the sidebar", "keyboard shortcut"),
            "vim": ["s"]
        },
        {
            "id": "listView",
            "keys": ["Ctrl+1"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("List view", "view mode showing one row per item")
        },
        {
            "id": "gridView",
            "keys": ["Ctrl+2"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Grid view", "view mode showing items as tiles")
        },
        {
            "id": "columnsView",
            "keys": ["Ctrl+3"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Column view", "view mode showing folders side by side")
        },
        {
            "id": "zoomIn",
            "keys": ["Ctrl++", "Ctrl+="],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Larger icons", "keyboard shortcut")
        },
        {
            "id": "zoomOut",
            "keys": ["Ctrl+-"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Smaller icons", "keyboard shortcut")
        },
        {
            "id": "zoomReset",
            "keys": ["Ctrl+0"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Default icon size", "keyboard shortcut")
        },
        {
            "id": "zoomWheel",
            "keys": ["Ctrl+Wheel"],
            "group": "view",
            "scope": "view",
            "label": I18n.tr("Resize icons", "keyboard shortcut")
        },
        {
            "id": "sortKey",
            "keys": ["Ctrl+Shift+S"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Sort by the next column", "keyboard shortcut cycling the sort criterion")
        },
        {
            "id": "sortDirection",
            "keys": ["Ctrl+Shift+R"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Reverse the sort order", "keyboard shortcut flipping between ascending and descending")
        },
        {
            "id": "showHidden",
            "keys": ["Ctrl+H"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Show hidden files", "menu entry listing dot files"),
            "vim": ["."]
        },
        {
            "id": "togglePreview",
            "keys": ["Space"],
            "group": "view",
            "scope": "view",
            "label": I18n.tr("Preview", "view menu entry and shortcut showing or hiding the preview drawer")
        },
        {
            "id": "previewExpand",
            "keys": ["F4"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Open the preview in a large view", "keyboard shortcut opening the preview overlay")
        },
        {
            "id": "previewStep",
            "keys": ["Shift+Space", "Ctrl+Shift+Space"],
            "group": "view",
            "scope": "view",
            "label": I18n.tr("Preview the next or previous selected item", "keyboard shortcut")
        },
        {
            "id": "previewScroll",
            "keys": ["Alt+PgUp", "Alt+PgDown"],
            "group": "view",
            "scope": "view",
            "label": I18n.tr("Scroll the preview", "keyboard shortcut")
        },
        {
            "id": "previewResize",
            "keys": ["Ctrl+Alt+Left", "Ctrl+Alt+Right", "Ctrl+Alt+0"],
            "group": "view",
            "scope": "view",
            "label": I18n.tr("Resize the preview, or reset its width", "keyboard shortcut")
        },
        {
            "id": "reload",
            "keys": ["F5", "Ctrl+R"],
            "group": "view",
            "scope": "window",
            "label": I18n.tr("Reload", "menu entry re-reading the current folder")
        },
        {
            "id": "shortcuts",
            "keys": ["F1", "?"],
            "group": "view",
            "scope": "path",
            "label": I18n.tr("Keyboard shortcuts", "overlay listing every keyboard shortcut")
        },
        {
            "id": "settings",
            "keys": ["Ctrl+,"],
            "group": "view",
            "scope": "path",
            "label": I18n.tr("Settings", "settings window header title")
        },
        {
            "id": "selectAll",
            "keys": ["Ctrl+A"],
            "group": "selection",
            "scope": "window",
            "label": I18n.tr("Select all", "menu entry selecting every item in the folder")
        },
        {
            "id": "invertSelection",
            "keys": ["Ctrl+Shift+A"],
            "group": "selection",
            "scope": "window",
            "label": I18n.tr("Invert the selection", "keyboard shortcut")
        },
        {
            "id": "toggleItem",
            "keys": ["Ctrl+Space"],
            "group": "selection",
            "scope": "view",
            "label": I18n.tr("Add or remove the item under the cursor", "keyboard shortcut"),
            "vim": ["v"]
        },
        {
            "id": "extendSelection",
            "keys": ["Shift+Click", "Shift+Arrows"],
            "group": "selection",
            "scope": "view",
            "label": I18n.tr("Select a range", "keyboard shortcut"),
            "vim": ["J", "K"]
        },
        {
            "id": "toggleWithPointer",
            "keys": ["Ctrl+Click"],
            "group": "selection",
            "scope": "view",
            "label": I18n.tr("Add or remove one item", "keyboard shortcut")
        },
        {
            "id": "clearSelection",
            "keys": ["Esc"],
            "group": "selection",
            "scope": "view",
            "label": I18n.tr("Clear the selection", "keyboard shortcut")
        },
        {
            "id": "copy",
            "keys": ["Ctrl+C"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Copy", "context menu entry copying the selection"),
            "vim": ["y"]
        },
        {
            "id": "cut",
            "keys": ["Ctrl+X"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Cut", "context menu entry marking the selection for a move"),
            "vim": ["x"]
        },
        {
            "id": "paste",
            "keys": ["Ctrl+V"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Paste", "context menu entry pasting the clipboard here"),
            "vim": ["p"]
        },
        {
            "id": "pasteLink",
            "keys": ["Ctrl+Shift+V"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Paste as link", "context menu entry creating symlinks from the clipboard")
        },
        {
            "id": "rename",
            "keys": ["F2"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Rename", "rename dialog header"),
            "vim": ["r"]
        },
        {
            "id": "trash",
            "keys": ["Del"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Move to Trash", "context menu entry trashing the selection"),
            "vim": ["d"]
        },
        {
            "id": "deletePermanently",
            "keys": ["Shift+Del"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Delete permanently", "context menu entry deleting the selection for good"),
            "vim": ["D"]
        },
        {
            "id": "undo",
            "keys": ["Ctrl+Z"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Undo", "toast action reversing the last operation"),
            "vim": ["u"]
        },
        {
            "id": "redo",
            "keys": ["Ctrl+Shift+Z", "Ctrl+Y"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Redo", "toast action repeating the operation that was just undone")
        },
        {
            "id": "duplicate",
            "keys": ["Ctrl+Shift+D"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("Duplicate", "context menu entry copying the selection beside itself")
        },
        {
            "id": "newFolder",
            "keys": ["Ctrl+Shift+N"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("New folder", "new folder dialog header")
        },
        {
            "id": "newFolderWith",
            "keys": ["Ctrl+Alt+N"],
            "group": "operations",
            "scope": "selection",
            "label": I18n.tr("New folder with selection", "context menu entry moving the selection into a new folder")
        },
        {
            "id": "copyPath",
            "keys": ["Ctrl+Shift+C"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Copy location path", "menu entry copying the current path to the clipboard")
        },
        {
            "id": "properties",
            "keys": ["Ctrl+I", "Alt+Return"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Properties", "menu entry showing details about the current folder")
        },
        {
            "id": "contextMenu",
            "keys": ["Menu", "Shift+F10"],
            "group": "operations",
            "scope": "window",
            "label": I18n.tr("Open the context menu", "keyboard shortcut")
        },
        {
            "id": "sidebarMove",
            "keys": ["Up", "Down"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Move through the sidebar", "keyboard shortcut"),
            "vim": ["j", "k"]
        },
        {
            "id": "sidebarOpen",
            "keys": ["Return", "Space"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Open the place", "keyboard shortcut")
        },
        {
            "id": "sidebarAction",
            "keys": ["Del"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Remove the bookmark or eject the drive", "keyboard shortcut")
        },
        {
            "id": "sidebarRename",
            "keys": ["F2"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Rename the bookmark", "keyboard shortcut")
        },
        {
            "id": "sidebarMenu",
            "keys": ["Menu", "Shift+F10"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Open the bookmark menu", "keyboard shortcut")
        },
        {
            "id": "sidebarLeave",
            "keys": ["Esc"],
            "group": "sidebar",
            "scope": "sidebar",
            "label": I18n.tr("Back to the files", "keyboard shortcut")
        }
    ]

    readonly property var groups: [
        {
            "id": "navigation",
            "label": I18n.tr("Getting around", "keyboard shortcut group")
        },
        {
            "id": "view",
            "label": I18n.tr("View", "keyboard shortcut group")
        },
        {
            "id": "selection",
            "label": I18n.tr("Selection", "keyboard shortcut group")
        },
        {
            "id": "operations",
            "label": I18n.tr("Files", "keyboard shortcut group")
        },
        {
            "id": "sidebar",
            "label": I18n.tr("Sidebar", "keyboard shortcut group")
        }
    ]

    function bound(scope) {
        return entries.filter(entry => entry.scope === scope);
    }

    function inGroup(group, vimKeys) {
        return entries.filter(entry => entry.group === group && !(vimKeys && entry.plainOnly === true));
    }

    function keysFor(entry, vimKeys) {
        return vimKeys && entry.vim ? entry.keys.concat(entry.vim) : entry.keys;
    }
}
