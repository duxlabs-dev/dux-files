import QtQuick
import Quickshell
import qs.Common
import qs.Modules
import qs.DCommon.Widgets

FloatingWindow {
    id: settingsModal

    property int currentTabIndex: 0
    property bool isCompactMode: width < SettingsMetrics.compactBreakpoint
    property bool menuVisible: false
    readonly property bool compactSearch: isCompactMode && menuVisible && content.searching

    function show() {
        visible = true;
    }

    function hide() {
        visible = false;
        sidebar.clearSearch();
    }

    function reveal(page, row) {
        currentTabIndex = page;
        sidebar.clearSearch();
        if (isCompactMode)
            menuVisible = false;
        content.revealRow(row);
    }

    onClosed: hide()

    WindowBlur {
        targetWindow: settingsModal
        blurEnabled: Theme.translucentWindows
        blurWidth: settingsModal.width
        blurHeight: settingsModal.height
    }

    title: I18n.tr("Files Settings", "settings window title")
    minimumSize: Qt.size(SettingsMetrics.windowMinWidth, SettingsMetrics.windowMinHeight)
    implicitWidth: screen ? Math.min(SettingsMetrics.windowWidth, screen.width - Theme.spacingXL * 2) : SettingsMetrics.windowWidth
    implicitHeight: screen ? Math.min(SettingsMetrics.windowHeight, screen.height - Theme.spacingXL * 2) : SettingsMetrics.windowHeight
    color: Theme.windowSurface
    surfaceFormat.opaque: false
    visible: false

    onIsCompactModeChanged: menuVisible = false

    FocusScope {
        anchors.fill: parent
        focus: true

        readonly property bool isFloatingWindowSurface: true

        LayoutMirroring.enabled: I18n.isRtl
        LayoutMirroring.childrenInherit: true

        Keys.onPressed: event => {
            if (event.key === Qt.Key_Escape) {
                if (!sidebar.clearSearch() && !sidebar.clearHighlight())
                    settingsModal.hide();
                event.accepted = true;
                return;
            }
            if (contentScope.activeFocus || !sidebar.visible)
                return;
            switch (event.key) {
            case Qt.Key_Down:
            case Qt.Key_J:
                sidebar.moveHighlight(1);
                break;
            case Qt.Key_Up:
            case Qt.Key_K:
                sidebar.moveHighlight(-1);
                break;
            case Qt.Key_Return:
            case Qt.Key_Enter:
                if (!sidebar.selectHighlighted())
                    return;
                break;
            default:
                return;
            }
            event.accepted = true;
        }

        Column {
            anchors.fill: parent
            spacing: 0

            DWindowHeader {
                id: header
                width: parent.width
                z: 10
                controls: windowControls
                title: I18n.tr("Settings", "settings window header title")
                onCloseRequested: settingsModal.hide()

                DActionButton {
                    visible: settingsModal.isCompactMode
                    iconName: "menu"
                    buttonSize: Theme.buttonHeightXXS
                    Accessible.name: I18n.tr("Menu", "settings header button that shows the section list on narrow windows")
                    onClicked: settingsModal.menuVisible = !settingsModal.menuVisible
                }
            }

            Item {
                width: parent.width
                height: parent.height - header.height
                clip: true

                SettingsSidebar {
                    id: sidebar
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: settingsModal.compactSearch ? undefined : parent.bottom
                    height: settingsModal.compactSearch ? implicitHeight : undefined
                    width: settingsModal.isCompactMode ? parent.width : implicitWidth
                    visible: settingsModal.isCompactMode ? settingsModal.menuVisible : true
                    fieldOnly: settingsModal.compactSearch
                    currentIndex: settingsModal.currentTabIndex
                    matchCounts: content.matchCounts
                    onTabSelected: index => {
                        settingsModal.currentTabIndex = index;
                        sidebar.clearSearch();
                        if (settingsModal.isCompactMode)
                            settingsModal.menuVisible = false;
                    }
                    onSearchAccepted: content.pickFirst()
                }

                FocusScope {
                    id: contentScope
                    anchors.left: settingsModal.isCompactMode ? (settingsModal.menuVisible && !settingsModal.compactSearch ? sidebar.right : parent.left) : sidebar.right
                    anchors.right: parent.right
                    anchors.top: settingsModal.compactSearch ? sidebar.bottom : parent.top
                    anchors.bottom: parent.bottom
                    clip: true

                    SettingsContent {
                        id: content

                        anchors.fill: parent
                        currentIndex: settingsModal.currentTabIndex
                        hostWindow: settingsModal
                        searchQuery: sidebar.query
                        onResultPicked: (page, row) => settingsModal.reveal(page, row)
                    }
                }
            }
        }
    }

    FloatingWindowControls {
        id: windowControls
        targetWindow: settingsModal
    }

    Toast {
        anchors.fill: parent
        z: 102
    }
}
