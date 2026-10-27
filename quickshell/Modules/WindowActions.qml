import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

QtObject {
    id: actions

    required property var mainWindow

    property string vimPrefix: ""

    readonly property var activePane: mainWindow.activePane
    readonly property string currentPath: mainWindow.currentPath
    readonly property string opScope: mainWindow.opScope

    readonly property Timer vimPrefixTimer: Timer {
        interval: FileBrowserMetrics.typeAheadInterval
        onTriggered: actions.vimPrefix = ""
    }

    function trashAction(id) {
        switch (id) {
        case "trash":
            return "deletePermanently";
        case "openWith":
        case "openWithMenu":
        case "openFolderWith":
        case "openContainingFolder":
        case "openInNewTab":
        case "openInNewWindow":
        case "openInPane":
        case "openTerminal":
        case "cut":
        case "copy":
        case "paste":
        case "pasteInto":
        case "pasteLink":
        case "rename":
        case "duplicate":
        case "moveTo":
        case "copyTo":
        case "newFolder":
        case "newFolderWith":
        case "newDocument":
        case "compress":
        case "extract":
        case "extractTo":
        case "properties":
        case "customizeFolder":
        case "copyPath":
        case "bookmark":
        case "bookmarkSelected":
            return "";
        default:
            return id;
        }
    }

    function recentAction(id) {
        switch (id) {
        case "openTerminal":
        case "openFolderWith":
        case "paste":
        case "pasteLink":
        case "rename":
        case "duplicate":
        case "newFolder":
        case "newFolderWith":
        case "newDocument":
        case "compress":
        case "extract":
        case "bookmark":
            return "";
        case "pasteInto":
            return mainWindow.selectedPaths().length === 1 ? id : "";
        case "copyPath":
        case "properties":
            return mainWindow.selectedPaths().length > 0 ? id : "";
        default:
            return id;
        }
    }

    function scopedAction(id) {
        if (activePane.trashActive)
            return trashAction(id);
        if (activePane.recentActive)
            return recentAction(id);
        return id;
    }

    function runAction(id) {
        switch (scopedAction(id)) {
        case "restore":
            mainWindow.openFlow.restoreEntries(mainWindow.selectedEntries());
            return;
        case "emptyTrash":
            mainWindow.dialogs.confirmEmptyTrash();
            return;
        case "removeFromRecent":
            RecentService.remove(mainWindow.selectedPaths());
            return;
        case "clearRecent":
            mainWindow.dialogs.confirmClearRecent();
            return;
        case "open":
            mainWindow.openFlow.openEntries(mainWindow.selectedEntries());
            return;
        case "openContainingFolder":
            openContainingFolder();
            return;
        case "openWith":
            openWithCurrent();
            return;
        case "openWithMenu":
            mainWindow.menus.showOpenWithMenu([], mainWindow.selectedPaths(), "item");
            return;
        case "openFolderWith":
            mainWindow.menus.showOpenWithMenu(["inode/directory"], [currentPath], "background");
            return;
        case "openTerminal":
            openTerminalHere();
            return;
        case "newTab":
            mainWindow.tabManager.newTab("", false);
            return;
        case "newWindow":
            newWindowAt(currentPath);
            return;
        case "openInNewWindow":
            openSelectionInNewWindow();
            return;
        case "openInNewTab":
            openSelectionInNewTab();
            return;
        case "reopenTab":
            mainWindow.tabManager.reopenTab();
            return;
        case "moveTabLeft":
            mainWindow.tabManager.moveTab(mainWindow.tabManager.currentTab, mainWindow.tabManager.currentTab - 1);
            return;
        case "moveTabRight":
            mainWindow.tabManager.moveTab(mainWindow.tabManager.currentTab, mainWindow.tabManager.currentTab + 1);
            return;
        case "home":
            mainWindow.openPath(Paths.strip(Paths.home), false);
            return;
        case "splitView":
            mainWindow.toggleSplit();
            return;
        case "toggleSidebar":
            mainWindow.sidebar.toggle();
            return;
        case "focusSidebar":
            mainWindow.sidebar.takeFocus();
            return;
        case "bookmark":
            BookmarksService.toggle(currentPath);
            return;
        case "bookmarkSelected":
            {
                const folder = mainWindow.selectedEntries().find(entry => entry.isDir);
                if (folder)
                    BookmarksService.toggle(folder.path);
                return;
            }
        case "zoomReset":
            activePane.setZoom(activePane.viewMode, FilesMetrics.defaultZoom(activePane.viewMode));
            return;
        case "openInPane":
            openInOtherPane(mainWindow.firstSelected()?.path ?? "");
            return;
        case "closeTab":
            mainWindow.tabManager.closeTab(mainWindow.tabManager.currentTab);
            return;
        case "nextTab":
            mainWindow.tabManager.cycleTab(1);
            return;
        case "previousTab":
            mainWindow.tabManager.cycleTab(-1);
            return;
        case "back":
            activePane.back();
            return;
        case "forward":
            activePane.forward();
            return;
        case "editPath":
            mainWindow.topbar.focusLocation();
            return;
        case "jump":
            mainWindow.topbar.focusSearch();
            return;
        case "search":
            mainWindow.topbar.focusSearch();
            return;
        case "sortKey":
            mainWindow.topbar.cycleSortKey();
            return;
        case "sortDirection":
            activePane.flipSort();
            return;
        case "searchFilters":
            activePane.focusSearchChips();
            return;
        case "searchHome":
            activePane.setSearchScope("home");
            if (activePane.nameFilter.trim() !== "") {
                activePane.startSearch(activePane.nameFilter.trim(), null);
                return;
            }
            mainWindow.topbar.focusSearch();
            return;
        case "settings":
            mainWindow.dialogs.openSettings();
            return;
        case "compress":
            mainWindow.dialogs.askCompress(mainWindow.selectedPaths());
            return;
        case "extract":
            mainWindow.openFlow.extractTo(mainWindow.selectedPaths(), currentPath, "reveal");
            return;
        case "extractTo":
            mainWindow.dialogs.askExtractTo(mainWindow.selectedPaths());
            return;
        case "newDocument":
            mainWindow.menus.showNewDocumentMenu();
            return;
        case "swapPane":
            swapPaneFocus();
            return;
        case "listView":
            activePane.setViewMode("list");
            return;
        case "gridView":
            activePane.setViewMode("grid");
            return;
        case "columnsView":
            activePane.setViewMode("columns");
            return;
        case "togglePreview":
            togglePreview();
            return;
        case "previewExpand":
            mainWindow.preview.present(mainWindow.previewTarget);
            return;
        case "zoomIn":
            activePane.body.zoomBy(1);
            return;
        case "zoomOut":
            activePane.body.zoomBy(-1);
            return;
        case "showHidden":
            mainWindow.setShowHidden(!mainWindow.showHidden);
            return;
        case "reload":
            activePane.refresh();
            return;
        case "shortcuts":
            mainWindow.dialogs.toggleShortcuts();
            return;
        case "selectAll":
            activePane.selection.selectAll(activePane.body.directory.entries);
            return;
        case "invertSelection":
            activePane.selection.invert(activePane.body.directory.entries);
            return;
        case "copy":
            ClipboardService.copy(mainWindow.selectedPaths());
            return;
        case "cut":
            ClipboardService.cutPaths(mainWindow.selectedPaths());
            return;
        case "paste":
            pasteInto(currentPath, false);
            return;
        case "pasteInto":
            pasteInto(mainWindow.firstSelected()?.path ?? currentPath, false);
            return;
        case "pasteLink":
            pasteInto(currentPath, true);
            return;
        case "rename":
            renameSelection();
            return;
        case "trash":
            trashSelection();
            return;
        case "deletePermanently":
            deleteSelection();
            return;
        case "undo":
            FileOpsService.undo(opScope);
            return;
        case "redo":
            FileOpsService.redo(opScope);
            return;
        case "duplicate":
            duplicateSelection();
            return;
        case "moveTo":
            mainWindow.dialogs.askTransferTo(mainWindow.selectedPaths(), true);
            return;
        case "copyTo":
            mainWindow.dialogs.askTransferTo(mainWindow.selectedPaths(), false);
            return;
        case "newFolder":
            FileOpsService.newFolder(currentPath, I18n.tr("Untitled folder", "default name for a new folder"), opScope, true);
            return;
        case "newFolderWith":
            newFolderWithSelection();
            return;
        case "properties":
            showProperties();
            return;
        case "customizeFolder":
            mainWindow.dialogs.showAppearance(mainWindow.selectedEntries());
            return;
        case "copyPath":
            copyText((mainWindow.selectedPaths().length > 0 ? mainWindow.selectedPaths() : [currentPath]).join("\n"));
            return;
        case "copyName":
            copyText(mainWindow.selectedEntries().map(entry => entry.name).join("\n"));
            return;
        case "contextMenu":
            mainWindow.menus.showContextMenuAtCursor();
            return;
        }
    }

    function copyText(text) {
        Paths.copyPathToClipboard(text);
        ClipboardService.clear();
    }

    function openSelectionInNewTab() {
        const entry = mainWindow.firstSelected();
        if (entry?.isDir)
            mainWindow.tabManager.newTab(entry.path, false);
    }

    function openSelectionInNewWindow() {
        const entry = mainWindow.firstSelected();
        if (entry?.isDir)
            newWindowAt(entry.path);
    }

    function handleVimKey(event) {
        if (!SettingsData.vimKeys || (event.modifiers & ~(Qt.ShiftModifier | Qt.KeypadModifier)) !== 0)
            return false;
        const shift = (event.modifiers & Qt.ShiftModifier) !== 0;
        const prefix = vimPrefix;
        vimPrefix = "";
        if (prefix === "g")
            return runPrefixedVimKey(event.key, shift);
        const body = activePane.body;
        const columns = body.view?.columnsPerRow ?? 1;
        switch (event.key) {
        case Qt.Key_J:
            body.moveCursor(columns, event.modifiers);
            return true;
        case Qt.Key_K:
            body.moveCursor(-columns, event.modifiers);
            return true;
        case Qt.Key_H:
            if (shift)
                activePane.back();
            else
                activePane.up();
            return true;
        case Qt.Key_L:
            if (shift)
                activePane.forward();
            else
                activePane.openSelection();
            return true;
        case Qt.Key_G:
            if (shift) {
                body.jumpTo(body.directory.count - 1, Qt.NoModifier);
                return true;
            }
            vimPrefix = "g";
            vimPrefixTimer.restart();
            return true;
        case Qt.Key_V:
            body.toggleAtCursor();
            return true;
        case Qt.Key_Y:
            return runOnSelection("copy");
        case Qt.Key_X:
            return runOnSelection("cut");
        case Qt.Key_P:
            runAction("paste");
            return true;
        case Qt.Key_D:
            return runOnSelection(shift ? "deletePermanently" : "trash");
        case Qt.Key_R:
            return runOnSelection("rename");
        case Qt.Key_U:
            runAction("undo");
            return true;
        case Qt.Key_Period:
            runAction("showHidden");
            return true;
        case Qt.Key_T:
            runAction("newTab");
            return true;
        case Qt.Key_S:
            runAction("focusSidebar");
            return true;
        }
        return false;
    }

    function runPrefixedVimKey(key, shift) {
        switch (key) {
        case Qt.Key_G:
            if (shift)
                return false;
            activePane.body.jumpTo(0, Qt.NoModifier);
            return true;
        case Qt.Key_T:
            mainWindow.tabManager.cycleTab(shift ? -1 : 1);
            return true;
        case Qt.Key_H:
            if (shift)
                return false;
            runAction("home");
            return true;
        }
        return false;
    }

    function runOnSelection(id) {
        if (mainWindow.selectedPaths().length > 0)
            runAction(id);
        return true;
    }

    function newWindowAt(path) {
        mainWindow.newWindowRequested([
            {
                "kind": "folder",
                "path": path,
                "select": []
            }
        ]);
    }

    function swapPaneFocus() {
        if (!mainWindow.splitView || !mainWindow.secondPane)
            return;
        mainWindow.activePaneIndex = mainWindow.activePaneIndex === 0 ? 1 : 0;
        mainWindow.activePane.focusBody();
    }

    function togglePreview() {
        const open = !mainWindow.previewOpen;
        mainWindow.previewOpen = open;
        SessionData.remember({
            "previewOpen": open
        });
    }

    function openInOtherPane(path) {
        if (path === "")
            return;
        if (!mainWindow.splitView)
            mainWindow.toggleSplit();
        const target = mainWindow.panes[mainWindow.activePaneIndex === 0 ? 1 : 0];
        target?.navigate(path);
    }

    function duplicateSelection() {
        const paths = mainWindow.selectedPaths();
        if (paths.length === 0 || activePane.resultsActive)
            return;
        FileOpsService.duplicate(paths, opScope);
    }

    function newFolderWithSelection() {
        const paths = mainWindow.selectedPaths();
        if (paths.length === 0 || activePane.resultsActive)
            return;
        FileOpsService.newFolderWith(paths, currentPath, I18n.tr("Untitled folder", "default name for a new folder"), opScope, true);
    }

    function openWithCurrent() {
        const paths = mainWindow.selectedPaths();
        if (paths.length === 0)
            return;
        mainWindow.dialogs.askOpenWith([], paths);
    }

    function openContainingFolder() {
        const entry = mainWindow.firstSelected();
        if (!entry)
            return;
        activePane.reveal(entry.path);
    }

    function openTerminalHere() {
        AppsService.openTerminal(currentPath, response => mainWindow.openFlow.reportError(response));
    }

    function trashSelection() {
        const paths = mainWindow.selectedPaths();
        trashPaths(paths, paths.length === 1 ? mainWindow.firstSelected().name : FileFormat.count(paths.length));
    }

    function trashPaths(paths, subject) {
        if (paths.length === 0)
            return;
        if (!SettingsData.confirmTrash) {
            FileOpsService.trash(paths, opScope);
            return;
        }
        mainWindow.dialogs.confirmTrash(paths, subject);
    }

    function deleteSelection() {
        const paths = mainWindow.selectedPaths();
        if (paths.length === 0)
            return;
        mainWindow.dialogs.confirmDelete(paths, paths.length === 1 ? mainWindow.firstSelected().name : FileFormat.count(paths.length));
    }

    function renameSelection() {
        const entry = mainWindow.firstSelected();
        if (!entry)
            return;
        activePane.beginRename(entry.path);
    }

    function pasteInto(dest, asLink) {
        ClipboardService.read(result => {
            if (result.paths.length === 0) {
                if (!asLink)
                    FileOpsService.pasteImage(dest, I18n.tr("Pasted image", "file name for an image pasted from the clipboard"), mainWindow.opScope);
                return;
            }
            if (asLink) {
                FileOpsService.link(result.paths, dest, mainWindow.opScope);
                return;
            }
            if (result.cut) {
                FileOpsService.move(result.paths, dest, mainWindow.opScope);
                ClipboardService.clear();
                return;
            }
            FileOpsService.copy(result.paths, dest, mainWindow.opScope);
        });
    }

    function showProperties() {
        const entries = mainWindow.selectedEntries();
        if (entries.length > 0) {
            mainWindow.dialogs.showProperties(entries);
            return;
        }
        mainWindow.dialogs.showPropertiesFor([currentPath]);
    }
}
