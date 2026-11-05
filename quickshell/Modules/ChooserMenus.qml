pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services

Item {
    id: menus

    required property var chooser
    required property var pane
    required property PaneActions actions

    function showItemMenu(pointX, pointY) {
        if (pane.virtualActive)
            return;
        backgroundMenu.close();
        const point = pane.mapToItem(menus, pointX, pointY);
        itemMenu.live = true;
        itemMenu.openAt(point.x, point.y);
    }

    function showBackgroundMenu(pointX, pointY) {
        if (pane.virtualActive)
            return;
        itemMenu.close();
        const point = pane.mapToItem(menus, pointX, pointY);
        backgroundMenu.openAt(point.x, point.y);
    }

    function runAction(id) {
        if (!FileActions.available(id, "picker"))
            return;
        const entries = pane.selectedEntries;
        const paths = entries.map(entry => entry.path);
        switch (id) {
        case "open":
            chooser.openEntries(entries);
            return;
        case "rename":
            if (entries.length === 1)
                pane.beginRename(entries[0].path);
            return;
        case "cut":
            ClipboardService.cutPaths(paths);
            return;
        case "copy":
            ClipboardService.copy(paths);
            return;
        case "paste":
            actions.pasteInto(pane.path, false);
            return;
        case "pasteInto":
            actions.pasteInto(entries[0]?.path ?? pane.path, false);
            return;
        case "trash":
            actions.trashPaths(paths);
            return;
        case "copyPath":
            actions.copyText((paths.length > 0 ? paths : [pane.path]).join("\n"));
            return;
        case "newFolder":
            actions.newFolder(pane.virtualActive ? "" : pane.path);
            return;
        case "newDocument":
            newDocumentMenu.present(backgroundMenu.x, backgroundMenu.y);
            return;
        case "showHidden":
            chooser.setShowHidden(!chooser.showHidden);
            return;
        case "reload":
            pane.refresh();
            return;
        case "selectAll":
            pane.selection.selectAll(pane.body.directory.entries);
            return;
        case "editPath":
            pane.editPath();
            return;
        case "jump":
            pane.startJump();
            return;
        case "search":
            chooser.focusSearch();
            return;
        case "back":
            pane.back();
            return;
        case "forward":
            pane.forward();
            return;
        case "togglePreview":
            chooser.setPreviewOpen(!chooser.previewOpen);
            return;
        case "toggleSidebar":
            chooser.setSidebarShown(!chooser.sidebarShown);
            return;
        }
    }

    FileContextMenu {
        id: itemMenu

        property bool live: false

        anchorItem: menus
        forPicker: true
        selectionCount: live ? menus.pane.selectedEntries.length : 0
        hasDirectory: live && menus.pane.selectedEntries.length > 0 && menus.pane.selectedEntries.every(entry => entry.isDir)
        hasClipboard: live && ClipboardService.hasFiles
        onActionTriggered: id => menus.runAction(id)
        onClosed: live = false
    }

    FileContextMenu {
        id: backgroundMenu

        anchorItem: menus
        forPicker: true
        forFolder: true
        hasClipboard: ClipboardService.hasFiles
        onActionTriggered: id => menus.runAction(id)
    }

    NewDocumentMenu {
        id: newDocumentMenu

        anchorItem: menus
        folder: menus.pane.path
        opScope: menus.chooser.opScope
    }
}
