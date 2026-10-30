pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Item {
    id: menus

    required property var chooser
    required property var pane

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
            pasteInto(pane.path);
            return;
        case "pasteInto":
            pasteInto(entries[0]?.path ?? pane.path);
            return;
        case "trash":
            trash(paths);
            return;
        case "copyPath":
            Paths.copyPathToClipboard((paths.length > 0 ? paths : [pane.path]).join("\n"));
            ClipboardService.clear();
            return;
        case "newFolder":
            chooser.createFolder();
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
        }
    }

    function pasteInto(dest) {
        ClipboardService.read(result => {
            if (result.paths.length === 0) {
                FileOpsService.pasteImage(dest, I18n.tr("Pasted image", "file name for an image pasted from the clipboard"), chooser.opScope);
                return;
            }
            if (result.cut) {
                FileOpsService.move(result.paths, dest, chooser.opScope);
                ClipboardService.clear();
                return;
            }
            FileOpsService.copy(result.paths, dest, chooser.opScope);
        });
    }

    function trash(paths) {
        if (paths.length === 0)
            return;
        if (!SettingsData.confirmTrash) {
            FileOpsService.trash(paths, chooser.opScope);
            return;
        }
        trashDialog.pending = paths;
        trashDialog.subject = paths.length === 1 ? FilePaths.baseName(paths[0]) : FileFormat.count(paths.length);
        trashDialog.open();
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

    ConfirmDialog {
        id: trashDialog

        property var pending: []

        title: I18n.tr("Move to Trash", "context menu entry trashing the selection")
        message: I18n.tr("%1 will be moved to the trash.", "trash confirmation explanation").arg(subject)
        confirmText: I18n.tr("Move to Trash", "context menu entry trashing the selection")
        danger: false
        onConfirmed: {
            FileOpsService.trash(pending, menus.chooser.opScope);
            pending = [];
        }
    }
}
