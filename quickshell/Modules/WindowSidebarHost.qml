pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Common as DC

Item {
    id: host

    required property var mainWindow
    property Item focusItem: null
    property bool overlayOpen: false
    property bool focusPending: false
    property real contentMinWidth: FilesMetrics.windowMinWidth

    readonly property bool compact: width < FilesMetrics.sidebarOverlayBreakpoint
    readonly property bool docked: !SettingsData.sidebarCollapsed && !compact
    readonly property bool focused: ItemTree.isInside(focusItem, dockedSidebar) || ItemTree.isInside(focusItem, overlaySidebar)
    readonly property alias dockedWidth: sidebarSlot.width

    onCompactChanged: {
        if (!compact)
            overlayOpen = false;
    }

    function toggle() {
        if (compact) {
            overlayOpen = !overlayOpen;
            return;
        }
        SettingsData.sidebarCollapsed = !SettingsData.sidebarCollapsed;
    }

    function takeFocus() {
        if (compact)
            overlayOpen = true;
        else
            SettingsData.sidebarCollapsed = false;
        const sidebar = compact ? overlaySidebar : dockedSidebar;
        const shown = compact ? sidebarOverlay.visible : dockedSidebar.visible;
        if (!shown) {
            focusPending = true;
            return;
        }
        sidebar.focusCurrent();
    }

    function focusIfPending(sidebar) {
        if (!focusPending)
            return;
        focusPending = false;
        sidebar.focusCurrent();
    }

    function leave() {
        overlayOpen = false;
        mainWindow.activePane.focusBody();
    }

    function dockedScrollerAt(item, x, y) {
        if (!docked)
            return null;
        const point = sidebarSlot.mapFromItem(item, x, y);
        const inside = point.x >= 0 && point.x <= sidebarSlot.width && point.y >= 0;
        return inside ? dockedSidebar : null;
    }

    component WindowSidebar: Sidebar {
        id: sidebar

        anchors.margins: FilesMetrics.sidebarPadding
        anchors.topMargin: 0
        currentPath: host.mainWindow.currentPath
        onVisibleChanged: {
            if (visible)
                host.focusIfPending(sidebar);
        }
        onNavigateRequested: path => {
            host.mainWindow.openPath(path, false);
            host.leave();
        }
        onTabRequested: path => host.mainWindow.tabManager.newTab(path, true)
        onUnlockRequested: device => host.mainWindow.dialogs.askUnlock(device)
        onConnectRequested: host.mainWindow.dialogs.askServer()
        onBookmarkMenuRequested: (path, pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("bookmark", path, sidebar, pointX, pointY)
        onTrashMenuRequested: (pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("trash", TrashService.filesPath, sidebar, pointX, pointY)
        onRecentMenuRequested: (pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("recent", RecentService.path, sidebar, pointX, pointY)
        onBookmarkRenameRequested: path => host.mainWindow.dialogs.renameBookmark(path)
        onLeaveRequested: host.leave()
        onDropRequested: (path, drop) => host.mainWindow.drag.applyDrop(DragService.pathsFrom(drop), path, DragService.overrideFor(drop.proposedAction))
    }

    Item {
        id: sidebarSlot

        readonly property real maximumWidth: Math.max(FilesMetrics.sidebarMinWidth, host.width - host.contentMinWidth)
        property real preferredWidth: CacheData.sidebarWidth

        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        width: host.docked ? Math.max(FilesMetrics.sidebarMinWidth, Math.min(maximumWidth, preferredWidth)) : 0
        clip: true

        Connections {
            target: CacheData

            function onSidebarWidthChanged() {
                if (!sidebarResizeArea.pressed)
                    sidebarSlot.preferredWidth = CacheData.sidebarWidth;
            }
        }

        Behavior on width {
            enabled: Theme.animationsEnabled && !sidebarResizeArea.pressed
            NumberAnimation {
                duration: Theme.expressiveDurations.expressiveFastSpatial
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.standard
            }
        }

        WindowSidebar {
            id: dockedSidebar

            anchors.fill: parent
            visible: parent.width > 0
        }
    }

    Item {
        id: sidebarResizer

        anchors.left: sidebarSlot.right
        anchors.leftMargin: -width / 2
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: FilesMetrics.splitterWidth
        visible: host.docked
        z: 1

        Rectangle {
            anchors.centerIn: parent
            width: Theme.outlineWidthFocused
            height: FilesMetrics.splitterHandleHeight
            radius: Theme.cornerRadiusFull
            color: Theme.primary
            visible: sidebarResizeArea.containsMouse || sidebarResizeArea.pressed
        }

        MouseArea {
            id: sidebarResizeArea

            property real pressX: 0
            property real startWidth: 0

            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.SplitHCursor
            acceptedButtons: Qt.LeftButton
            onPressed: mouse => {
                pressX = mapToItem(host, mouse.x, 0).x;
                startWidth = sidebarSlot.width;
            }
            onPositionChanged: mouse => {
                if (!pressed)
                    return;
                const delta = mapToItem(host, mouse.x, 0).x - pressX;
                const width = startWidth + (I18n.isRtl ? -delta : delta);
                sidebarSlot.preferredWidth = Math.max(FilesMetrics.sidebarMinWidth, Math.min(sidebarSlot.maximumWidth, width));
            }
            onReleased: CacheData.sidebarWidth = Math.round(sidebarSlot.preferredWidth)
            onCanceled: sidebarSlot.preferredWidth = CacheData.sidebarWidth
            onDoubleClicked: {
                sidebarSlot.preferredWidth = FilesMetrics.sidebarWidth;
                CacheData.sidebarWidth = Math.round(FilesMetrics.sidebarWidth);
            }
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: opacity > 0
        opacity: host.overlayOpen ? 1 : 0
        color: Theme.withAlpha(Theme.scrimColor, Theme.scrimAlpha)

        Behavior on opacity {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: Theme.shortDuration
                easing.type: Theme.standardEasing
            }
        }

        MouseArea {
            anchors.fill: parent
            onClicked: host.overlayOpen = false
        }
    }

    Rectangle {
        id: sidebarOverlay

        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: Math.max(FilesMetrics.sidebarMinWidth, Math.min(host.width - FilesMetrics.contentMarginH * 2, CacheData.sidebarWidth))
        visible: x > -width
        x: host.overlayOpen ? 0 : -width
        color: Theme.foregroundColor(Theme.surfaceContainerLow, true)
        topRightRadius: Theme.windowRadius
        bottomRightRadius: Theme.windowRadius

        DC.ElevationShadow {
            anchors.fill: parent
            z: -1
            level: Theme.elevationLevel1
            targetRadius: Theme.windowRadius
            targetColor: sidebarOverlay.color
            shadowEnabled: Theme.elevationEnabled
        }

        Behavior on x {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: Theme.expressiveDurations.expressiveDefaultSpatial
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveDefaultSpatial
            }
        }

        WindowSidebar {
            id: overlaySidebar

            anchors.fill: parent
        }
    }
}
