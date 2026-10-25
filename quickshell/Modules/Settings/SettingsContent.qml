import QtQuick
import qs.Common
import qs.Services
import qs.Modules
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser
import qs.DCommon.Common as DC

Item {
    id: root

    property int currentIndex: 0
    property var hostWindow: null
    property string searchQuery: ""
    property var results: []
    property var matchCounts: []

    readonly property bool searching: searchQuery.trim() !== ""
    readonly property var pageComponents: [generalPage, appearancePage, behaviourPage, devicesPage, aboutPage]
    readonly property var pageLabels: [I18n.tr("General", "settings sidebar tab label"), I18n.tr("Appearance", "settings sidebar tab label"), I18n.tr("Behaviour", "settings sidebar tab label"), I18n.tr("Devices", "sidebar section listing drives and volumes"), I18n.tr("About", "settings sidebar tab label")]

    signal resultPicked(int page, var row)
    property var terminals: []
    property var fileManagerStatus: ({})
    property var portalStatus: ({})
    property bool portalBusy: false

    readonly property var colorSourceOptions: [
        {
            label: I18n.tr("Auto", "color source button group option following DMS"),
            value: "auto"
        },
        {
            label: I18n.tr("Preset", "color source button group option for bundled palettes"),
            value: "preset"
        },
        {
            label: I18n.tr("Custom", "color source button group option for a custom theme file"),
            value: "custom"
        }
    ]

    readonly property var fontSourceOptions: [
        {
            label: I18n.tr("Auto", "font source button group option following DMS"),
            value: "auto"
        },
        {
            label: I18n.tr("System", "font source button group option for the Qt system font"),
            value: "system"
        },
        {
            label: I18n.tr("Custom", "font source button group option for app-chosen fonts"),
            value: "custom"
        }
    ]

    readonly property var densityOptions: [
        {
            label: I18n.tr("Spacious", "interface density option with the most room between items"),
            value: "spacious"
        },
        {
            label: I18n.tr("Comfortable", "interface density option with the default spacing"),
            value: "comfortable"
        },
        {
            label: I18n.tr("Compact", "interface density option fitting the most items on screen"),
            value: "compact"
        }
    ]

    readonly property var viewOptions: [
        {
            label: I18n.tr("List view", "view mode showing one row per item"),
            value: "list"
        },
        {
            label: I18n.tr("Grid view", "view mode showing items as tiles"),
            value: "grid"
        },
        {
            label: I18n.tr("Column view", "view mode showing folders side by side"),
            value: "columns"
        }
    ]

    readonly property var gridSizeOptions: FileBrowserMetrics.gridIconSizes.map((size, index) => ({
                label: I18n.tr("%1 px", "icon size option").arg(size),
                value: index
            }))

    readonly property var listSizeOptions: FileBrowserMetrics.listIconSizes.map((size, index) => ({
                label: I18n.tr("%1 px", "icon size option").arg(size),
                value: index
            }))

    readonly property var sortOptions: [
        {
            label: I18n.tr("Name", "file browser sort criterion option"),
            value: "name"
        },
        {
            label: I18n.tr("Size", "file browser sort criterion option"),
            value: "size"
        },
        {
            label: I18n.tr("Modified", "file browser sort criterion option"),
            value: "mtime"
        },
        {
            label: I18n.tr("Type", "file browser sort criterion option"),
            value: "type"
        }
    ]

    readonly property var dragOptions: [
        {
            label: I18n.tr("Move, copy across drives", "drag default action option"),
            value: "auto"
        },
        {
            label: I18n.tr("Copy", "context menu entry copying the selection"),
            value: "copy"
        },
        {
            label: I18n.tr("Move", "drag label for moving files"),
            value: "move"
        },
        {
            label: I18n.tr("Ask every time", "drag default action option"),
            value: "ask"
        }
    ]

    readonly property var terminalOptions: [
        {
            label: I18n.tr("Automatic", "terminal dropdown option following $TERMINAL and the installed emulators"),
            value: ""
        }
    ].concat(terminals.map(name => ({
                label: name,
                value: name
            })))

    function localeDisplayName(code) {
        const name = I18n.presentLocales[code].nativeLanguageName;
        return name.charAt(0).toUpperCase() + name.slice(1);
    }

    readonly property var languageOptions: [
        {
            label: I18n.tr("System default", "language dropdown option that follows the system language"),
            value: ""
        }
    ].concat(Object.keys(I18n.presentLocales).sort().map(code => ({
                label: root.localeDisplayName(code),
                value: code
            })))

    function optionLabels(options) {
        return options.map(o => o.label);
    }

    function labelForValue(options, value) {
        for (let i = 0; i < options.length; i++) {
            if (options[i].value === value)
                return options[i].label;
        }
        return options[0].label;
    }

    function valueForLabel(options, label) {
        for (let i = 0; i < options.length; i++) {
            if (options[i].label === label)
                return options[i].value;
        }
        return options[0].value;
    }

    function indexForValue(options, value) {
        const index = options.findIndex(option => option.value === value);
        return index < 0 ? 0 : index;
    }

    function loadTerminals() {
        DuxFilesService.sendRequest("files.apps.terminals", null, response => {
            if (response.error)
                return;
            root.terminals = (response.result || {}).terminals || [];
        });
    }

    function loadPortalStatus() {
        DuxFilesService.sendRequest("system.portal.get", null, response => {
            if (response.error)
                return;
            root.portalStatus = response.result || {};
        });
    }

    function setPortal(enabled) {
        root.portalBusy = true;
        DuxFilesService.sendRequest(enabled ? "system.portal.install" : "system.portal.uninstall", null, response => {
            root.portalBusy = false;
            if (response.error) {
                ToastService.info(response.error);
                root.loadPortalStatus();
                return;
            }
            root.portalStatus = response.result || {};
            if (root.portalStatus.frontend?.restartNeeded === true)
                portalRestartDialog.open();
        });
    }

    function restartPortal() {
        root.portalBusy = true;
        DuxFilesService.sendRequest("system.portal.restart", null, response => {
            root.portalBusy = false;
            if (response.error)
                ToastService.info(response.error);
            root.loadPortalStatus();
        });
    }

    function portalDescription() {
        const status = portalStatus;
        if (status.available === false)
            return FilesFormat.featureUnavailable(status.reason ?? "");
        if (status.systemManaged === true)
            return I18n.tr("The portal list is managed by the system.", "portal chooser setting description when XDG_DESKTOP_PORTAL_DIR is set");
        return "";
    }

    function portalFileLine() {
        const file = portalStatus.portalFile ?? ({});
        if (file.found !== true)
            return I18n.tr("Portal file: not installed", "portal chooser status line");
        return I18n.tr("Portal file: %1", "portal chooser status line, %1 is a path").arg(file.path ?? "");
    }

    function portalPreferenceLine() {
        const config = portalStatus.config ?? ({});
        const effective = config.effective ?? "";
        if (effective === "dfiles")
            return I18n.tr("Preference: %1 names Dux Files", "portal chooser status line, %1 is a path").arg(config.path ?? "");
        if (effective !== "")
            return I18n.tr("Preference: %1 is used", "portal chooser status line, %1 is another backend name").arg(effective);
        return I18n.tr("Preference: no file chooser backend configured", "portal chooser status line");
    }

    function portalBackendLine() {
        const backend = portalStatus.backend ?? ({});
        const service = portalStatus.service ?? ({});
        if (backend.owned === true)
            return I18n.tr("Backend: served by this instance", "portal chooser status line");
        if (service.found === true)
            return I18n.tr("Backend: started on demand through %1", "portal chooser status line, %1 is a path").arg(service.path ?? "");
        return I18n.tr("Backend: not reachable, no activation file", "portal chooser status line");
    }

    function loadFileManagerStatus() {
        DuxFilesService.sendRequest("system.fileManager.get", null, response => {
            if (response.error)
                return;
            root.fileManagerStatus = response.result || {};
        });
    }

    function setFileManager(enabled) {
        SettingsData.registerFileManager = enabled;
        DuxFilesService.sendRequest("system.fileManager.set", {
            "enabled": enabled
        }, response => {
            if (response.error) {
                root.loadFileManagerStatus();
                return;
            }
            root.fileManagerStatus = response.result || {};
        });
    }

    function fileManagerDescription() {
        const status = fileManagerStatus;
        if (status.available === false)
            return I18n.tr("No session bus.", "file manager registration setting description when the daemon has no session bus connection");
        if (status.queued === true && (status.owner ?? "") !== "")
            return I18n.tr("%1 holds it; Files takes over when it quits.", "file manager registration setting description while another file manager holds the name").arg(status.owner);
        if (status.queued === true)
            return I18n.tr("Another file manager holds it.", "file manager registration setting description while an unknown file manager holds the name");
        if ((status.reason ?? "") !== "")
            return I18n.tr("The session bus refused it.", "file manager registration setting description when requesting the name failed");
        return "";
    }

    function collectRows(item, out, state) {
        for (const child of item.children) {
            if (!child.visible)
                continue;
            if (child.isSettingsCard === true)
                state.section = child.title;
            if (child.isSettingsRow === true) {
                if (child.title !== "")
                    out.push({
                        "row": child,
                        "section": state.section
                    });
                continue;
            }
            collectRows(child, out, state);
        }
        return out;
    }

    function rebuildResults() {
        if (!searching) {
            results = [];
            matchCounts = [];
            return;
        }
        const found = [];
        const counts = [];
        for (let page = 0; page < pages.count; page++) {
            counts.push(0);
            const item = pages.itemAt(page)?.item;
            if (!item)
                continue;
            for (const hit of collectRows(item, [], {
                "section": ""
            })) {
                const score = SettingsSearch.score(searchQuery, hit.row.title, hit.row.subtitle, hit.row.keywords);
                if (score <= 0)
                    continue;
                counts[page]++;
                found.push({
                    "page": page,
                    "order": found.length,
                    "score": score,
                    "title": hit.row.title,
                    "where": [pageLabels[page], hit.section].filter(part => part !== "").join(" · "),
                    "row": hit.row
                });
            }
        }
        found.sort((a, b) => b.score - a.score || a.order - b.order);
        results = found;
        matchCounts = counts;
    }

    function pickFirst() {
        if (results.length === 0)
            return;
        resultPicked(results[0].page, results[0].row);
    }

    function revealRow(row) {
        revealTimer.row = row;
        revealTimer.restart();
    }

    function scrollToRow(row) {
        if (!row)
            return;
        let page = row.parent;
        while (page && page.isSettingsPage !== true)
            page = page.parent;
        if (!page)
            return;
        page.revealRow(row);
        row.flash();
    }

    onSearchQueryChanged: rebuild.restart()
    onTerminalsChanged: rebuild.restart()
    onFileManagerStatusChanged: rebuild.restart()
    onPortalStatusChanged: rebuild.restart()

    Timer {
        id: rebuild

        interval: 0
        onTriggered: root.rebuildResults()
    }

    Timer {
        id: revealTimer

        property var row: null

        interval: 0
        onTriggered: {
            if (row)
                root.scrollToRow(row);
            row = null;
        }
    }

    Component.onCompleted: {
        loadTerminals();
        loadFileManagerStatus();
        loadPortalStatus();
    }

    Connections {
        target: root.hostWindow
        function onVisibleChanged() {
            if (!root.hostWindow.visible)
                return;
            root.loadFileManagerStatus();
            root.loadPortalStatus();
        }
    }

    ConfirmDialog {
        id: portalRestartDialog

        title: I18n.tr("Restart the portal service?", "dialog title before restarting xdg-desktop-portal")
        message: I18n.tr("The change takes effect after xdg-desktop-portal restarts. Open and save dialogs other apps have open will close.", "dialog message before restarting xdg-desktop-portal")
        confirmText: I18n.tr("Restart", "dialog button restarting xdg-desktop-portal")
        danger: false
        onConfirmed: root.restartPortal()
    }

    ConfirmDialog {
        id: forgetFoldersDialog

        title: I18n.tr("Forget all folder settings?", "dialog title before clearing every remembered folder sort and view")
        message: I18n.tr("Every folder goes back to the default sort and view.", "dialog message before clearing every remembered folder sort and view")
        confirmText: I18n.tr("Forget", "dialog button clearing the remembered folder settings")
        onConfirmed: FolderViews.clear()
    }

    Item {
        id: pageHeader

        width: Math.min(SettingsMetrics.contentMaxWidth, parent.width - Theme.spacingL * 2)
        anchors.horizontalCenter: parent.horizontalCenter
        height: Math.max(SettingsMetrics.pageHeaderHeight, pageHeading.implicitHeight + Theme.spacingM * 2)

        StyledText {
            id: pageHeading
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignLeft
            text: root.searching ? I18n.tr("Search results", "settings page header while a search query is typed") : root.pageLabels[root.currentIndex]
            font.pixelSize: Theme.fontSizeXXLarge
            color: Theme.surfaceText
        }
    }

    Item {
        id: pagesHost

        anchors.top: pageHeader.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        clip: true

        Repeater {
            id: pages

            model: root.pageComponents.length

            Loader {
                id: pageLoader

                required property int index
                readonly property bool current: root.currentIndex === index

                width: pagesHost.width
                height: pagesHost.height
                y: current ? 0 : pagesHost.height
                active: current || root.searching
                enabled: current && !root.searching
                sourceComponent: root.pageComponents[index]
                onLoaded: rebuild.restart()
            }
        }
    }

    Rectangle {
        anchors.fill: pagesHost
        visible: root.searching
        color: Theme.surface

        SettingsPage {
            StyledText {
                visible: root.results.length === 0
                width: parent.width
                text: I18n.tr("No results", "search results header when nothing matched")
                font.pixelSize: Theme.fontSizeMedium
                color: Theme.surfaceVariantText
                horizontalAlignment: Text.AlignLeft
            }

            SettingsGroup {
                Repeater {
                    model: root.results

                    SettingsRow {
                        required property var modelData

                        clickable: true
                        showChevron: true
                        title: modelData.title
                        subtitle: modelData.where
                        onClicked: root.resultPicked(modelData.page, modelData.row)
                    }
                }
            }
        }
    }

    component OptionDropdownRow: SettingsDropdownRow {
        id: optionRow

        property var optionList: []
        property var current: null

        signal picked(var value)

        dropdownWidth: Math.max(SettingsMetrics.dropdownWidth, widest.width + SettingsMetrics.dropdownChrome)
        options: root.optionLabels(optionList)

        TextMetrics {
            id: widest
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontSizeMedium
            font.weight: Theme.fontWeightMedium
            text: optionRow.options.reduce((longest, label) => label.length > longest.length ? label : longest, "")
        }
        currentValue: root.labelForValue(optionList, current)
        onValueChanged: label => picked(root.valueForLabel(optionList, label))
    }

    component FontFamilyRow: SettingsDropdownRow {
        property string family: ""
        property string defaultFamily: Theme.defaultFontFamily
        readonly property string defaultLabel: I18n.tr("Default", "font family dropdown option for the bundled font")

        signal familyPicked(string name)

        enableFuzzySearch: true
        options: [defaultLabel]
        currentValue: family || defaultLabel
        onValueChanged: value => familyPicked(value === defaultLabel ? "" : value)
        Component.onCompleted: Qt.callLater(() => options = [defaultLabel].concat(DC.Fonts.selectableFamilies(defaultFamily)))
    }

    component OptionGroupRow: SettingsButtonGroupRow {
        property var optionList: []
        property var current: null

        signal picked(var value)

        model: root.optionLabels(optionList)
        currentIndex: root.indexForValue(optionList, current)
        onSelectionChanged: (index, selected) => {
            if (selected)
                picked(optionList[index].value);
        }
    }

    Component {
        id: generalPage
        SettingsPage {
            SettingsCard {
                title: I18n.tr("Window", "general settings section label")

                SettingsButtonGroupRow {
                    text: I18n.tr("Close behavior", "close behavior setting label")
                    keywords: I18n.tr("quit exit hide daemon keep running", "settings search keywords, space separated, add synonyms in your language")
                    model: [I18n.tr("Keep running", "close behavior button group option keeping the daemon warm"), I18n.tr("Quit", "close behavior button group option to quit the app")]
                    currentIndex: SettingsData.closeBehavior === "quit" ? 1 : 0
                    onSelectionChanged: (index, selected) => {
                        if (!selected)
                            return;
                        SettingsData.closeBehavior = index === 1 ? "quit" : "hide";
                    }
                }

                SettingsToggleRow {
                    text: I18n.tr("Register as the session file manager", "setting label for owning org.freedesktop.FileManager1")
                    keywords: I18n.tr("default file manager dbus reveal", "settings search keywords, space separated, add synonyms in your language")
                    description: root.fileManagerDescription()
                    enabled: root.fileManagerStatus.available !== false
                    checked: SettingsData.registerFileManager
                    onToggled: checked => root.setFileManager(checked)
                }

                SettingsToggleRow {
                    text: I18n.tr("Open folders from other apps in a new tab", "setting label for external open requests")
                    keywords: I18n.tr("tabs external open location", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.openLocationIn === "tab"
                    onToggled: checked => SettingsData.openLocationIn = checked ? "tab" : "current"
                }

                SettingsToggleRow {
                    text: I18n.tr("Extract archives on open", "setting label for opening archives")
                    keywords: I18n.tr("zip tar unpack compressed", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Unpacks into a folder beside it.", "setting description for opening archives")
                    checked: SettingsData.extractOnOpen
                    onToggled: checked => SettingsData.extractOnOpen = checked
                }

                OptionDropdownRow {
                    text: I18n.tr("Language", "interface language setting label")
                    keywords: I18n.tr("locale translation interface", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.languageOptions
                    current: SettingsData.language
                    onPicked: value => SettingsData.language = value
                }
            }

            SettingsCard {
                title: I18n.tr("Open and save dialogs", "general settings section label for the portal file chooser")

                SettingsToggleRow {
                    text: I18n.tr("Use Dux Files for open and save dialogs", "setting label for serving the desktop portal file chooser")
                    keywords: I18n.tr("portal file chooser picker xdg", "settings search keywords, space separated, add synonyms in your language")
                    description: root.portalDescription()
                    enabled: root.portalStatus.available === true && root.portalStatus.systemManaged !== true
                    toggling: root.portalBusy
                    checked: root.portalStatus.active === true
                    onToggled: checked => root.setPortal(checked)
                }

                SettingsRow {
                    visible: root.portalStatus.available === true
                    iconName: root.portalStatus.portalFile?.found === true ? "check_circle" : "info"
                    iconColor: root.portalStatus.portalFile?.found === true ? Theme.success : Theme.surfaceVariantText
                    title: root.portalFileLine()
                }

                SettingsRow {
                    visible: root.portalStatus.available === true
                    iconName: root.portalStatus.config?.effective === "dfiles" ? "check_circle" : "info"
                    iconColor: root.portalStatus.config?.effective === "dfiles" ? Theme.success : Theme.surfaceVariantText
                    title: root.portalPreferenceLine()
                }

                SettingsRow {
                    visible: root.portalStatus.available === true
                    iconName: root.portalStatus.backend?.owned === true || root.portalStatus.service?.found === true ? "check_circle" : "info"
                    iconColor: root.portalStatus.backend?.owned === true || root.portalStatus.service?.found === true ? Theme.success : Theme.surfaceVariantText
                    title: root.portalBackendLine()
                }

                SettingsRow {
                    visible: root.portalStatus.frontend?.restartNeeded === true
                    iconName: "restart_alt"
                    iconColor: Theme.error
                    clickable: true
                    title: I18n.tr("Restart the portal service", "settings row restarting xdg-desktop-portal")
                    keywords: I18n.tr("xdg-desktop-portal", "settings search keywords, space separated, add synonyms in your language")
                    showChevron: true
                    onClicked: portalRestartDialog.open()
                }
            }

            SettingsCard {
                title: I18n.tr("View", "keyboard shortcut group")

                OptionGroupRow {
                    text: I18n.tr("Default view", "setting label for the view a new window starts in")
                    keywords: I18n.tr("list grid columns icons tiles", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.viewOptions
                    current: SettingsData.defaultView
                    onPicked: value => SettingsData.defaultView = value
                }

                OptionDropdownRow {
                    text: I18n.tr("Sort by", "setting label for the default sort key")
                    keywords: I18n.tr("order name size modified type date", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.sortOptions
                    current: SettingsData.defaultSort
                    onPicked: value => SettingsData.defaultSort = value
                }

                SettingsToggleRow {
                    text: I18n.tr("Sort descending", "sort direction option")
                    keywords: I18n.tr("order direction ascending reverse", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.defaultSortDescending
                    onToggled: checked => SettingsData.defaultSortDescending = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Show hidden files", "menu entry listing dot files")
                    keywords: I18n.tr("dotfiles invisible", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Ctrl+H toggles per window.", "show hidden files setting description")
                    checked: SettingsData.showHidden
                    onToggled: checked => SettingsData.showHidden = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Show file extensions", "toggle showing the file type suffix in names")
                    keywords: I18n.tr("suffix type filename", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.showExtensions
                    onToggled: checked => SettingsData.showExtensions = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Folders before files", "folders first toggle label")
                    keywords: I18n.tr("directories first sort", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.foldersFirst
                    onToggled: checked => SettingsData.foldersFirst = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Remember view and sort per folder", "setting label for per-folder sort and view memory")
                    keywords: I18n.tr("folder memory per-folder remember", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.rememberFolderViews
                    onToggled: checked => SettingsData.rememberFolderViews = checked
                }

                SettingsRow {
                    clickable: true
                    iconName: "delete_sweep"
                    iconColor: Theme.error
                    enabled: SettingsData.rememberFolderViews
                    title: I18n.tr("Forget all folder settings", "settings row clearing every remembered folder sort and view")
                    keywords: I18n.tr("reset clear per-folder memory", "settings search keywords, space separated, add synonyms in your language")
                    showChevron: true
                    onClicked: forgetFoldersDialog.open()
                }

                OptionDropdownRow {
                    text: I18n.tr("Grid icon size", "setting label for the grid view icon size")
                    keywords: I18n.tr("zoom thumbnails tiles", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Ctrl+Wheel resizes per window.", "icon size setting description")
                    optionList: root.gridSizeOptions
                    current: SessionData.gridZoom
                    onPicked: value => SessionData.gridZoom = value
                }

                OptionDropdownRow {
                    text: I18n.tr("List icon size", "setting label for the list view icon size")
                    keywords: I18n.tr("zoom rows", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.listSizeOptions
                    current: SessionData.listZoom
                    onPicked: value => SessionData.listZoom = value
                }

                SettingsToggleRow {
                    text: I18n.tr("Count items in folders", "folder counts toggle label")
                    keywords: I18n.tr("size column entries", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Shown in the size column.", "folder counts toggle description")
                    checked: SettingsData.folderCounts
                    onToggled: checked => SettingsData.folderCounts = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Files", "keyboard shortcut group")

                SettingsButtonGroupRow {
                    text: I18n.tr("Open items with", "setting label choosing single or double click")
                    keywords: I18n.tr("single double click activate", "settings search keywords, space separated, add synonyms in your language")
                    model: [I18n.tr("Double click", "click to open option"), I18n.tr("Single click", "click to open option")]
                    currentIndex: SettingsData.clickToOpen === "single" ? 1 : 0
                    onSelectionChanged: (index, selected) => {
                        if (!selected)
                            return;
                        SettingsData.clickToOpen = index === 1 ? "single" : "double";
                    }
                }

                SettingsToggleRow {
                    text: I18n.tr("Confirm before moving to the trash", "setting label")
                    keywords: I18n.tr("delete confirmation prompt", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.confirmTrash
                    onToggled: checked => SettingsData.confirmTrash = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Keep a list of recent files", "setting label for the recently used files list")
                    keywords: I18n.tr("history recently used opened sidebar", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Shared with other apps.", "setting description for the recently used files list")
                    checked: SettingsData.recentFiles
                    onToggled: checked => SettingsData.recentFiles = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Keyboard", "general settings section label")

                SettingsToggleRow {
                    text: I18n.tr("Vim keys", "toggle label for vim style navigation")
                    keywords: I18n.tr("hjkl keyboard navigation modal", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("h j k l move, / searches.", "vim keys toggle description")
                    checked: SettingsData.vimKeys
                    onToggled: checked => SettingsData.vimKeys = checked
                }

                SettingsButtonGroupRow {
                    text: I18n.tr("Typing in the file list", "setting label choosing what typed letters do in the file list")
                    keywords: I18n.tr("type-ahead jump filter search keyboard", "settings search keywords, space separated, add synonyms in your language")
                    model: [I18n.tr("Jump to name", "typing in the file list option"), I18n.tr("Filter", "typing in the file list option")]
                    currentIndex: SettingsData.listTyping === "filter" ? 1 : 0
                    enabled: !SettingsData.vimKeys
                    onSelectionChanged: (index, selected) => {
                        if (!selected)
                            return;
                        SettingsData.listTyping = index === 1 ? "filter" : "jump";
                    }
                }

                SettingsRow {
                    title: I18n.tr("Keyboard shortcuts", "overlay listing every keyboard shortcut")
                    keywords: I18n.tr("keys bindings hotkeys F1", "settings search keywords, space separated, add synonyms in your language")
                    subtitle: I18n.tr("? or F1 in the main window.", "keyboard shortcuts settings row description")
                    iconName: "keyboard"
                }
            }
        }
    }

    Component {
        id: appearancePage
        SettingsPage {
            id: appearanceFlickable

            SettingsCard {
                title: I18n.tr("Theme", "appearance settings section label")

                SettingsButtonGroupRow {
                    text: I18n.tr("Theme", "theme mode setting label")
                    keywords: I18n.tr("light dark mode system color scheme", "settings search keywords, space separated, add synonyms in your language")
                    description: {
                        switch (SettingsData.themeMode) {
                        case "light":
                            return I18n.tr("Always use the light theme.", "theme mode setting description");
                        case "dark":
                            return I18n.tr("Always use the dark theme.", "theme mode setting description");
                        default:
                            return PortalService.available ? I18n.tr("Following the system color scheme (currently %1).", "theme mode setting description").arg(Theme.isLightMode ? I18n.tr("light", "current scheme name in theme mode description") : I18n.tr("dark", "current scheme name in theme mode description")) : I18n.tr("System preference unavailable, using dark.", "theme mode setting description");
                        }
                    }
                    model: [I18n.tr("Auto", "theme mode button group option"), I18n.tr("Light", "theme mode button group option"), I18n.tr("Dark", "theme mode button group option")]
                    currentIndex: {
                        switch (SettingsData.themeMode) {
                        case "light":
                            return 1;
                        case "dark":
                            return 2;
                        default:
                            return 0;
                        }
                    }
                    onSelectionChanged: (index, selected) => {
                        if (!selected)
                            return;
                        switch (index) {
                        case 1:
                            SettingsData.themeMode = "light";
                            break;
                        case 2:
                            SettingsData.themeMode = "dark";
                            break;
                        default:
                            SettingsData.themeMode = "auto";
                            break;
                        }
                    }
                }

                OptionGroupRow {
                    text: I18n.tr("Color source", "color source setting label")
                    keywords: I18n.tr("palette dms preset custom accent", "settings search keywords, space separated, add synonyms in your language")
                    description: {
                        switch (SettingsData.colorSource) {
                        case "preset":
                            return I18n.tr("Use a bundled color palette.", "color source description for preset");
                        case "custom":
                            return I18n.tr("Load colors from your own theme file.", "color source description for custom");
                        default:
                            return I18n.tr("Follow Dux Shell colors, falling back to a preset.", "color source description for auto");
                        }
                    }
                    optionList: root.colorSourceOptions
                    current: SettingsData.colorSource
                    onPicked: value => SettingsData.colorSource = value
                }

                SettingsRow {
                    visible: SettingsData.colorSource === "auto"
                    iconName: Theme.dmsColorsAvailable ? "check_circle" : "info"
                    iconColor: Theme.dmsColorsAvailable ? Theme.success : Theme.surfaceVariantText
                    title: Theme.dmsColorsAvailable ? I18n.tr("Using Dux Shell dynamic colors.", "auto color source status when DMS is active") : I18n.tr("Dux Shell not detected, using the %1 preset.", "auto color source status when DMS is missing").arg(Theme.presetLabel(SettingsData.presetTheme))
                }

                SettingsRow {
                    visible: SettingsData.colorSource === "preset"
                    title: I18n.tr("Palette: %1", "selected preset palette name").arg(Theme.presetLabel(SettingsData.presetTheme))
                    keywords: I18n.tr("preset palette swatch colors", "settings search keywords, space separated, add synonyms in your language")

                    body: Flow {
                        width: parent.width
                        spacing: Theme.spacingS

                        Repeater {
                            model: Theme.presetNames()

                            Rectangle {
                                id: swatch
                                required property string modelData
                                readonly property bool active: SettingsData.presetTheme === modelData

                                width: Theme.avatarSize
                                height: Theme.avatarSize
                                radius: Theme.fullRadius(width, height)
                                color: Theme.presetColors(modelData).primary
                                border.color: active ? Theme.primary : Theme.outlineVariant
                                border.width: active ? Theme.outlineWidthFocused + Theme.outlineWidth : Theme.outlineWidth
                                Accessible.role: Accessible.RadioButton
                                Accessible.name: Theme.presetLabel(modelData)
                                Accessible.checked: active

                                StateLayer {
                                    stateColor: Theme.surfaceText
                                    cornerRadius: parent.radius
                                    onClicked: SettingsData.presetTheme = swatch.modelData
                                }
                            }
                        }
                    }
                }

                SettingsRow {
                    visible: SettingsData.colorSource === "custom"
                    clickable: true
                    iconName: "folder_open"
                    title: SettingsData.customThemeFile ? SettingsData.customThemeFile.split('/').pop() : I18n.tr("No theme file selected", "custom theme empty state title")
                    keywords: I18n.tr("custom theme json file", "settings search keywords, space separated, add synonyms in your language")
                    subtitle: {
                        if (!SettingsData.customThemeFile)
                            return I18n.tr("Choose a JSON color theme file.", "custom theme hint when none selected");
                        if (Theme.customThemeLoaded)
                            return SettingsData.customThemeFile;
                        return I18n.tr("Could not read that file, expected a JSON color theme.", "custom theme error when file is invalid");
                    }
                    subtitleColor: (SettingsData.customThemeFile && !Theme.customThemeLoaded) ? Theme.error : Theme.surfaceVariantText
                    showChevron: true
                    onClicked: {
                        themePickerLoader.active = true;
                        themePickerLoader.item.open();
                    }
                }
            }

            SettingsCard {
                title: I18n.tr("Shape", "appearance settings section label")

                SettingsSliderRow {
                    text: I18n.tr("Radius strength", "global component corner rounding")
                    keywords: I18n.tr("corners rounding shape square", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("50 is the Material default.", "radius strength slider description")
                    minimumLabel: I18n.tr("Square", "radius strength slider label at zero")
                    value: SettingsData.radiusStrength
                    minimum: 0
                    maximum: 100
                    unit: ""
                    onSliderValueChanged: newValue => SettingsData.radiusStrength = newValue
                }
            }

            SettingsCard {
                title: I18n.tr("Density", "appearance settings section label")

                OptionGroupRow {
                    text: I18n.tr("Interface density", "interface density setting label")
                    keywords: I18n.tr("density compact spacious comfortable spacing padding rows", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.densityOptions
                    current: SettingsData.density
                    onPicked: value => SettingsData.density = value
                }
            }

            SettingsCard {
                title: I18n.tr("Typography", "appearance settings section label")

                OptionGroupRow {
                    text: I18n.tr("Font source", "font source setting label")
                    keywords: I18n.tr("font family typeface dms system qt", "settings search keywords, space separated, add synonyms in your language")
                    optionList: root.fontSourceOptions
                    current: SettingsData.fontSource
                    onPicked: value => SettingsData.fontSource = value
                }

                SettingsRow {
                    visible: SettingsData.fontSource === "auto" && !Theme.typography.followingDms
                    iconName: "info"
                    iconColor: Theme.surfaceVariantText
                    title: I18n.tr("Dux Shell not detected, using %1.", "auto font source status when DMS is missing").arg(Theme.fontFamily)
                }

                FontFamilyRow {
                    visible: !Theme.typography.followingDms && SettingsData.fontSource !== "system"
                    text: I18n.tr("Family", "font family dropdown label")
                    keywords: I18n.tr("font typeface", "settings search keywords, space separated, add synonyms in your language")
                    family: SettingsData.fontFamily
                    onFamilyPicked: name => SettingsData.fontFamily = name
                }

                FontFamilyRow {
                    visible: !Theme.typography.followingDms && SettingsData.fontSource !== "system"
                    text: I18n.tr("Monospace family", "monospace font family dropdown label")
                    keywords: I18n.tr("font typeface code mono", "settings search keywords, space separated, add synonyms in your language")
                    defaultFamily: Theme.defaultMonoFontFamily
                    family: SettingsData.monoFontFamily
                    onFamilyPicked: name => SettingsData.monoFontFamily = name
                }

                FontFamilyRow {
                    visible: !Theme.typography.followingDms && SettingsData.fontSource !== "system"
                    text: I18n.tr("Display family", "display font family dropdown label")
                    keywords: I18n.tr("font typeface headline title", "settings search keywords, space separated, add synonyms in your language")
                    defaultFamily: Theme.defaultDisplayFontFamily
                    family: SettingsData.displayFontFamily
                    onFamilyPicked: name => SettingsData.displayFontFamily = name
                }

                SettingsSliderRow {
                    visible: !Theme.typography.followingDms
                    text: I18n.tr("Weight", "font weight slider label")
                    keywords: I18n.tr("font bold typography", "settings search keywords, space separated, add synonyms in your language")
                    minimum: Font.Thin
                    maximum: Font.Black
                    step: 100
                    showStops: true
                    unit: ""
                    value: SettingsData.fontWeight
                    onSliderValueChanged: newValue => SettingsData.fontWeight = newValue
                }

                SettingsSliderRow {
                    visible: !Theme.typography.followingDms
                    text: I18n.tr("Scale", "font scale slider label")
                    keywords: I18n.tr("font size text zoom typography", "settings search keywords, space separated, add synonyms in your language")
                    minimum: 75
                    maximum: 150
                    unit: "%"
                    value: Math.round(SettingsData.fontScale * 100)
                    onSliderValueChanged: newValue => SettingsData.fontScale = newValue / 100
                }
            }

            SettingsCard {
                title: I18n.tr("Windows", "appearance settings section label")

                SettingsToggleRow {
                    text: I18n.tr("Title bars", "window title bars toggle label")
                    keywords: I18n.tr("decorations csd window header", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Off leaves decorations to the compositor. Applies to new windows.", "window title bars toggle description")
                    checked: SettingsData.windowTitleBars
                    onToggled: checked => SettingsData.windowTitleBars = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Window header", "toggle label for the window title and buttons")
                    keywords: I18n.tr("title bar header hide decorations", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Hides the title and the window buttons, leaving a thin strip to drag the window.", "window header toggle description")
                    checked: SettingsData.showTitleBar
                    onToggled: checked => SettingsData.showTitleBar = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Motion", "appearance settings section label")

                SettingsSliderRow {
                    text: I18n.tr("Duration", "animation duration slider label")
                    keywords: I18n.tr("animation speed motion", "settings search keywords, space separated, add synonyms in your language")
                    minimumLabel: I18n.tr("Off", "animation duration slider label at zero")
                    minimum: 0
                    maximum: 1000
                    unit: "ms"
                    value: SettingsData.animationDuration
                    onSliderValueChanged: newValue => SettingsData.animationDuration = newValue
                }

                SettingsButtonGroupRow {
                    text: I18n.tr("Spring", "spring bounce setting label")
                    keywords: I18n.tr("bounce animation motion", "settings search keywords, space separated, add synonyms in your language")
                    model: [I18n.tr("Smooth", "spring bounce option"), I18n.tr("Balanced", "spring bounce option"), I18n.tr("Playful", "spring bounce option")]
                    currentIndex: SettingsData.springBounce
                    onSelectionChanged: (index, selected) => {
                        if (selected)
                            SettingsData.springBounce = index;
                    }
                }

                SettingsToggleRow {
                    text: I18n.tr("Ripple effects", "ripple effects toggle label")
                    keywords: I18n.tr("animation click feedback", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.enableRippleEffects
                    onToggled: checked => SettingsData.enableRippleEffects = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Reduce motion", "reduce motion toggle label")
                    keywords: I18n.tr("accessibility animation disable", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.reduceMotion
                    onToggled: checked => SettingsData.reduceMotion = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Focus ring", "appearance settings section label")

                SettingsToggleRow {
                    text: I18n.tr("Focus ring", "focus ring toggle label")
                    keywords: I18n.tr("keyboard outline accessibility", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.focusRingEnabled
                    onToggled: checked => SettingsData.focusRingEnabled = checked
                }

                SettingsSliderRow {
                    text: I18n.tr("Thickness", "focus ring thickness slider label")
                    keywords: I18n.tr("focus ring width outline", "settings search keywords, space separated, add synonyms in your language")
                    enabled: SettingsData.focusRingEnabled
                    value: Math.round(SettingsData.focusRingWidth * 10)
                    minimum: 10
                    maximum: 40
                    step: 5
                    decimals: 1
                    unit: "px"
                    onSliderValueChanged: newValue => SettingsData.focusRingWidth = newValue / 10
                }
            }

            Loader {
                id: themePickerLoader
                active: false
                sourceComponent: FileBrowserModal {
                    parentModal: root.hostWindow
                    bucket: "theme"
                    filters: ["*.json"]
                    onAccepted: paths => SettingsData.customThemeFile = paths[0]
                }
            }
        }
    }

    Component {
        id: behaviourPage
        SettingsPage {
            SettingsCard {
                title: I18n.tr("Open in terminal", "context menu entry opening a terminal")

                OptionDropdownRow {
                    text: I18n.tr("Terminal", "setting label for the terminal emulator")
                    keywords: I18n.tr("console emulator shell command line", "settings search keywords, space separated, add synonyms in your language")
                    description: SandboxService.sandboxed ? I18n.tr("Name the host terminal to launch.", "terminal setting description inside flatpak") : I18n.tr("Automatic uses $TERMINAL.", "terminal setting description")
                    optionList: root.terminalOptions
                    current: SettingsData.terminal
                    onPicked: value => SettingsData.terminal = value
                }
            }

            SettingsCard {
                title: I18n.tr("Drag and drop", "behaviour settings section label")

                OptionDropdownRow {
                    text: I18n.tr("Dropping without a modifier", "drag default action setting label")
                    keywords: I18n.tr("drag drop copy move ask default action", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Ctrl copies, Alt asks.", "drag default action setting description")
                    optionList: root.dragOptions
                    current: SettingsData.dragAction
                    onPicked: value => SettingsData.dragAction = value
                }
            }

            SettingsCard {
                title: I18n.tr("Thumbnails", "capability row on the about page")

                SettingsToggleRow {
                    text: I18n.tr("Show thumbnails", "thumbnails toggle label")
                    keywords: I18n.tr("previews images video pdf", "settings search keywords, space separated, add synonyms in your language")
                    description: SandboxService.degraded("thumbnails") ? FilesFormat.featureUnavailable(SandboxService.reason("thumbnails")) : ""
                    checked: SettingsData.thumbnails
                    onToggled: checked => SettingsData.thumbnails = checked
                }

                SettingsSliderRow {
                    text: I18n.tr("Skip images larger than", "thumbnail size cap slider label")
                    keywords: I18n.tr("thumbnail size limit cap megabytes", "settings search keywords, space separated, add synonyms in your language")
                    minimumLabel: I18n.tr("No limit", "thumbnail size cap slider label at zero")
                    enabled: SettingsData.thumbnails
                    minimum: 0
                    maximum: 1024
                    step: 16
                    unit: " MB"
                    value: SettingsData.thumbnailSizeLimitMb
                    onSliderValueChanged: newValue => SettingsData.thumbnailSizeLimitMb = newValue
                }

                SettingsToggleRow {
                    text: I18n.tr("Thumbnails on network locations", "network thumbnails toggle label")
                    keywords: I18n.tr("remote sftp smb previews", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Downloads each file.", "network thumbnails toggle description")
                    enabled: SettingsData.thumbnails
                    checked: SettingsData.networkThumbnails
                    onToggled: checked => SettingsData.networkThumbnails = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Search in Files", "placeholder in the search field")

                SettingsToggleRow {
                    text: I18n.tr("Include hidden files", "search hidden files toggle label")
                    keywords: I18n.tr("search dotfiles recursive", "settings search keywords, space separated, add synonyms in your language")
                    checked: SettingsData.searchHidden
                    onToggled: checked => SettingsData.searchHidden = checked
                }

                SettingsRow {
                    title: I18n.tr("Folders to skip", "search exclusions setting label")
                    keywords: I18n.tr("search exclude ignore", "settings search keywords, space separated, add synonyms in your language")
                    subtitle: I18n.tr("Names or paths, comma separated. Build and cache folders are skipped anyway.", "search exclusions setting description")

                    body: DTextField {
                        width: parent.width
                        outlined: true
                        text: SettingsData.searchExcludes.join(", ")
                        placeholderText: I18n.tr("Steam, ~/VirtualBox VMs", "example list in the search exclusions field, folder names or paths separated by commas")
                        onEditingFinished: {
                            const names = text.split(",").map(entry => entry.trim()).filter(entry => entry !== "");
                            const current = SettingsData.searchExcludes;
                            if (names.length !== current.length || names.some((name, index) => name !== current[index]))
                                SettingsData.searchExcludes = names;
                        }
                    }
                }
            }
        }
    }

    Component {
        id: devicesPage
        SettingsPage {
            SettingsGroup {
                SettingsToggleRow {
                    text: I18n.tr("Show system partitions", "devices setting label")
                    keywords: I18n.tr("internal drives udisks home boot", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Such as a separate /home.", "devices setting description")
                    checked: SettingsData.showSystemPartitions
                    onToggled: checked => SettingsData.showSystemPartitions = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Open unmounted drives on click", "devices setting label")
                    keywords: I18n.tr("mount usb removable sidebar", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Off only mounts.", "devices setting description")
                    checked: SettingsData.mountOnClick
                    onToggled: checked => SettingsData.mountOnClick = checked
                }
            }

            SettingsCard {
                title: I18n.tr("Games in the sidebar", "games settings section label")

                SettingsToggleRow {
                    text: I18n.tr("Show games", "games setting label")
                    keywords: I18n.tr("steam heroic lutris proton games sidebar hide", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Only launchers with an installed game are listed.", "games setting description")
                    checked: SettingsData.showGamesSidebar
                    onToggled: checked => SettingsData.showGamesSidebar = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Steam", "games setting label for the steam group")
                    keywords: I18n.tr("steam proton valve", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Libraries and Proton prefixes.", "steam games setting description")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showSteamGames
                    onToggled: checked => SettingsData.showSteamGames = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Heroic", "games setting label for the heroic group")
                    keywords: I18n.tr("heroic epic gog amazon", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Games and prefixes Heroic installed.", "heroic games setting description")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showHeroicGames
                    onToggled: checked => SettingsData.showHeroicGames = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Lutris", "games setting label for the lutris group")
                    keywords: I18n.tr("lutris wine prefix", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Games and prefixes Lutris installed.", "lutris games setting description")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showLutrisGames
                    onToggled: checked => SettingsData.showLutrisGames = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Settings folders", "games setting label hiding the launcher config folders")
                    keywords: I18n.tr("config folder launcher wine prefix", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Each launcher's own configuration folder.", "games setting description for the settings folders")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showGameSettingsFolders
                    onToggled: checked => SettingsData.showGameSettingsFolders = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Prefixes", "games setting label hiding wine prefixes")
                    keywords: I18n.tr("wine prefix proton compatdata", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("The Wine prefix of each game.", "games setting description for the wine prefixes")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showGamePrefixes
                    onToggled: checked => SettingsData.showGamePrefixes = checked
                }

                SettingsToggleRow {
                    text: I18n.tr("Games folders", "games setting label hiding the folders holding the games")
                    keywords: I18n.tr("games library steamapps common", "settings search keywords, space separated, add synonyms in your language")
                    description: I18n.tr("Where the launcher keeps the games.", "games setting description for the games folders")
                    enabled: SettingsData.showGamesSidebar
                    checked: SettingsData.showGameFolders
                    onToggled: checked => SettingsData.showGameFolders = checked
                }
            }

            SettingsRow {
                visible: !SandboxService.available("devices")
                iconName: "info"
                iconColor: Theme.surfaceVariantText
                title: FilesFormat.featureUnavailable(SandboxService.reason("devices"))
            }
        }
    }

    Component {
        id: aboutPage
        SettingsAboutPage {}
    }
}
