pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Window
import Quickshell
import Quickshell.Wayland
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FloatingWindow {
    id: window

    signal closeRequested
    signal newWindowRequested(var locations)

    required property int windowId
    readonly property bool primary: WindowService.primaryId === windowId
    property var toplevel: null

    property int activePaneIndex: 0
    property bool splitView: false
    property real splitRatio: 0.5
    property bool showHidden: SessionData.lastUsed("showHidden", SettingsData.showHidden)
    property bool previewOpen: SessionData.lastUsed("previewOpen", true)

    readonly property string opScope: "window" + windowId
    readonly property var cutSet: {
        const set = ({});
        if (!ClipboardService.cut)
            return set;
        for (const path of ClipboardService.paths)
            set[path] = true;
        return set;
    }
    readonly property var secondPane: splitLoader.item
    readonly property var panes: [primaryPane, secondPane]
    readonly property TabManager tabManager: tabManager
    readonly property WindowDialogs dialogs: dialogs
    readonly property WindowMenus menus: menus
    readonly property DragController drag: drag
    readonly property FileOpenFlow openFlow: openFlow
    readonly property WindowActions actions: actions
    readonly property WindowSidebarHost sidebar: sidebarHost
    readonly property FilesTopbar topbar: topbar
    readonly property bool headerHostsJobs: !header.compositorDecorated
    readonly property PreviewOverlay preview: previewOverlay
    readonly property var activePane: splitView && activePaneIndex === 1 && secondPane ? secondPane : primaryPane
    readonly property string currentPath: activePane.path
    readonly property bool dialogOpen: dialogs.anyOpen
    readonly property bool textEditing: surface.focusItem instanceof TextInput || surface.focusItem instanceof TextEdit
    readonly property bool sidebarFocused: sidebarHost.focused
    readonly property bool tabRowFocused: ItemTree.isInside(surface.focusItem, tabRow)
    readonly property bool shortcutsEnabled: !dialogOpen && !textEditing && !(preview?.open ?? false)
    readonly property bool lastActive: WindowService.lastActiveId === windowId
    readonly property var previewTarget: {
        const entries = selectedEntries();
        return entries.length === 1 ? entries[0] : null;
    }
    readonly property bool previewExpandable: preview.supports(previewTarget)

    Component.onDestruction: FileOpsService.forget(opScope)

    title: tabManager.folderTitle(currentPath)
    implicitWidth: SessionData.windowWidth
    implicitHeight: SessionData.windowHeight
    minimumSize: Qt.size(FilesMetrics.windowMinWidth, FilesMetrics.windowMinHeight)
    color: Theme.surface
    visible: true

    onClosed: requestClose()
    onMaximizedChanged: SessionData.maximized = maximized

    onWidthChanged: {
        if (!maximized)
            SessionData.windowWidth = width;
    }

    onHeightChanged: {
        if (!maximized)
            SessionData.windowHeight = height;
    }

    Component.onCompleted: {
        keepOwnBrowsing();
        const initial = WindowService.takePendingLocations(windowId);
        if (primary) {
            splitView = SessionData.splitView;
            splitRatio = SessionData.splitRatio;
            tabManager.restoreSession();
        } else {
            splitRatio = SessionData.splitRatio;
            tabManager.startFresh();
        }
        maximized = SessionData.maximized;
        if (primary)
            offerGtkImport();
        primaryPane.focusBody();
        if (initial.length > 0)
            openLocations(initial, true);
    }

    function claimToplevel() {
        if (!surface.windowActive)
            return;
        const active = ToplevelManager.activeToplevel;
        if (active && active.appId === "dev.duxlabs.files")
            toplevel = active;
    }

    function offerGtkImport() {
        if (!BookmarksService.importOffer)
            return;
        BookmarksService.declineImport();
        ToastService.show(I18n.tr("Import your GTK bookmarks?", "toast offering to copy the gtk file chooser bookmarks once"), {
            "actionLabel": I18n.tr("Import", "toast action importing gtk bookmarks"),
            "duration": ToastService.offerDuration,
            "action": () => BookmarksService.importGtk(added => ToastService.info(I18n.tr("%1 bookmarks imported", "toast after importing gtk bookmarks").arg(added)))
        });
    }

    function requestClose() {
        closeRequested();
    }

    function setSplitRatio(value) {
        splitRatio = Math.max(FilesMetrics.minimumPaneFraction, Math.min(1 - FilesMetrics.minimumPaneFraction, value));
        if (primary)
            SessionData.splitRatio = splitRatio;
    }

    function resizeSplit(fraction) {
        setSplitRatio(splitRatio + fraction);
    }

    function openPath(path, inNewTab) {
        if (path === "")
            return;
        if (inNewTab) {
            tabManager.newTab(path, false);
            return;
        }
        activePane.navigate(path);
    }

    function openLocations(locations, fresh) {
        locations.forEach((location, index) => openLocation(location, index > 0 || (!fresh && SettingsData.openLocationIn === "tab")));
    }

    function openLocation(location, inNewTab) {
        switch (location.kind) {
        case "servers":
            dialogs.askServer();
            return;
        case "recent":
            openPath(RecentService.path, inNewTab);
            return;
        case "remote":
            dialogs.connectRemote(location.uri, location.password);
            return;
        case "archive":
            openArchive(location.path, inNewTab);
            return;
        }
        if ((location.select ?? []).length === 0) {
            openPath(location.path, inNewTab);
            return;
        }
        if (inNewTab)
            tabManager.newTab(location.path, false);
        activePane.reveal(location.select);
    }

    function openArchive(path, inNewTab) {
        openPath(FilePaths.parentOf(path), inNewTab);
        if (!SettingsData.extractOnOpen) {
            activePane.reveal([path]);
            return;
        }
        openFlow.extractBeside(path, "enter");
    }

    function toggleSplit() {
        tabManager.captureTabs();
        splitView = !splitView;
        if (primary)
            SessionData.splitView = splitView;
        if (splitView)
            return;
        activePaneIndex = 0;
        primaryPane.focusBody();
    }

    function keepOwnBrowsing() {
        showHidden = showHidden;
        previewOpen = previewOpen;
    }

    function resolvePanes(keepBrowsingView) {
        for (const pane of [primaryPane, secondPane]) {
            if (pane && pane.path !== "")
                pane.resolveFolder(pane.path, keepBrowsingView ? pane.browsingView : "");
        }
    }

    function useForAllFolders() {
        const values = {
            "sortKey": activePane.sortKey,
            "sortDescending": activePane.sortDescending,
            "viewMode": activePane.viewMode
        };
        SettingsData.defaultSort = values.sortKey;
        SettingsData.defaultSortDescending = values.sortDescending;
        SettingsData.defaultView = values.viewMode;
        SessionData.remember(values);
        FolderViews.dropMatching(values);
    }

    function setShowHidden(show) {
        showHidden = show;
        SessionData.remember({
            "showHidden": show
        });
    }

    function escapeBack() {
        if (sidebarHost.overlayOpen) {
            sidebarHost.leave();
            return true;
        }
        if (!activePane.resultsActive && activePane.nameFilter === "")
            return false;
        topbar.clearSearch();
        return true;
    }

    function selectedPaths() {
        return activePane.selection.paths;
    }

    function selectedEntries() {
        return activePane.selectedEntries;
    }

    function firstSelected() {
        return selectedEntries()[0] ?? null;
    }

    function dismissEditors(position) {
        for (const pane of [primaryPane, secondPane]) {
            if (!pane)
                continue;
            const local = pane.mapFromItem(surface, position.x, position.y);
            pane.dismissPathEditor(local.x, local.y);
        }
        if (!topbar.searchFocused)
            return;
        const local = topbar.mapFromItem(surface, position.x, position.y);
        if (!topbar.searchContains(local.x, local.y))
            activePane.focusBody();
    }

    Connections {
        target: SettingsData

        function onDefaultViewChanged() {
            SessionData.remember({
                "viewMode": SettingsData.defaultView
            });
            window.resolvePanes();
        }

        function onDefaultSortChanged() {
            SessionData.remember({
                "sortKey": SettingsData.defaultSort
            });
            window.resolvePanes(true);
        }

        function onDefaultSortDescendingChanged() {
            SessionData.remember({
                "sortDescending": SettingsData.defaultSortDescending
            });
            window.resolvePanes(true);
        }

        function onShowHiddenChanged() {
            window.setShowHidden(SettingsData.showHidden);
        }
    }

    Connections {
        target: DevicesService

        function onRequestFailed(message, code) {
            ToastService.info(FileFormat.operationError(code, message));
        }
    }

    Connections {
        target: NetworkService

        function onRequestFailed(message, code) {
            ToastService.info(FileFormat.operationError(code, message));
        }
    }

    Connections {
        target: BookmarksService

        function onRequestFailed(message) {
            ToastService.info(message);
        }

        function onImportOfferChanged() {
            window.offerGtkImport();
        }
    }

    Connections {
        target: FileOpsService

        function onRequestFailed(message) {
            ToastService.info(message);
        }
    }

    Connections {
        target: RecentService

        function onRequestFailed(message, code) {
            ToastService.info(FileFormat.operationError(code, message));
        }
    }

    TabManager {
        id: tabManager

        panes: window.panes
        activePane: window.activePane
        activePaneIndex: window.activePaneIndex
        primary: window.primary
        onActivePaneRequested: index => window.activePaneIndex = index
        onDropRequested: (paths, dest, action) => window.drag.applyDrop(paths, dest, action)
    }

    FloatingWindowControls {
        id: windowControls

        targetWindow: window
    }

    component WindowPane: FilesPane {
        id: pane

        property int paneIndex: 0

        active: window.activePaneIndex === paneIndex
        showHidden: window.showHidden
        previewOpen: window.previewOpen
        showActiveOutline: window.splitView
        cutSet: window.cutSet
        onActivateRequested: entry => window.openFlow.activate(entry)
        onOpenRequested: entries => window.openFlow.openEntries(entries)
        onTabRequested: path => window.tabManager.newTab(path, true)
        onPathMenuRequested: (path, pointX, pointY) => window.menus.presentPlaceMenu("folder", path, pane, pointX, pointY)
        onItemMenuRequested: (index, pointX, pointY, modifiers) => window.menus.showItemMenu(pointX, pointY, modifiers)
        onBackgroundMenuRequested: (pointX, pointY) => window.menus.showBackgroundMenu(pointX, pointY)
        onDropRequested: (paths, dest, action) => window.drag.applyDrop(paths, dest, action)
        onDragRequested: paths => window.drag.beginDrag(paths, window.firstSelected())
        onRenameRequested: (target, name) => FileOpsService.rename(target, name, window.opScope)
        onSortRequested: key => pane.toggleSort(key)
        onColumnsMenuRequested: (pointX, pointY) => window.menus.showColumnsMenu(pointX, pointY)
        onFocusRequested: {
            window.activePaneIndex = paneIndex;
            if (window.sidebarFocused)
                pane.focusBody();
        }
        onPreviewToggleRequested: window.actions.runAction("togglePreview")
        onServerDialogRequested: window.dialogs.askServer()
        onConnectFailed: (uri, result) => window.dialogs.showServerFailure(uri, result)
        onRestoreRequested: window.actions.runAction("restore")
        onEmptyTrashRequested: window.actions.runAction("emptyTrash")
        onRemoveFromRecentRequested: window.actions.runAction("removeFromRecent")
        onClearRecentRequested: window.actions.runAction("clearRecent")
        onFilterRequested: text => topbar.openFilter(text)
        onFilterFocusRequested: topbar.focusFilter()
        onPathChanged: {
            window.tabManager.captureTabs();
            if (!window.tabManager.applyingTab)
                FolderHistory.visit(path);
        }
    }

    Item {
        id: surface

        anchors.fill: parent
        readonly property bool isFloatingWindowSurface: true
        readonly property bool windowActive: Window.active
        readonly property Item focusItem: Window.activeFocusItem

        LayoutMirroring.enabled: I18n.isRtl
        LayoutMirroring.childrenInherit: true

        FocusFallback {
            onTriggered: window.activePane.focusBody()
        }

        WheelHandler {
            property var forwardedScroller: null

            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            acceptedModifiers: Qt.NoModifier
            enabled: !window.dialogOpen
            onWheel: event => {
                forwardedScroller = sidebarHost.dockedScrollerAt(surface, event.x, event.y) ?? window.activePane.scroller;
                if (!forwardedScroller?.forwardWheel) {
                    event.accepted = false;
                    return;
                }
                forwardedScroller.forwardWheel(event);
            }
            onActiveChanged: {
                if (active)
                    return;
                forwardedScroller?.forwardWheelEnd();
                forwardedScroller = null;
            }
        }

        onWindowActiveChanged: {
            if (!windowActive)
                return;
            WindowService.activate(window.windowId);
            window.claimToplevel();
            ClipboardService.refresh();
            TrashService.refresh();
            RecentService.refresh();
            primaryPane.refreshOnActivate();
            window.secondPane?.refreshOnActivate();
        }

        Keys.onPressed: event => {
            if (window.textEditing)
                return;
            if (!window.sidebarFocused && !ItemTree.isInside(surface.focusItem, window.activePane)) {
                window.activePane.handleStrayKey(event);
                if (event.accepted)
                    return;
            }
            switch (event.key) {
            case Qt.Key_Escape:
                if (!window.escapeBack())
                    return;
                break;
            default:
                if (window.sidebarFocused || !window.actions.handleVimKey(event))
                    return;
            }
            event.accepted = true;
        }

        DWindowHeader {
            id: header

            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            controls: windowControls
            title: window.title
            onCloseRequested: window.requestClose()

            JobsButton {
                anchors.verticalCenter: parent.verticalCenter
                hosted: window.headerHostsJobs
            }
        }

        Item {
            id: content

            anchors.top: header.bottom
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottomMargin: FilesMetrics.contentMarginV
            anchors.leftMargin: Math.max(FilesMetrics.contentMarginH, sidebarHost.dockedWidth)
            anchors.rightMargin: FilesMetrics.contentMarginH
            enabled: !window.preview.open

            TabRow {
                id: tabRow

                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                visible: window.tabManager.tabs.length > 1
                height: visible ? implicitHeight : 0
                tabs: window.tabManager.tabs.map(state => window.tabManager.tabLabel(state))
                currentIndex: window.tabManager.currentTab
                onTabSelected: index => window.tabManager.selectTab(index)
                onTabClosed: index => window.tabManager.closeTab(index)
                onTabAdded: window.tabManager.newTab("", false)
                onTabDropHovered: index => window.tabManager.selectTab(index)
                onTabDropped: (index, drop) => window.tabManager.dropOnTab(index, drop)
                onTabMoved: (from, to) => window.tabManager.moveTab(from, to)
                onTabMenuRequested: (index, pointX, pointY) => window.menus.presentTabMenu(index, tabRow, pointX, pointY)
            }

            FilesTopbar {
                id: topbar

                anchors.top: tabRow.bottom
                anchors.topMargin: tabRow.visible ? FilesMetrics.contentSpacing : 0
                anchors.left: parent.left
                anchors.right: parent.right
                splitView: window.splitView
                backEnabled: window.activePane.canGoBack
                forwardEnabled: window.activePane.canGoForward
                sortKey: window.activePane.effectiveSortKey
                sortDescending: window.activePane.effectiveSortDescending
                resultsActive: window.activePane.resultsActive
                compact: sidebarHost.compact
                showJobs: !window.headerHostsJobs
                viewMode: window.activePane.viewMode
                folderRemembered: window.activePane.folderRemembered
                folderHasSettings: window.activePane.hasFolderSettings
                previewOpen: window.previewOpen
                previewExpandable: window.previewExpandable
                trashActive: window.activePane.trashActive
                recentActive: window.activePane.recentActive
                filterText: window.activePane.nameFilter
                currentPath: window.currentPath
                onFilterChanged: text => window.activePane.nameFilter = text
                onSearchRequested: query => window.activePane.startSearch(query, null)
                onSearchCleared: {
                    window.activePane.closeSearch();
                    window.activePane.focusBody();
                }
                onLocationRequested: location => {
                    window.openLocation(location, false);
                    window.activePane.focusBody();
                }
                onResultsRequested: {
                    window.activePane.focusBody();
                    window.activePane.body.jumpTo(0, Qt.NoModifier);
                }
                onSettingsRequested: window.dialogs.openSettings()
                onShortcutsRequested: window.actions.runAction("shortcuts")
                onSidebarToggled: sidebarHost.toggle()
                onBackRequested: window.activePane.back()
                onForwardRequested: window.activePane.forward()
                onSplitToggled: window.toggleSplit()
                onSortKeySelected: key => window.activePane.sortBy(key)
                onSortDirectionToggled: window.activePane.flipSort()
                onCopyPathRequested: window.actions.copyText(window.currentPath)
                onViewModeSelected: mode => window.activePane.setViewMode(mode)
                onZoomRequested: step => window.activePane.body.zoomBy(step)
                onUseForAllFoldersRequested: window.useForAllFolders()
                onResetFolderRequested: window.activePane.resetFolder()
                onPreviewToggled: window.actions.runAction("togglePreview")
                onPreviewExpandRequested: window.actions.runAction("previewExpand")
                onReloadRequested: window.actions.runAction("reload")
                onTerminalRequested: window.actions.runAction("openTerminal")
                onPropertiesRequested: window.actions.runAction("properties")
                onNewWindowRequested: window.actions.runAction("newWindow")
            }

            Item {
                id: panesRow

                anchors.top: topbar.bottom
                anchors.topMargin: FilesMetrics.contentSpacing
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right

                readonly property real usableWidth: width - (window.splitView ? FilesMetrics.splitterWidth : 0)

                WindowPane {
                    id: primaryPane

                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.left: parent.left
                    width: window.splitView ? panesRow.usableWidth * window.splitRatio : panesRow.width
                }

                PaneSplitter {
                    id: splitter

                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.left: primaryPane.right
                    visible: window.splitView
                    onMoved: delta => window.resizeSplit(delta / Math.max(1, panesRow.usableWidth))
                    onReset: window.setSplitRatio(0.5)
                }

                Loader {
                    id: splitLoader

                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.left: splitter.right
                    anchors.right: parent.right
                    active: window.splitView
                    sourceComponent: WindowPane {
                        paneIndex: 1
                    }
                    onLoaded: {
                        if (window.tabManager.tabs.length === 0)
                            return;
                        const stored = window.tabManager.tabState(window.tabManager.currentTab)?.panes?.[1];
                        item.restoreState(stored ?? {
                            "path": primaryPane.path
                        });
                    }
                }
            }
        }

        WindowSidebarHost {
            id: sidebarHost

            contentMinWidth: Math.max(FilesMetrics.windowMinWidth, topbar.minimumWidth + FilesMetrics.contentMarginH)
            anchors.top: header.bottom
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            mainWindow: window
            focusItem: surface.focusItem
            enabled: !window.preview.open
        }

        DragController {
            id: drag

            anchors.fill: parent
            anchorItem: surface
            opScope: window.opScope
            currentPath: window.currentPath
            onTrashRequested: (paths, subject) => window.actions.trashPaths(paths, subject)
        }

        Item {
            anchors.fill: parent
            z: 1

            TapHandler {
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                enabled: primaryPane.editingPath || (window.secondPane?.editingPath ?? false) || topbar.searchFocused
                onPressedChanged: {
                    if (pressed)
                        window.dismissEditors(point.position);
                }
            }
        }

        PreviewOverlay {
            id: previewOverlay

            anchors.fill: parent
            onClosed: window.activePane.focusBody()
        }
    }

    Instantiator {
        model: Shortcuts.bound("window")

        Shortcut {
            required property var modelData

            enabled: window.shortcutsEnabled && (modelData.id !== "contextMenu" || !(window.sidebarFocused || window.tabRowFocused))
            sequences: modelData.keys
            onActivated: window.actions.runAction(modelData.id)
        }
    }

    Instantiator {
        model: Shortcuts.bound("selection")

        Shortcut {
            required property var modelData

            enabled: window.shortcutsEnabled && !window.sidebarFocused && window.selectedPaths().length > 0
            sequences: modelData.keys
            onActivated: window.actions.runAction(modelData.id)
        }
    }

    Instantiator {
        model: FilesMetrics.tabShortcutCount

        Shortcut {
            required property int index

            enabled: window.shortcutsEnabled
            sequence: "Alt+" + (index + 1)
            onActivated: window.tabManager.selectTab(index)
        }
    }

    Instantiator {
        model: Shortcuts.bound("path")

        Shortcut {
            required property var modelData

            enabled: !window.dialogOpen
            sequences: modelData.keys
            onActivated: window.actions.runAction(modelData.id)
        }
    }

    WindowMenus {
        id: menus

        anchors.fill: parent
        mainWindow: window
        anchorItem: surface
    }

    WindowDialogs {
        id: dialogs

        mainWindow: window
    }

    FileOpenFlow {
        id: openFlow

        mainWindow: window
    }

    WindowActions {
        id: actions

        mainWindow: window
    }

    Connections {
        target: ToplevelManager
        function onActiveToplevelChanged() {
            window.claimToplevel();
        }
    }

    Toast {
        anchors.fill: surface
        shown: window.lastActive
    }
}
