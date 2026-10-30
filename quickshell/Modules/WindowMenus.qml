pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Item {
    id: menus

    required property var mainWindow
    required property Item anchorItem

    property int activeModifiers: Qt.NoModifier
    readonly property bool shiftHeld: (activeModifiers & Qt.ShiftModifier) !== 0

    function selectionIsDirectory() {
        const entries = mainWindow.selectedEntries();
        return entries.length > 0 && entries.every(entry => entry.isDir);
    }

    function selectionIsExtractable() {
        const entries = mainWindow.selectedEntries();
        return entries.length > 0 && entries.every(entry => !entry.isDir && FilesFormat.extractable(entry.mime));
    }

    function showItemMenu(pointX, pointY, modifiers) {
        activeModifiers = modifiers ?? Qt.NoModifier;
        openWithMenu.close();
        backgroundMenu.close();
        const point = mainWindow.activePane.mapToItem(anchorItem, pointX, pointY);
        itemMenu.live = true;
        itemMenu.openAt(point.x, point.y);
    }

    function showColumnsMenu(pointX, pointY) {
        itemMenu.close();
        backgroundMenu.close();
        const point = mainWindow.activePane.mapToItem(anchorItem, pointX, pointY);
        columnsMenu.openAt(point.x, point.y);
    }

    function toggleColumn(id) {
        const shown = SessionData.listColumns;
        SessionData.listColumns = shown.includes(id) ? shown.filter(entry => entry !== id) : shown.concat([id]);
    }

    function showBackgroundMenu(pointX, pointY) {
        openWithMenu.close();
        itemMenu.close();
        const point = mainWindow.activePane.mapToItem(anchorItem, pointX, pointY);
        backgroundMenu.openAt(point.x, point.y);
    }

    function showOpenWithMenu(mimes, paths, source) {
        if (paths.length === 0)
            return;
        const anchor = source === "background" ? backgroundMenu : itemMenu;
        openWithMenu.mimes = mimes;
        openWithMenu.paths = paths;
        openWithMenu.apps = [];
        itemMenu.close();
        backgroundMenu.close();
        openWithMenu.openAt(anchor.x, anchor.y);
        const token = ++openWithMenu.requestSerial;
        AppsService.sections(mimes, paths, response => {
            if (response.error || token !== openWithMenu.requestSerial || !openWithMenu.visible)
                return;
            openWithMenu.mimes = response.mimeTypes || [];
            const recommended = response.recommended || [];
            const preferred = recommended.find(app => app.isDefault === true);
            const rest = recommended.filter(app => app !== preferred);
            openWithMenu.apps = (preferred ? [preferred] : []).concat(response.recent || [], rest).slice(0, 5);
        });
    }

    function showNewDocumentMenu(pointX, pointY) {
        openWithMenu.close();
        itemMenu.close();
        newDocumentMenu.present(pointX ?? backgroundMenu.x, pointY ?? backgroundMenu.y);
    }

    function showContextMenuAtCursor() {
        const pane = mainWindow.activePane;
        if (mainWindow.selectedPaths().length === 0) {
            showBackgroundMenu(pane.width / 2, pane.height / 2);
            return;
        }
        const rect = pane.cursorRect();
        if (rect.width === 0) {
            showItemMenu(pane.width / 2, pane.height / 2, Qt.NoModifier);
            return;
        }
        showItemMenu(rect.x + Math.min(rect.width, rect.height), rect.y + rect.height, Qt.NoModifier);
    }

    function presentPlaceMenu(kind, path, source, pointX, pointY) {
        placeMenu.present(kind, path, source, pointX, pointY);
    }

    function presentTabMenu(index, source, pointX, pointY) {
        tabMenu.present(index, source, pointX, pointY);
    }

    FileContextMenu {
        id: itemMenu

        // The rows rebuild on every input change, so the inputs only follow the selection while the menu is up.
        property bool live: false

        anchorItem: menus.anchorItem
        selectionCount: live ? menus.mainWindow.selectedPaths().length : 0
        hasDirectory: live && menus.selectionIsDirectory()
        hasClipboard: live && ClipboardService.hasFiles
        shiftHeld: menus.shiftHeld
        extractable: live && menus.selectionIsExtractable()
        inResults: live && menus.mainWindow.activePane.resultsActive
        inTrash: live && menus.mainWindow.activePane.trashActive
        inRecent: live && menus.mainWindow.activePane.recentActive
        selectedBookmarked: live && menus.selectionIsDirectory() && BookmarksService.has(menus.mainWindow.selectedPaths()[0] ?? "")
        onActionTriggered: id => menus.mainWindow.actions.runAction(id)
        onClosed: live = false
    }

    NewDocumentMenu {
        id: newDocumentMenu

        anchorItem: menus.anchorItem
        folder: menus.mainWindow.currentPath
        opScope: menus.mainWindow.opScope
    }

    PlaceMenu {
        id: placeMenu

        anchorItem: menus.anchorItem
        onTabRequested: path => menus.mainWindow.tabManager.newTab(path, false)
        onWindowRequested: path => menus.mainWindow.actions.newWindowAt(path)
        onRenameRequested: path => menus.mainWindow.dialogs.renameBookmark(path)
        onRemoveRequested: path => BookmarksService.remove(path)
        onCopyPathRequested: path => menus.mainWindow.actions.copyText(path)
        onPropertiesRequested: path => menus.mainWindow.dialogs.showPropertiesFor([path])
        onEmptyTrashRequested: menus.mainWindow.dialogs.confirmEmptyTrash()
        onClearRecentRequested: menus.mainWindow.dialogs.confirmClearRecent()
    }

    FileMenu {
        id: tabMenu

        property int index: -1

        anchorItem: menus.anchorItem

        function present(target, source, pointX, pointY) {
            index = target;
            const point = source.mapToItem(menus.anchorItem, pointX, pointY);
            openAt(point.x, point.y);
        }

        FileMenuItem {
            text: I18n.tr("Duplicate tab", "tab context menu entry opening a copy of this tab beside it")
            iconName: "tab_duplicate"
            onClicked: {
                tabMenu.close();
                menus.mainWindow.tabManager.duplicateTab(tabMenu.index);
            }
        }

        FileMenuItem {
            text: I18n.tr("Reopen the closed tab", "keyboard shortcut")
            iconName: "history"
            enabled: menus.mainWindow.tabManager.closedTabs.length > 0
            onClicked: {
                tabMenu.close();
                menus.mainWindow.tabManager.reopenTab();
            }
        }

        FileMenuDivider {}

        FileMenuItem {
            text: I18n.tr("Close tab", "tooltip on the button closing a tab")
            iconName: "close"
            enabled: menus.mainWindow.tabManager.tabs.length > 1
            onClicked: {
                tabMenu.close();
                menus.mainWindow.tabManager.closeTab(tabMenu.index);
            }
        }

        FileMenuItem {
            text: I18n.tr("Close other tabs", "tab context menu entry closing every tab but this one")
            iconName: "tab_close"
            enabled: menus.mainWindow.tabManager.tabs.length > 1
            onClicked: {
                tabMenu.close();
                menus.mainWindow.tabManager.closeOtherTabs(tabMenu.index);
            }
        }

        FileMenuItem {
            text: I18n.tr("Close tabs to the right", "tab context menu entry closing the tabs after this one")
            iconName: "tab_close_right"
            enabled: tabMenu.index < menus.mainWindow.tabManager.tabs.length - 1
            onClicked: {
                tabMenu.close();
                menus.mainWindow.tabManager.closeTabsToRight(tabMenu.index);
            }
        }
    }

    FileMenu {
        id: columnsMenu

        anchorItem: menus.anchorItem

        Repeater {
            model: FileColumns.optional

            FileMenuItem {
                required property var modelData

                text: FileColumns.label(modelData.id)
                checked: SessionData.listColumns.includes(modelData.id)
                onClicked: {
                    columnsMenu.close();
                    menus.toggleColumn(modelData.id);
                }
            }
        }
    }

    FileContextMenu {
        id: backgroundMenu

        anchorItem: menus.anchorItem
        forFolder: true
        hasClipboard: ClipboardService.hasFiles
        inTrash: menus.mainWindow.activePane.trashActive
        inRecent: menus.mainWindow.activePane.recentActive
        trashCount: TrashService.count
        recentCount: menus.mainWindow.activePane.recentView?.model.total ?? 0
        onActionTriggered: id => menus.mainWindow.actions.runAction(id)
    }

    FileMenu {
        id: openWithMenu

        property var apps: []
        property var mimes: []
        property var paths: []
        property int requestSerial: 0

        anchorItem: menus.anchorItem
        onClosed: apps = []

        Repeater {
            model: openWithMenu.apps

            FileMenuItem {
                required property var modelData

                text: modelData.name
                checked: modelData.isDefault === true
                onClicked: {
                    openWithMenu.close();
                    AppsService.openWith(openWithMenu.paths, modelData.id, response => menus.mainWindow.openFlow.reportError(response));
                }
            }
        }

        FileMenuDivider {
            visible: openWithMenu.apps.length > 0
        }

        FileMenuItem {
            text: I18n.tr("Other application…", "open with menu entry opening the full application picker")
            iconName: "more_horiz"
            onClicked: {
                openWithMenu.close();
                menus.mainWindow.dialogs.askOpenWith(openWithMenu.mimes, openWithMenu.paths);
            }
        }
    }
}
