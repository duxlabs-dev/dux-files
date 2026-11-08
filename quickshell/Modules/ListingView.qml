pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser
import "../DCommon/FileBrowser/NameFilter.js" as NameFilter

Rectangle {
    id: view

    required property var host
    required property var model
    property string title: ""
    property string summary: {
        if (model.error !== "")
            return FileFormat.listingError(model.errorCode);
        if (model.total === 0)
            return "";
        if (host.nameFilter !== "")
            return FilesFormat.matching(model.count, host.nameFilter);
        return FileFormat.count(model.total);
    }
    property var listColumns: []
    property var columnSortKeys: ({})
    property string emptyText: ""
    property bool showHidden: true
    property var nameHighlight: NameFilter.needle(host.nameFilter)
    property bool filtering: host.nameFilter !== ""
    property bool dragEnabled: false
    property bool navigable: true
    property bool backgroundMenu: true
    property string dropPath: ""

    readonly property alias selection: selectionModel
    readonly property alias body: viewBody
    readonly property bool compact: width < Theme.smallBreakpoint
    default property alias actions: actionsRow.data
    property alias extras: extrasSlot.data

    function focusBody() {
        viewBody.focusBody();
    }

    function refresh() {
        model.refresh();
    }

    function sortBy(key) {
        model.sortBy(key);
    }

    radius: FilesMetrics.paneRadius
    color: Theme.foregroundColor(Theme.cardSurface, true)
    border.width: host.showActiveOutline && host.active ? Theme.outlineWidthFocused : 0
    border.color: Theme.primary

    SelectionModel {
        id: selectionModel
    }

    Item {
        id: heading

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: FilesMetrics.paneBodyMargin
        anchors.leftMargin: FilesMetrics.paneBodyMarginH
        anchors.rightMargin: FilesMetrics.paneBodyMarginH
        height: Math.max(FileBrowserMetrics.pathPillHeight, titleColumn.implicitHeight, actionsRow.implicitHeight)

        Column {
            id: titleColumn

            anchors.left: parent.left
            anchors.right: actionsRow.left
            anchors.rightMargin: Theme.spacingS
            anchors.verticalCenter: parent.verticalCenter
            spacing: Theme.spacingXXS

            StyledText {
                width: parent.width
                text: view.title
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeMedium
                font.weight: Theme.fontWeightMedium
                elide: Text.ElideMiddle
            }

            StyledText {
                width: parent.width
                text: view.summary
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                elide: Text.ElideRight
            }
        }

        Row {
            id: actionsRow

            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Theme.spacingS
        }
    }

    Item {
        id: extrasSlot

        anchors.top: heading.bottom
        anchors.topMargin: height > 0 ? Theme.spacingS : 0
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: FilesMetrics.paneBodyMarginH
        anchors.rightMargin: FilesMetrics.paneBodyMarginH
        height: childrenRect.height
    }

    FileViewBody {
        id: viewBody

        anchors.top: extrasSlot.bottom
        anchors.topMargin: Theme.spacingS
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottomMargin: FilesMetrics.paneBodyMargin
        anchors.leftMargin: FilesMetrics.paneBodyMarginH
        anchors.rightMargin: FilesMetrics.paneBodyMarginH + drawer.width
        focus: true
        directory: view.model
        selection: selectionModel
        viewMode: view.host.viewMode === "columns" ? "list" : view.host.viewMode
        gridZoom: SessionData.gridZoom
        listZoom: SessionData.listZoom
        listColumns: view.listColumns
        columnWidths: SessionData.columnWidths
        columnSortKeys: view.columnSortKeys
        previewKeys: true
        sortKey: view.model.sortKey
        sortDescending: view.model.sortDescending
        showHidden: view.showHidden
        dragEnabled: view.dragEnabled
        dropEnabled: false
        showSelectionFooter: true
        emptyText: view.emptyText
        surfaceColor: view.color
        cutSet: view.host.cutSet
        singleClickActivates: view.host.singleClickActivates
        typeAheadEnabled: view.host.typeAhead
        typeAheadMode: SettingsData.listTyping
        countFolders: SettingsData.folderCounts
        nameHighlight: view.nameHighlight
        filtering: view.filtering
        onFilterRequested: text => view.host.filterRequested(text)
        onFilterFocusRequested: view.host.filterFocusRequested()
        onActivateRequested: entry => view.host.activateRequested(entry)
        onItemMenuRequested: (index, pointX, pointY, modifiers) => {
            view.host.focusRequested();
            const point = mapToItem(view.host, pointX, pointY);
            view.host.requestItemMenu(index, point.x, point.y, modifiers);
        }
        onBackgroundMenuRequested: (pointX, pointY) => {
            if (!view.backgroundMenu)
                return;
            view.host.focusRequested();
            const point = mapToItem(view.host, pointX, pointY);
            view.host.requestBackgroundMenu(point.x, point.y);
        }
        onDragRequested: paths => view.host.dragRequested(paths)
        onRenameRequested: (target, name) => view.host.renameRequested(target, name)
        onSortRequested: key => view.sortBy(key)
        onZoomRequested: (mode, level) => view.host.setZoom(mode, level)
        onColumnResizeRequested: (id, width) => view.host.setColumnWidth(id, width)
        onPreviewKeyRequested: (action, step) => {
            if (action === "toggle") {
                view.host.previewToggleRequested();
                return;
            }
            drawer.handleKey(action, step);
        }
        onFocusRequested: view.host.focusRequested()
        onTabRequested: target => {
            if (view.navigable)
                view.host.tabRequested(target);
        }
        onOpenRequested: target => {
            if (!view.navigable || target === "..")
                return;
            view.host.focusRequested();
            view.host.activateRequested({
                "path": target,
                "isDir": true
            });
        }
    }

    FilePreviewDrawer {
        id: drawer

        anchors.top: viewBody.top
        anchors.bottom: viewBody.bottom
        anchors.right: parent.right
        anchors.rightMargin: FilesMetrics.paneBodyMarginH
        body: viewBody
        open: view.host.previewOpen
        available: view.width - FilesMetrics.paneBodyMarginH * 2
        preferredWidth: CacheData.previewWidth
        onResized: width => view.host.setPreviewWidth(width)
        onFocusRequested: view.host.focusRequested()
    }

    Rectangle {
        anchors.fill: parent
        radius: parent.radius
        visible: dropZone.containsDrag
        color: Theme.withAlpha(Theme.primary, Theme.stateLayerDrag)
    }

    FileDropArea {
        id: dropZone

        anchors.fill: viewBody
        dropPath: view.dropPath
        enabled: dropPath !== ""
        onDropped: drop => view.host.dropOn(view.dropPath, drop)
    }

    TapHandler {
        enabled: view.navigable
        acceptedButtons: Qt.BackButton | Qt.ForwardButton
        onTapped: (point, button) => {
            view.host.focusRequested();
            if (button === Qt.BackButton) {
                view.host.back();
                return;
            }
            view.host.forward();
        }
    }
}
