pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.FileBrowser

Singleton {
    id: root

    readonly property real windowMinWidth: Theme.smallBreakpoint
    readonly property real windowMinHeight: Theme.smallBreakpoint * 2 / 3
    readonly property real sidebarWidth: FileBrowserMetrics.sidebarWidth
    readonly property real sidebarMinWidth: sidebarWidth * 0.75
    readonly property real sidebarPadding: FileBrowserMetrics.sidebarPadding
    readonly property real sidebarSectionSpacing: Theme.spacingL
    readonly property int sidebarOverlayBreakpoint: FileBrowserMetrics.sidebarBreakpoint

    readonly property real topbarControlSize: FileBrowserMetrics.controlSize
    readonly property real topbarSpacing: FileBrowserMetrics.controlSpacing
    readonly property real topbarGroupSpacing: Theme.spacingXS

    readonly property real contentMarginV: FileBrowserMetrics.contentMargin
    readonly property real contentMarginH: contentMarginV
    readonly property real contentSpacing: FileBrowserMetrics.contentSpacing
    readonly property real paneRadius: FileBrowserMetrics.paneRadius
    readonly property real paneBodyMargin: FileBrowserMetrics.paneMargin
    readonly property real paneBodyMarginH: FileBrowserMetrics.paneMarginHorizontal
    readonly property real splitterWidth: FileBrowserMetrics.splitterWidth
    readonly property real splitterHandleHeight: FileBrowserMetrics.splitterHandleHeight
    readonly property real minimumPaneFraction: 0.2

    readonly property real tabHeight: Theme.buttonHeightS
    readonly property int tabShortcutCount: 9
    readonly property int closedTabHistory: 10
    readonly property int defaultGridZoom: FileBrowserMetrics.defaultGridZoom
    readonly property int defaultListZoom: FileBrowserMetrics.defaultListZoom
    readonly property int defaultColumnZoom: FileBrowserMetrics.defaultColumnZoom

    function defaultZoom(viewMode) {
        return FileBrowserMetrics.defaultZoom(viewMode);
    }
    readonly property real tabMinWidth: Theme.tabMinWidth
    readonly property real tabMaxWidth: Theme.sidebarWidth
    readonly property real tabSpacing: Theme.spacingXXS
    readonly property int tabHoverDelay: FileBrowserMetrics.springLoadDelay

    readonly property real dragGhostSize: FileBrowserMetrics.gridIconSizes[0]
    readonly property int openConfirmLimit: 10

    readonly property real popoverWidth: Theme.smallBreakpoint * 0.75
    readonly property real propertiesIconSize: FileBrowserMetrics.listIconSizes[3]
    readonly property real propertiesLabelWidth: FileBrowserMetrics.detailLabelWidth

    readonly property int folderHistoryWriteDelay: 10000
    readonly property int folderVerifyInterval: 60000
}
