pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FloatingWindow {
    id: window

    property var request: null

    readonly property string requestId: request?.id ?? ""
    readonly property string method: request?.method ?? "open"
    readonly property bool saving: method === "save"
    readonly property bool savingFiles: method === "saveFiles"
    readonly property bool folderMode: savingFiles || (method === "open" && request?.directory === true)
    readonly property bool multiple: method === "open" && request?.multiple === true
    readonly property var filters: request?.filters ?? []
    readonly property var choices: request?.choices ?? []
    readonly property var incomingFiles: request?.files ?? []
    readonly property var filterGlobs: filterIndex >= 0 && filterIndex < filters.length ? (filters[filterIndex].globs ?? []) : []
    readonly property string opScope: "chooser" + requestId
    readonly property string typedTarget: {
        const typed = nameField.text.trim();
        if (typed.startsWith("/") || typed.startsWith("~"))
            return FilePaths.normalize(typed);
        return typed === "" || pane.path === "" ? "" : FilePaths.join(pane.path, typed);
    }
    readonly property bool canAccept: {
        if (pane.virtualActive)
            return false;
        switch (method) {
        case "save":
            return typedTarget !== "" && FileFormat.validName(FilePaths.baseName(typedTarget));
        case "saveFiles":
            return pane.path !== "";
        default:
            {
                if (folderMode)
                    return pane.path !== "";
                const files = pane.selectedEntries.filter(entry => !entry.isDir).length;
                return multiple ? files > 0 : files === 1;
            }
        }
    }
    readonly property string acceptText: {
        if ((request?.acceptLabel ?? "") !== "")
            return request.acceptLabel;
        switch (method) {
        case "save":
            return I18n.tr("Save", "file browser save button");
        case "saveFiles":
            return I18n.tr("Save here", "file chooser accept button when several files go into the chosen folder");
        default:
            return folderMode ? I18n.tr("Use this folder", "file browser folder selection confirm button") : I18n.tr("Open", "file browser open button");
        }
    }
    readonly property string defaultTitle: {
        switch (method) {
        case "save":
            return I18n.tr("Save file", "file chooser default title when saving one file");
        case "saveFiles":
            return I18n.tr("Save files", "file chooser default title when saving several files into a folder");
        default:
            if (folderMode)
                return I18n.tr("Select folder", "file chooser default title when picking a folder");
            return multiple ? I18n.tr("Open files", "file chooser default title when picking several files") : I18n.tr("Open file", "file chooser default title when picking one file");
        }
    }

    property int filterIndex: -1
    property var choiceValues: ({})
    property bool showHidden: SettingsData.showHidden
    property string sortKey: SettingsData.defaultSort
    property bool sortDescending: SettingsData.defaultSortDescending
    property string viewMode: SettingsData.defaultView
    property bool previewOpen: true
    property bool sidebarShown: true
    property string errorText: ""
    property string overwriteTarget: ""
    property var pendingTargets: []
    property bool settled: false
    property bool restoring: false

    readonly property bool compact: width < FilesMetrics.sidebarOverlayBreakpoint
    readonly property bool sidebarDocked: sidebarShown && !compact
    readonly property bool textEditing: surface.focusItem instanceof TextInput || surface.focusItem instanceof TextEdit

    title: (request?.title ?? "") !== "" ? request.title : defaultTitle
    implicitWidth: Math.min(FileBrowserMetrics.pickerWidth, (screen?.width ?? FileBrowserMetrics.pickerWidth) * FileBrowserMetrics.pickerScreenFraction)
    implicitHeight: Math.min(FileBrowserMetrics.pickerHeight, (screen?.height ?? FileBrowserMetrics.pickerHeight) * FileBrowserMetrics.pickerScreenFraction)
    minimumSize: Qt.size(FileBrowserMetrics.pickerMinWidth, FileBrowserMetrics.pickerMinHeight)
    color: Theme.surface
    visible: request !== null

    onClosed: cancel()
    Component.onCompleted: start()
    Component.onDestruction: FileOpsService.forget(opScope)

    function start() {
        if (!request)
            return;
        const saved = FileBrowserSettings.load("chooser");
        restoring = true;
        viewMode = saved.viewMode || SettingsData.defaultView;
        sortKey = saved.sortKey;
        sortDescending = saved.sortDesc;
        showHidden = saved.showHidden ?? SettingsData.showHidden;
        sidebarShown = saved.showSidebar;
        restoring = false;
        filterIndex = request.currentFilter >= 0 ? request.currentFilter : (filters.length > 0 ? 0 : -1);
        const values = ({});
        for (const choice of choices)
            values[choice.id] = choice.initial ?? "";
        choiceValues = values;
        nameField.text = request.currentName ?? "";
        openStart(saved.lastPath);
        focusStart();
    }

    function openStart(lastPath) {
        const fallback = lastPath || Paths.strip(Paths.home);
        const file = request.currentFile ?? "";
        const folder = request.currentFolder ?? "";
        if (file !== "") {
            DirectoryService.stat(file, result => {
                if (result.error || result.entry?.isDir) {
                    pane.navigate(folder || fallback);
                    return;
                }
                pane.reveal([file]);
            });
            return;
        }
        pane.navigate(folder || fallback);
    }

    function focusStart() {
        if (!saving) {
            pane.focusBody();
            return;
        }
        nameField.forceActiveFocus();
        selectStem();
    }

    function selectStem() {
        nameField.select(0, FileFormat.stemLength(nameField.text, false));
    }

    function persist(patch) {
        if (restoring)
            return;
        FileBrowserSettings.save("chooser", patch);
    }

    function setSort(key) {
        const descending = key === sortKey ? !sortDescending : false;
        sortKey = key;
        sortDescending = descending;
        persist({
            "sortKey": key,
            "sortDesc": descending
        });
    }

    function setDirection(descending) {
        sortDescending = descending;
        persist({
            "sortDesc": descending
        });
    }

    function setViewMode(mode) {
        viewMode = mode;
        persist({
            "viewMode": mode
        });
    }

    function setShowHidden(show) {
        showHidden = show;
        persist({
            "showHidden": show
        });
    }

    function setSidebarShown(show) {
        sidebarShown = show;
        persist({
            "showSidebar": show
        });
    }

    function respond(answer) {
        if (settled)
            return;
        settled = true;
        if (pane.path !== "")
            persist({
                "lastPath": pane.path
            });
        ChooserService.answer(requestId, answer);
    }

    function cancel() {
        respond({
            "response": 1
        });
    }

    function finish(paths) {
        respond({
            "response": 0,
            "paths": paths,
            "choices": choices.map(choice => ({
                        "id": choice.id,
                        "value": choiceValues[choice.id] ?? choice.initial ?? ""
                    })),
            "currentFilter": filterIndex
        });
    }

    function activate(entry) {
        errorText = "";
        if (entry.isDir) {
            pane.navigate(entry.path);
            return;
        }
        switch (method) {
        case "open":
            if (!folderMode)
                finish([entry.path]);
            return;
        case "save":
            nameField.text = entry.name;
            acceptCurrent();
            return;
        }
    }

    function openEntries(entries) {
        if (entries.length === 1 && entries[0].isDir) {
            pane.navigate(entries[0].path);
            return;
        }
        if (saving) {
            const file = entries.find(entry => !entry.isDir);
            if (file)
                nameField.text = file.name;
            acceptCurrent();
            return;
        }
        acceptCurrent();
    }

    function acceptCurrent() {
        if (!canAccept)
            return;
        errorText = "";
        const selected = pane.selectedEntries;
        switch (method) {
        case "save":
            checkSave(typedTarget);
            return;
        case "saveFiles":
            {
                const dirs = selected.filter(entry => entry.isDir);
                checkSaveFiles(dirs.length === 1 ? dirs[0].path : pane.path);
                return;
            }
        default:
            if (folderMode) {
                const dirs = selected.filter(entry => entry.isDir);
                finish([dirs.length === 1 ? dirs[0].path : pane.path]);
                return;
            }
            {
                const files = selected.filter(entry => !entry.isDir).map(entry => entry.path);
                if (files.length === 0)
                    return;
                finish(multiple ? files : [files[0]]);
            }
        }
    }

    function checkSave(target) {
        DirectoryService.stat(target, result => {
            if (result.error) {
                if (result.code === "ENOENT") {
                    window.finish([target]);
                    return;
                }
                window.errorText = FileFormat.operationError(result.code, result.error);
                return;
            }
            if (result.entry?.isDir) {
                nameField.text = "";
                pane.navigate(target);
                return;
            }
            window.overwriteTarget = target;
            overwriteDialog.open();
        });
    }

    function checkSaveFiles(folder) {
        const targets = incomingFiles.map(name => FilePaths.join(folder, name));
        if (targets.length === 0) {
            finish([]);
            return;
        }
        let remaining = targets.length;
        const existing = [];
        for (const target of targets) {
            DirectoryService.stat(target, result => {
                if (!result.error)
                    existing.push(target);
                remaining--;
                if (remaining > 0)
                    return;
                if (existing.length === 0) {
                    window.finish(targets);
                    return;
                }
                window.pendingTargets = targets;
                window.overwriteTarget = existing.map(path => FilePaths.baseName(path)).join(", ");
                overwriteDialog.open();
            });
        }
    }

    function confirmOverwrite() {
        if (savingFiles) {
            const targets = pendingTargets;
            pendingTargets = [];
            overwriteTarget = "";
            finish(targets);
            return;
        }
        const target = overwriteTarget;
        overwriteTarget = "";
        finish([target]);
    }

    function clearSearch() {
        search.text = "";
        pane.closeSearch();
        pane.focusBody();
    }

    function escapeBack() {
        if (!pane.resultsActive && pane.nameFilter === "")
            return false;
        clearSearch();
        return true;
    }

    function createFolder() {
        if (pane.path === "" || pane.virtualActive)
            return;
        FileOpsService.newFolder(pane.path, I18n.tr("Untitled folder", "default name for a new folder"), opScope, true);
    }

    function setChoice(id, value) {
        choiceValues = Object.assign({}, choiceValues, {
            [id]: value
        });
    }

    FocusScope {
        id: surface

        readonly property bool isFloatingWindowSurface: true
        readonly property Item focusItem: Window.activeFocusItem

        anchors.fill: parent
        focus: true
        LayoutMirroring.enabled: I18n.isRtl
        LayoutMirroring.childrenInherit: true

        FocusFallback {
            onTriggered: pane.focusBody()
        }

        Keys.onPressed: event => {
            const control = (event.modifiers & Qt.ControlModifier) !== 0;
            const alt = (event.modifiers & Qt.AltModifier) !== 0;
            switch (event.key) {
            case Qt.Key_Escape:
                if (!window.escapeBack())
                    window.cancel();
                break;
            case Qt.Key_L:
                if (!control)
                    return;
                pane.editPath();
                break;
            case Qt.Key_H:
                if (!control)
                    return;
                window.setShowHidden(!window.showHidden);
                break;
            case Qt.Key_F:
                if (!control)
                    return;
                search.forceActiveFocus();
                search.selectAll();
                break;
            case Qt.Key_Left:
                if (!alt)
                    return;
                pane.back();
                break;
            case Qt.Key_Right:
                if (!alt)
                    return;
                pane.forward();
                break;
            case Qt.Key_Return:
            case Qt.Key_Enter:
                if (window.textEditing || !window.folderMode)
                    return;
                window.acceptCurrent();
                break;
            default:
                if (window.textEditing || ItemTree.isInside(surface.focusItem, pane))
                    return;
                pane.handleStrayKey(event);
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
            onCloseRequested: window.cancel()
        }

        Item {
            id: sidebarSlot

            anchors.top: header.bottom
            anchors.bottom: footer.top
            anchors.left: parent.left
            width: window.sidebarDocked ? Math.max(FilesMetrics.sidebarMinWidth, Math.min(CacheData.sidebarWidth, surface.width - FilesMetrics.windowMinWidth)) : 0
            clip: true

            Behavior on width {
                enabled: Theme.animationsEnabled
                NumberAnimation {
                    duration: Theme.expressiveDurations.expressiveFastSpatial
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Theme.expressiveCurves.standard
                }
            }

            Sidebar {
                id: sidebar

                anchors.fill: parent
                anchors.margins: FilesMetrics.sidebarPadding
                visible: parent.width > 0
                currentPath: pane.path
                onNavigateRequested: path => pane.navigate(path)
                onTabRequested: path => pane.navigate(path)
                onUnlockRequested: device => unlockDialog.ask(device)
                onConnectRequested: connectDialog.ask()
                onLeaveRequested: pane.focusBody()
            }
        }

        Item {
            id: content

            anchors.top: header.bottom
            anchors.bottom: footer.top
            anchors.left: sidebarSlot.right
            anchors.right: parent.right
            anchors.topMargin: FilesMetrics.contentMarginV
            anchors.leftMargin: FilesMetrics.contentMarginH
            anchors.rightMargin: FilesMetrics.contentMarginH

            Item {
                id: topbar

                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                height: FilesMetrics.topbarControlSize

                Row {
                    id: leftActions

                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: FilesMetrics.topbarGroupSpacing

                    FlatButton {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !window.compact
                        iconName: "menu"
                        tooltipText: I18n.tr("Toggle sidebar", "tooltip on the button showing or hiding the sidebar")
                        onClicked: window.setSidebarShown(!window.sidebarShown)
                    }

                    NavButtonPair {
                        anchors.verticalCenter: parent.verticalCenter
                        backEnabled: pane.canGoBack
                        forwardEnabled: pane.canGoForward
                        onBackRequested: pane.back()
                        onForwardRequested: pane.forward()
                    }

                    FlatButton {
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: pane.path !== "" && !pane.virtualActive
                        iconName: "create_new_folder"
                        backgroundColor: Theme.chipSurface
                        tooltipText: I18n.tr("New folder", "file browser toolbar button creating a folder")
                        onClicked: window.createFolder()
                    }
                }

                Row {
                    id: rightActions

                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: FilesMetrics.topbarSpacing

                    FlatButton {
                        anchors.verticalCenter: parent.verticalCenter
                        iconName: window.showHidden ? "visibility" : "visibility_off"
                        iconColor: window.showHidden ? Theme.onPrimary : Theme.surfaceText
                        backgroundColor: window.showHidden ? Theme.primary : Theme.chipSurface
                        tooltipText: I18n.tr("Show hidden files", "file browser toolbar toggle")
                        onClicked: window.setShowHidden(!window.showHidden)
                    }

                    FlatButton {
                        anchors.verticalCenter: parent.verticalCenter
                        iconName: window.viewMode === "grid" ? "view_list" : "grid_view"
                        backgroundColor: Theme.chipSurface
                        tooltipText: window.viewMode === "grid" ? I18n.tr("List view", "file browser toolbar button switching to the list") : I18n.tr("Grid view", "file browser toolbar button switching to the grid")
                        onClicked: window.setViewMode(window.viewMode === "grid" ? "list" : "grid")
                    }

                    FlatButton {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !window.compact
                        iconName: "view_sidebar"
                        iconColor: window.previewOpen ? Theme.onPrimary : Theme.surfaceText
                        backgroundColor: window.previewOpen ? Theme.primary : Theme.chipSurface
                        tooltipText: I18n.tr("Preview", "view menu entry and shortcut showing or hiding the preview drawer")
                        onClicked: window.previewOpen = !window.previewOpen
                    }

                    FileSortButton {
                        anchors.verticalCenter: parent.verticalCenter
                        sortKey: window.sortKey
                        descending: window.sortDescending
                        showLabel: !window.compact
                        onSortKeySelected: key => window.setSort(key)
                        onDirectionToggled: window.setDirection(!window.sortDescending)
                    }
                }

                DSearchField {
                    id: search

                    anchors.left: leftActions.right
                    anchors.right: rightActions.left
                    anchors.leftMargin: FilesMetrics.topbarSpacing
                    anchors.rightMargin: FilesMetrics.topbarSpacing
                    anchors.verticalCenter: parent.verticalCenter
                    controlHeight: FilesMetrics.topbarControlSize
                    leftIconSize: Theme.iconSizeMedium
                    backgroundColor: Theme.surfaceContainerLowest
                    placeholderText: I18n.tr("Search in Files", "placeholder in the search field")
                    onTextChanged: {
                        if (text !== pane.nameFilter)
                            pane.nameFilter = text;
                    }
                    onAccepted: {
                        if (text.trim() !== "")
                            pane.startSearch(text.trim(), null);
                    }
                    Keys.onEscapePressed: event => {
                        event.accepted = true;
                        window.clearSearch();
                    }
                    Keys.onDownPressed: event => {
                        event.accepted = true;
                        pane.focusBody();
                    }
                }
            }

            FilesPane {
                id: pane

                anchors.top: topbar.bottom
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.topMargin: FilesMetrics.contentSpacing
                focus: !window.saving
                folderViews: false
                showHidden: window.showHidden
                sortKey: window.sortKey
                sortDescending: window.sortDescending
                viewMode: window.viewMode
                previewOpen: window.previewOpen && !window.compact
                filters: window.filterGlobs
                multiSelect: window.multiple
                escapeClearsSelection: false
                typeAhead: true
                dragEnabled: false
                dropEnabled: false
                menusEnabled: false
                onActivateRequested: entry => window.activate(entry)
                onOpenRequested: entries => window.openEntries(entries)
                onTabRequested: path => pane.navigate(path)
                onRenameRequested: (target, name) => FileOpsService.rename(target, name, window.opScope)
                onSortRequested: key => window.setSort(key)
                onPreviewToggleRequested: window.previewOpen = !window.previewOpen
                onServerDialogRequested: connectDialog.ask()
                onConnectFailed: (uri, result) => connectDialog.showFailure(uri, result)
                onNameFilterChanged: {
                    if (search.text !== nameFilter)
                        search.text = nameFilter;
                }
                onFilterRequested: text => {
                    search.text = pane.nameFilter + text;
                    search.forceActiveFocus();
                    search.cursorPosition = search.text.length;
                }
                onFilterFocusRequested: search.forceActiveFocus()
                onPathChanged: {
                    window.errorText = "";
                    if (path !== "")
                        FolderHistory.visit(path);
                }
            }
        }

        Column {
            id: footer

            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: FilesMetrics.contentMarginH
            anchors.rightMargin: FilesMetrics.contentMarginH
            anchors.bottomMargin: FilesMetrics.contentMarginV
            spacing: Theme.spacingS

            Flow {
                width: parent.width
                spacing: Theme.spacingM
                visible: window.filters.length > 0 || window.choices.length > 0

                DDropdown {
                    id: filterDropdown

                    visible: window.filters.length > 0
                    showLabel: false
                    options: window.filters.map(filter => filter.name)
                    currentValue: window.filterIndex >= 0 && window.filterIndex < window.filters.length ? window.filters[window.filterIndex].name : ""
                    onValueChanged: value => {
                        const index = window.filters.findIndex(filter => filter.name === value);
                        if (index >= 0)
                            window.filterIndex = index;
                    }
                }

                Repeater {
                    model: window.choices

                    delegate: Row {
                        id: choiceRow

                        required property var modelData

                        readonly property bool isToggle: (modelData.options ?? []).length === 0
                        readonly property string value: window.choiceValues[modelData.id] ?? ""

                        spacing: Theme.spacingS

                        StyledText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: choiceRow.modelData.label
                            color: Theme.surfaceText
                            font.pixelSize: Theme.fontSizeMedium
                        }

                        DToggle {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: choiceRow.isToggle
                            hideText: true
                            text: choiceRow.modelData.label
                            checked: choiceRow.value === "true"
                            onToggled: value => window.setChoice(choiceRow.modelData.id, value ? "true" : "false")
                        }

                        DDropdown {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: !choiceRow.isToggle
                            showLabel: false
                            options: (choiceRow.modelData.options ?? []).map(option => option.label)
                            currentValue: (choiceRow.modelData.options ?? []).find(option => option.id === choiceRow.value)?.label ?? ""
                            onValueChanged: value => {
                                const option = (choiceRow.modelData.options ?? []).find(option => option.label === value);
                                if (option)
                                    window.setChoice(choiceRow.modelData.id, option.id);
                            }
                        }
                    }
                }
            }

            StyledText {
                width: parent.width
                visible: window.savingFiles && window.incomingFiles.length > 0
                text: I18n.tr("Saving %1 into the chosen folder", "file chooser footer listing the names an app wants to save").arg(window.incomingFiles.join(", "))
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                elide: Text.ElideMiddle
            }

            StyledText {
                width: parent.width
                visible: window.errorText !== ""
                text: window.errorText
                color: Theme.error
                font.pixelSize: Theme.fontSizeSmall
                elide: Text.ElideRight
            }

            Item {
                width: parent.width
                height: Theme.buttonHeightS

                DTextField {
                    id: nameField

                    anchors.left: parent.left
                    anchors.right: buttons.left
                    anchors.rightMargin: Theme.spacingM
                    anchors.verticalCenter: parent.verticalCenter
                    visible: window.saving
                    controlHeight: Theme.buttonHeightS
                    placeholderText: I18n.tr("Enter filename...", "file browser save filename input placeholder")
                    isError: text.trim() !== "" && !window.canAccept
                    onAccepted: window.acceptCurrent()
                    Keys.onEscapePressed: event => {
                        event.accepted = true;
                        if (!window.escapeBack())
                            window.cancel();
                    }
                }

                Row {
                    id: buttons

                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Theme.spacingS
                    LayoutMirroring.enabled: I18n.isRtl

                    TextButton {
                        text: I18n.tr("Cancel", "dialog button to cancel")
                        onClicked: window.cancel()
                    }

                    DButton {
                        enabled: window.canAccept
                        text: window.acceptText
                        onClicked: window.acceptCurrent()
                    }
                }
            }
        }

        Connections {
            target: FileOpsService

            function onJobFinished(job) {
                if (job.scope !== window.opScope)
                    return;
                if (job.state === "failed") {
                    ToastService.info(FileFormat.operationError(job.code, job.error));
                    return;
                }
                const created = job.created?.[0]?.path ?? "";
                if (job.kind !== "newFolder" || created === "" || FilePaths.parentOf(created) !== pane.path)
                    return;
                pane.revealAndRename(created);
            }
        }

        Connections {
            target: pane.selection
            enabled: window.saving

            function onPathsChanged() {
                const selected = pane.selectedEntries;
                if (selected.length !== 1 || selected[0].isDir)
                    return;
                nameField.text = selected[0].name;
            }
        }

        ConfirmDialog {
            id: overwriteDialog

            title: I18n.tr("This item already exists", "file conflict dialog header")
            message: window.savingFiles ? I18n.tr("%1 already exist in this folder. Do you want to overwrite them?", "file chooser overwrite dialog message, %1 lists the file names").arg(window.overwriteTarget) : I18n.tr("%1 already exists in this folder.", "file conflict dialog explanation").arg(FilePaths.baseName(window.overwriteTarget))
            confirmText: I18n.tr("Replace", "file conflict dialog button to overwrite the existing item")
            onConfirmed: window.confirmOverwrite()
            onClosed: {
                if (!window.settled)
                    window.focusStart();
            }
        }

        ConnectServerDialog {
            id: connectDialog

            onConnected: path => pane.navigate(path)
        }

        UnlockDialog {
            id: unlockDialog

            onUnlocked: mountPoint => {
                if (mountPoint !== "")
                    pane.navigate(mountPoint);
            }
        }

        ConflictDialog {
            opScope: window.opScope
        }

        OperationErrorDialog {
            opScope: window.opScope
        }

        Toast {
            anchors.fill: parent
            shown: surface.Window.active
        }
    }

    FloatingWindowControls {
        id: windowControls

        targetWindow: window
    }
}
