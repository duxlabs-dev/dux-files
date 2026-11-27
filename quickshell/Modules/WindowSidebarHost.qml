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
    property real dockedTop: 0

    readonly property bool compact: width < FilesMetrics.sidebarOverlayBreakpoint
    readonly property bool docked: !compact
    readonly property bool rail: docked && SettingsData.sidebarCollapsed
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
        onRefreshRequested: host.mainWindow.activePane.reload()
        onNetworkMountMenuRequested: (id, path, pointX, pointY) => host.mainWindow.menus.presentNetworkMountMenu(id, path, sidebar, pointX, pointY)
        onBookmarkMenuRequested: (path, pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("bookmark", path, sidebar, pointX, pointY)
        onTrashMenuRequested: (pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("trash", TrashService.filesPath, sidebar, pointX, pointY)
        onRecentMenuRequested: (pointX, pointY) => host.mainWindow.menus.presentPlaceMenu("recent", RecentService.path, sidebar, pointX, pointY)
        onBookmarkRenameRequested: path => host.mainWindow.dialogs.renameBookmark(path)
        onLeaveRequested: host.leave()
        onDropRequested: (path, drop) => {
            if (DragService.extractArkDrop(drop, path))
                return;
            host.mainWindow.drag.applyDrop(DragService.pathsFrom(drop), path, DragService.overrideFor(drop.proposedAction));
        }
    }

    Item {
        id: sidebarSlot

        readonly property real maximumWidth: Math.max(FilesMetrics.sidebarMinWidth, host.width - host.contentMinWidth)
        readonly property real expandedWidth: Math.max(FilesMetrics.sidebarMinWidth, Math.min(maximumWidth, FilesMetrics.sidebarWidth))
        readonly property real layoutWidth: width > FilesMetrics.sidebarRailWidth ? expandedWidth : width

        anchors.top: parent.top
        anchors.topMargin: host.dockedTop
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        width: {
            if (!host.docked)
                return 0;
            return host.rail ? FilesMetrics.sidebarRailWidth : expandedWidth;
        }
        clip: true

        Behavior on width {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: Theme.expressiveDurations.expressiveSlowEffects
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveSlowEffects
            }
        }

        WindowSidebar {
            id: dockedSidebar

            anchors.top: parent.top
            anchors.bottom: dockedJobs.top
            anchors.left: parent.left
            anchors.bottomMargin: dockedJobs.visible ? FilesMetrics.sidebarSectionSpacing : 0
            width: Math.max(0, parent.layoutWidth - FilesMetrics.sidebarPadding * 2)
            visible: parent.width > 0
        }

        SidebarJobs {
            id: dockedJobs

            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.margins: FilesMetrics.sidebarPadding
            width: dockedSidebar.width
            maximumHeight: parent.height * FilesMetrics.sidebarJobsShare
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
                duration: Theme.expressiveDurations.expressiveFastEffects
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveFastEffects
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
        width: Math.max(FilesMetrics.sidebarMinWidth, Math.min(host.width - FilesMetrics.contentMarginH * 2, FilesMetrics.sidebarWidth))
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
            anchors.topMargin: FilesMetrics.sidebarPadding
        }
    }
}
