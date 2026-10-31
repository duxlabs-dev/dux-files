pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FocusScope {
    id: pane

    property bool active: true
    property bool showActiveOutline: false
    property bool showHidden: false
    property string sortKey: SessionData.lastUsed("sortKey", SettingsData.defaultSort)
    property bool sortDescending: SessionData.lastUsed("sortDescending", SettingsData.defaultSortDescending)
    property string viewMode: SessionData.lastUsed("viewMode", SettingsData.defaultView)
    property bool folderViews: true
    property bool hasFolderSettings: false
    property var restoringView: null
    property bool previewOpen: true
    property var cutSet: ({})
    property string nameFilter: ""
    property var filters: []
    property bool multiSelect: true
    property bool escapeClearsSelection: true
    property bool typeAhead: !SettingsData.vimKeys
    property bool dragEnabled: true
    property bool dropEnabled: true
    property bool menusEnabled: true
    property int connectRequest: 0

    readonly property alias path: browser.path
    readonly property alias canGoBack: browser.canGoBack
    readonly property alias canGoForward: browser.canGoForward
    readonly property alias editingPath: browser.editingPath
    readonly property alias searchModel: searchModel
    readonly property bool resultsActive: searchModel.active
    readonly property bool trashActive: !resultsActive && TrashService.filesPath !== "" && browser.path === TrashService.filesPath
    readonly property bool recentActive: !resultsActive && browser.path === RecentService.path
    readonly property bool virtualActive: trashActive || recentActive
    readonly property var trashView: trashActive ? trashLoader.item : null
    readonly property var recentView: recentActive ? recentLoader.item : null
    readonly property var snapshotView: trashView ?? recentView
    readonly property var overlay: resultsActive ? results : snapshotView
    readonly property string effectiveSortKey: resultsActive ? searchModel.sortKey : snapshotView?.model.sortKey ?? sortKey
    readonly property bool effectiveSortDescending: resultsActive ? searchModel.sortDescending : snapshotView?.model.sortDescending ?? sortDescending
    readonly property var scroller: body.view?.activeList ?? body.view
    readonly property var selection: overlay ? overlay.selection : browser.selection
    readonly property var body: overlay ? overlay.body : browser.body
    readonly property var currentEntry: overlay ? overlay.body.cursorEntry() : browser.currentEntry
    readonly property var selectedEntries: overlay ? overlay.body.selectedEntries() : browser.selectedEntries
    readonly property bool filtering: nameFilter !== "" && !resultsActive
    readonly property string browsingView: viewMode === "columns" ? viewMode : ""
    readonly property bool folderRemembered: SettingsData.rememberFolderViews && path !== "" && !virtualActive && !resultsActive
    readonly property bool singleClickActivates: SettingsData.clickToOpen === "single"
    readonly property real thumbnailSizeLimit: SettingsData.thumbnailSizeLimitMb > 0 ? SettingsData.thumbnailSizeLimitMb * 1000000 : -1

    signal activateRequested(var entry)
    signal openRequested(var entries)
    signal tabRequested(string path)
    signal pathMenuRequested(string path, real pointX, real pointY)
    signal itemMenuRequested(int index, real pointX, real pointY, int modifiers)
    signal backgroundMenuRequested(real pointX, real pointY)
    signal dropRequested(var paths, string dest, string action)
    signal dragRequested(var paths)
    signal renameRequested(string target, string name)
    signal sortRequested(string key)
    signal columnsMenuRequested(real pointX, real pointY)
    signal previewToggleRequested
    signal focusRequested
    signal serverDialogRequested
    signal connectFailed(string uri, var result)
    signal restoreRequested
    signal emptyTrashRequested
    signal removeFromRecentRequested
    signal clearRecentRequested
    signal filterRequested(string text)
    signal filterFocusRequested

    function navigate(target) {
        closeSearch();
        browser.navigate(target);
    }

    function resolveFolder(target, heldView) {
        if (!folderViews)
            return;
        const entry = SettingsData.rememberFolderViews ? FolderViews.lookup(target) : null;
        const values = FolderViews.resolve(entry, SessionData.browsing, {
            "sortKey": SettingsData.defaultSort,
            "sortDescending": SettingsData.defaultSortDescending,
            "viewMode": SettingsData.defaultView
        });
        hasFolderSettings = entry !== null;
        sortKey = values.sortKey;
        sortDescending = values.sortDescending;
        viewMode = heldView || values.viewMode;
        if (entry)
            FolderViews.touch(target);
    }

    function refreshFolder() {
        if (path === "" || resultsActive)
            return;
        resolveFolder(path, browsingView);
    }

    function toggleSort(key) {
        setSortOrder(key, sortKey === key ? !sortDescending : false);
    }

    function setSortOrder(key, descending) {
        sortKey = key;
        sortDescending = descending;
        const values = {
            "sortKey": key,
            "sortDescending": descending
        };
        if (folderRemembered) {
            FolderViews.set(path, values);
            return;
        }
        SessionData.remember(values);
    }

    function sortBy(key) {
        if (resultsActive) {
            if (key !== searchModel.sortKey)
                searchModel.setSort(key);
            return;
        }
        if (key === "")
            return;
        if (snapshotView) {
            snapshotView.model.setSort(key, snapshotView.model.sortDescending);
            return;
        }
        setSortOrder(key, sortDescending);
    }

    function flipSort() {
        if (resultsActive) {
            searchModel.sortDescending = !searchModel.sortDescending;
            return;
        }
        if (snapshotView) {
            snapshotView.model.setSort(snapshotView.model.sortKey, !snapshotView.model.sortDescending);
            return;
        }
        setSortOrder(sortKey, !sortDescending);
    }

    function setViewMode(mode) {
        viewMode = mode;
        if (folderRemembered && mode === "columns")
            return;
        if (folderRemembered) {
            FolderViews.set(path, {
                "viewMode": mode
            });
            return;
        }
        SessionData.remember({
            "viewMode": mode
        });
    }

    function resetFolder() {
        FolderViews.reset(path);
    }

    function reveal(targets) {
        closeSearch();
        browser.reveal(targets);
    }

    function revealAndRename(target) {
        closeSearch();
        browser.revealAndRename(target);
    }

    function back() {
        closeSearch();
        browser.back();
    }

    function forward() {
        closeSearch();
        browser.forward();
    }

    function up() {
        if (virtualActive)
            return;
        closeSearch();
        browser.up();
    }

    function refreshOnActivate() {
        if (recentActive)
            return;
        refresh();
    }

    function refresh() {
        if (resultsActive) {
            searchModel.refresh();
            return;
        }
        if (snapshotView) {
            snapshotView.refresh();
            return;
        }
        browser.refresh();
    }

    function editPath() {
        closeSearch();
        browser.editPath();
    }

    function startJump() {
        closeSearch();
        browser.startJump();
    }

    function settleLocation(target) {
        browser.settleLocation(target);
    }

    function dismissPathEditor(x, y) {
        browser.dismissPathEditor(x, y);
    }

    function focusBody() {
        if (resultsActive) {
            results.focusBody();
            return;
        }
        if (snapshotView && !editingPath) {
            snapshotView.focusBody();
            return;
        }
        browser.focusBody();
    }

    function beginRename(target) {
        if (virtualActive)
            return;
        if (resultsActive) {
            results.body.beginRename(target);
            return;
        }
        browser.beginRename(target);
    }

    function saveState() {
        return Object.assign(browser.saveState(), {
            "viewMode": viewMode
        });
    }

    function restoreState(state) {
        closeSearch();
        const folder = path => path === "computer:" ? "/" : path;
        restoringView = state?.viewMode === "columns" ? state.viewMode : "";
        browser.restoreState(Object.assign({}, state, {
            "path": folder(state?.path),
            "back": state?.back?.map(folder),
            "forward": state?.forward?.map(folder)
        }));
        restoringView = null;
    }

    function openSelection() {
        const entries = selectedEntries.length > 0 ? selectedEntries : [currentEntry].filter(entry => entry);
        if (entries.length === 0)
            return false;
        openRequested(entries);
        return true;
    }

    function cursorRect() {
        const rect = body.cursorRect();
        const point = body.mapToItem(pane, rect.x, rect.y);
        return Qt.rect(point.x, point.y, rect.width, rect.height);
    }

    function startSearch(query, filterState) {
        if (path === "" || virtualActive)
            return;
        const state = filterState ?? results.chips.filterState;
        searchModel.rootPath = state.scope === "folder" ? path : Paths.strip(Paths.home);
        searchModel.query = query;
        searchModel.options = SearchFilters.options(state);
        searchModel.start();
        results.focusBody();
    }

    function setSearchScope(scope) {
        results.chips.scope = scope;
    }

    function focusSearchChips() {
        if (resultsActive)
            results.chips.focusFirst();
    }

    function closeSearch() {
        if (resultsActive)
            searchModel.clear();
        results.chips.resetScope();
        nameFilter = "";
    }

    function openTypedLocation(location) {
        switch (location.kind) {
        case "recent":
            navigate(RecentService.path);
            return;
        case "remote":
            break;
        default:
            serverDialogRequested();
            return;
        }
        const request = ++connectRequest;
        NetworkService.connectTo(location.uri, result => {
            if (request !== connectRequest)
                return;
            const path = result.error ? "" : (result.location || result.mount?.path || "");
            settleLocation(path);
            if (path === "" && !result.error)
                connectFailed(location.uri, result);
        }, location.password);
    }

    function requestItemMenu(index, pointX, pointY, modifiers) {
        if (menusEnabled)
            itemMenuRequested(index, pointX, pointY, modifiers);
    }

    function requestBackgroundMenu(pointX, pointY) {
        if (menusEnabled)
            backgroundMenuRequested(pointX, pointY);
    }

    function dropOn(dest, drop) {
        const paths = DragService.pathsFrom(drop);
        if (paths.length === 0 || paths.includes(dest))
            return;
        dropRequested(paths, dest, DragService.overrideFor(drop.proposedAction));
    }

    function setZoom(mode, level) {
        if (mode === "grid") {
            SessionData.gridZoom = level;
            return;
        }
        if (mode === "columns") {
            SessionData.columnZoom = level;
            SessionData.columnPaneWidth = 0;
            return;
        }
        SessionData.listZoom = level;
    }

    function setPreviewWidth(width) {
        CacheData.previewWidth = Math.round(width);
    }

    function setColumnWidth(id, width) {
        const widths = Object.assign({}, SessionData.columnWidths);
        widths[id] = width;
        SessionData.columnWidths = widths;
    }

    function forwardWheel(event) {
        if (!scroller?.forwardWheel) {
            event.accepted = false;
            return;
        }
        scroller.forwardWheel(event);
    }

    function forwardWheelEnd() {
        scroller?.forwardWheelEnd();
    }

    WheelHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        acceptedModifiers: Qt.NoModifier
        onWheel: event => pane.forwardWheel(event)
        onActiveChanged: {
            if (!active)
                pane.forwardWheelEnd();
        }
    }

    Keys.onPressed: event => handleKey(event)

    onVirtualActiveChanged: refocus.restart()
    onEditingPathChanged: {
        if (!editingPath && virtualActive)
            refocus.restart();
    }

    function refocusIfOwned() {
        if (pane.active && (pane.activeFocus || !pane.Window.activeFocusItem))
            focusBody();
    }

    Timer {
        id: refocus

        interval: 0
        onTriggered: pane.refocusIfOwned()
    }

    function handleKey(event) {
        switch (event.key) {
        case Qt.Key_Return:
        case Qt.Key_Enter:
            if (event.modifiers !== Qt.NoModifier && event.modifiers !== Qt.KeypadModifier)
                return;
            if (!openSelection())
                return;
            break;
        case Qt.Key_Up:
            if ((event.modifiers & Qt.AltModifier) === 0)
                return;
            up();
            break;
        default:
            return;
        }
        event.accepted = true;
    }

    function handleStrayKey(event) {
        handleKey(event);
        if (!event.accepted)
            body.handleKey(event);
        if (event.accepted)
            focusBody();
    }

    Connections {
        target: FileOpsService

        function onJobFinished(job) {
            if (pane.resultsActive && job.state === "done")
                searchModel.refresh();
        }
    }

    Connections {
        target: FolderViews

        function onChanged(key) {
            if (key !== "" && pane.path !== "" && key !== FolderViews.key(pane.path))
                return;
            pane.refreshFolder();
        }
    }

    Connections {
        target: SettingsData

        function onRememberFolderViewsChanged() {
            pane.refreshFolder();
        }
    }

    Connections {
        target: DevicesService

        function onDevicesChanged() {
            pane.refreshFolder();
        }
    }

    SearchModel {
        id: searchModel

        backend: DirectoryService
        thumbnailsEnabled: SettingsData.thumbnails
        thumbnailSizeLimit: pane.thumbnailSizeLimit
    }

    FileBrowserView {
        id: browser

        anchors.fill: parent
        visible: !pane.resultsActive && (!pane.virtualActive || pane.editingPath)
        focus: !pane.resultsActive && (!pane.virtualActive || pane.editingPath)
        showSidebar: false
        showNavigation: false
        showHidden: pane.showHidden
        sortKey: pane.sortKey
        sortDescending: pane.sortDescending
        viewMode: pane.viewMode
        gridZoom: SessionData.gridZoom
        listZoom: SessionData.listZoom
        columnZoom: SessionData.columnZoom
        columnPaneWidth: SessionData.columnPaneWidth
        listColumns: SessionData.listColumns
        columnWidths: SessionData.columnWidths
        dragEnabled: pane.dragEnabled
        dropEnabled: pane.dropEnabled
        showSelectionFooter: true
        previewDrawer: true
        previewOpen: pane.previewOpen
        previewWidth: CacheData.previewWidth
        previewKeys: true
        cutSet: pane.cutSet
        nameFilter: pane.nameFilter
        filters: pane.filters
        multiSelect: pane.multiSelect
        escapeClearsSelection: pane.escapeClearsSelection
        singleClickActivates: pane.singleClickActivates
        typeAheadEnabled: pane.typeAhead
        typeAheadMode: SettingsData.listTyping
        countFolders: SettingsData.folderCounts
        dirsFirst: SettingsData.foldersFirst
        hiddenLast: SettingsData.hiddenLast
        thumbnailsEnabled: SettingsData.thumbnails
        thumbnailSizeLimit: pane.thumbnailSizeLimit
        networkThumbnails: SettingsData.networkThumbnails
        folderJump: FolderHistory
        unlistedPaths: [TrashService.filesPath, RecentService.path]
        header: pane.filtering ? filterHeader : null
        pathBarTrailing: paneStatus
        emptyText: {
            if (pane.virtualActive)
                return "";
            if (pane.filtering)
                return I18n.tr("No results", "search results header when nothing matched");
            return I18n.tr("This folder is empty", "empty directory placeholder");
        }
        paneColor: Theme.foregroundColor(Theme.cardSurface, true)
        paneRadius: FilesMetrics.paneRadius
        paneMargin: FilesMetrics.paneBodyMargin
        paneBorderWidth: pane.showActiveOutline && pane.active ? Theme.outlineWidthFocused : 0
        paneBorderColor: Theme.primary
        onNavigating: target => pane.resolveFolder(target, pane.restoringView ?? pane.browsingView)
        onActivateRequested: entry => pane.activateRequested(entry)
        onItemMenuRequested: (index, pointX, pointY, modifiers) => pane.requestItemMenu(index, pointX, pointY, modifiers)
        onBackgroundMenuRequested: (pointX, pointY) => pane.requestBackgroundMenu(pointX, pointY)
        onColumnsMenuRequested: (pointX, pointY) => {
            if (pane.menusEnabled)
                pane.columnsMenuRequested(pointX, pointY);
        }
        onItemsDropped: (dest, drop) => pane.dropOn(dest, drop)
        onDragRequested: paths => pane.dragRequested(paths)
        onRenameRequested: (target, name) => pane.renameRequested(target, name)
        onSortRequested: key => pane.sortRequested(key)
        onZoomRequested: (mode, level) => pane.setZoom(mode, level)
        onColumnResizeRequested: (id, width) => pane.setColumnWidth(id, width)
        onColumnPaneResizeRequested: width => SessionData.columnPaneWidth = width
        onPreviewToggleRequested: pane.previewToggleRequested()
        onPreviewResized: width => pane.setPreviewWidth(width)
        onFocusRequested: pane.focusRequested()
        onTabRequested: target => pane.tabRequested(target)
        onPathMenuRequested: (target, pointX, pointY) => {
            if (pane.menusEnabled)
                pane.pathMenuRequested(target, pointX, pointY);
        }
        onLocationRequested: location => pane.openTypedLocation(location)
        onLocationCancelled: pane.connectRequest++
        onFilterRequested: text => pane.filterRequested(text)
        onFilterFocusRequested: pane.filterFocusRequested()
    }

    SearchResults {
        id: results

        anchors.fill: parent
        visible: pane.resultsActive
        focus: pane.resultsActive
        model: searchModel
        viewMode: pane.viewMode
        gridZoom: SessionData.gridZoom
        listZoom: SessionData.listZoom
        columnWidths: SessionData.columnWidths
        previewOpen: pane.previewOpen
        previewWidth: CacheData.previewWidth
        cutSet: pane.cutSet
        dragEnabled: pane.dragEnabled
        singleClickActivates: pane.singleClickActivates
        typeAheadEnabled: pane.typeAhead
        countFolders: SettingsData.folderCounts
        border.width: pane.showActiveOutline && pane.active ? Theme.outlineWidthFocused : 0
        border.color: Theme.primary
        onActivateRequested: entry => pane.activateRequested(entry)
        onItemMenuRequested: (index, pointX, pointY, modifiers) => pane.requestItemMenu(index, pointX, pointY, modifiers)
        onDragRequested: paths => pane.dragRequested(paths)
        onRenameRequested: (target, name) => pane.renameRequested(target, name)
        onZoomRequested: (mode, level) => pane.setZoom(mode, level)
        onColumnResizeRequested: (id, width) => pane.setColumnWidth(id, width)
        onPreviewToggleRequested: pane.previewToggleRequested()
        onPreviewResized: width => pane.setPreviewWidth(width)
        onFocusRequested: pane.focusRequested()
        onTabRequested: target => pane.tabRequested(target)
        onCloseRequested: {
            pane.closeSearch();
            pane.focusBody();
        }
        onFiltersChanged: pane.startSearch(searchModel.query, results.chips.filterState)
    }

    Loader {
        id: trashLoader

        anchors.fill: parent
        active: pane.trashActive
        visible: !pane.editingPath
        sourceComponent: TrashView {
            viewMode: pane.viewMode
            gridZoom: SessionData.gridZoom
            listZoom: SessionData.listZoom
            columnWidths: SessionData.columnWidths
            previewOpen: pane.previewOpen
            previewWidth: CacheData.previewWidth
            nameFilter: pane.nameFilter
            singleClickActivates: pane.singleClickActivates
            typeAheadEnabled: pane.typeAhead
            typeAheadMode: SettingsData.listTyping
            countFolders: SettingsData.folderCounts
            thumbnailsEnabled: SettingsData.thumbnails
            thumbnailSizeLimit: pane.thumbnailSizeLimit
            outlined: pane.showActiveOutline && pane.active
            onFilterRequested: text => pane.filterRequested(text)
            onFilterFocusRequested: pane.filterFocusRequested()
            onActivateRequested: entry => pane.activateRequested(entry)
            onItemMenuRequested: (index, pointX, pointY, modifiers) => pane.requestItemMenu(index, pointX, pointY, modifiers)
            onBackgroundMenuRequested: (pointX, pointY) => pane.requestBackgroundMenu(pointX, pointY)
            onRestoreRequested: pane.restoreRequested()
            onEmptyRequested: pane.emptyTrashRequested()
            onItemsDropped: drop => pane.dropOn(TrashService.filesPath, drop)
            onZoomRequested: (mode, level) => pane.setZoom(mode, level)
            onColumnResizeRequested: (id, width) => pane.setColumnWidth(id, width)
            onPreviewToggleRequested: pane.previewToggleRequested()
            onPreviewResized: width => pane.setPreviewWidth(width)
            onFocusRequested: pane.focusRequested()
            onBackRequested: pane.back()
            onForwardRequested: pane.forward()
        }
    }

    Loader {
        id: recentLoader

        anchors.fill: parent
        active: pane.recentActive
        visible: !pane.editingPath
        sourceComponent: RecentView {
            viewMode: pane.viewMode
            gridZoom: SessionData.gridZoom
            listZoom: SessionData.listZoom
            columnWidths: SessionData.columnWidths
            previewOpen: pane.previewOpen
            previewWidth: CacheData.previewWidth
            cutSet: pane.cutSet
            nameFilter: pane.nameFilter
            singleClickActivates: pane.singleClickActivates
            typeAheadEnabled: pane.typeAhead
            typeAheadMode: SettingsData.listTyping
            countFolders: SettingsData.folderCounts
            thumbnailsEnabled: SettingsData.thumbnails
            thumbnailSizeLimit: pane.thumbnailSizeLimit
            dragEnabled: pane.dragEnabled
            outlined: pane.showActiveOutline && pane.active
            onFilterRequested: text => pane.filterRequested(text)
            onFilterFocusRequested: pane.filterFocusRequested()
            onActivateRequested: entry => pane.activateRequested(entry)
            onItemMenuRequested: (index, pointX, pointY, modifiers) => pane.requestItemMenu(index, pointX, pointY, modifiers)
            onBackgroundMenuRequested: (pointX, pointY) => pane.requestBackgroundMenu(pointX, pointY)
            onRemoveRequested: pane.removeFromRecentRequested()
            onClearRequested: pane.clearRecentRequested()
            onDragRequested: paths => pane.dragRequested(paths)
            onTabRequested: target => pane.tabRequested(target)
            onZoomRequested: (mode, level) => pane.setZoom(mode, level)
            onColumnResizeRequested: (id, width) => pane.setColumnWidth(id, width)
            onPreviewToggleRequested: pane.previewToggleRequested()
            onPreviewResized: width => pane.setPreviewWidth(width)
            onFocusRequested: pane.focusRequested()
            onBackRequested: pane.back()
            onForwardRequested: pane.forward()
        }
    }

    Component {
        id: paneStatus

        PaneStatus {
            total: browser.directory.total
            hidden: pane.showHidden ? 0 : browser.directory.hiddenCount
            compact: pane.width < Theme.smallBreakpoint
            showClipboard: pane.active
        }
    }

    Component {
        id: filterHeader

        Item {
            width: browser.body.width
            implicitHeight: filterRow.implicitHeight + Theme.spacingS * 2

            Row {
                id: filterRow

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Theme.spacingM

                StyledText {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - subfolderChip.width - parent.spacing
                    text: I18n.tr("%1 matching “%2”", "filter bar above a folder listing, count then the typed text").arg(FileFormat.count(browser.directory.count)).arg(pane.nameFilter)
                    color: Theme.surfaceVariantText
                    font.pixelSize: Theme.fontSizeSmall
                    elide: Text.ElideMiddle
                }

                DFilterChips {
                    id: subfolderChip

                    anchors.verticalCenter: parent.verticalCenter
                    width: implicitWidth
                    model: [
                        {
                            "value": "subfolders",
                            "label": I18n.tr("Search subfolders", "chip that turns the folder filter into a recursive search")
                        }
                    ]
                    multiSelect: true
                    selectedValues: []
                    showCounts: false
                    onSelectionToggled: pane.startSearch(pane.nameFilter, null)
                }
            }
        }
    }
}
