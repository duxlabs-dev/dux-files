pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

DFlickable {
    id: sidebar

    property string currentPath: ""

    signal navigateRequested(string path)
    signal tabRequested(string path)
    signal unlockRequested(var device)
    signal connectRequested
    signal dropRequested(string path, var drop)
    signal bookmarkMenuRequested(string path, real pointX, real pointY)
    signal bookmarkRenameRequested(string path)
    signal trashMenuRequested(real pointX, real pointY)
    signal recentMenuRequested(real pointX, real pointY)
    signal leaveRequested

    function rows() {
        const found = [];
        collectRows(sections, found);
        return found;
    }

    function collectRows(item, out) {
        for (const child of item.children) {
            if (!child.visible)
                continue;
            if (child instanceof FilePlaceRow) {
                out.push(child);
                continue;
            }
            collectRows(child, out);
        }
    }

    function focusedRow() {
        const item = Window.activeFocusItem;
        return item instanceof FilePlaceRow && rows().includes(item) ? item : null;
    }

    function focusRow(row) {
        if (!row)
            return;
        row.forceActiveFocus(Qt.TabFocusReason);
        const top = row.mapToItem(contentItem, 0, 0).y;
        const bottom = top + row.height;
        if (top < contentY) {
            contentY = top;
            return;
        }
        if (bottom > contentY + height)
            contentY = bottom - height;
    }

    function focusCurrent() {
        const all = rows();
        focusRow(all.find(row => row.selected) ?? all[0] ?? null);
    }

    function moveFocus(step) {
        const all = rows();
        const index = all.indexOf(focusedRow());
        if (index < 0) {
            focusRow(all[0] ?? null);
            return;
        }
        focusRow(all[Math.max(0, Math.min(all.length - 1, index + step))]);
    }

    function focusEdge(last) {
        const all = rows();
        focusRow(last ? all[all.length - 1] ?? null : all[0] ?? null);
    }

    function bookmarkPath(row) {
        return row && row.dropPath !== "" && BookmarksService.has(row.dropPath) ? row.dropPath : "";
    }

    contentHeight: sections.height + FilesMetrics.sidebarPadding
    clip: true
    onVisibleChanged: {
        if (visible)
            DevicesService.refreshUsage(false);
    }

    Keys.onPressed: event => {
        const row = focusedRow();
        const shift = (event.modifiers & Qt.ShiftModifier) !== 0;
        const key = event.key === Qt.Key_F10 && shift ? Qt.Key_Menu : event.key;
        switch (key) {
        case Qt.Key_Down:
        case Qt.Key_J:
            moveFocus(1);
            break;
        case Qt.Key_Up:
        case Qt.Key_K:
            moveFocus(-1);
            break;
        case Qt.Key_Home:
            focusEdge(false);
            break;
        case Qt.Key_End:
            focusEdge(true);
            break;
        case Qt.Key_Delete:
            if (bookmarkPath(row) !== "") {
                BookmarksService.remove(row.dropPath);
                break;
            }
            if (!row || row.actionIcon === "" || row.busy)
                return;
            row.actionTriggered();
            break;
        case Qt.Key_F2:
            if (bookmarkPath(row) === "")
                return;
            bookmarkRenameRequested(row.dropPath);
            break;
        case Qt.Key_Menu:
            if (row?.kind === "trash") {
                const point = row.mapToItem(sidebar, row.width / 2, row.height / 2);
                trashMenuRequested(point.x, point.y);
                break;
            }
            if (row?.kind === "recent") {
                const point = row.mapToItem(sidebar, row.width / 2, row.height / 2);
                recentMenuRequested(point.x, point.y);
                break;
            }
            if (bookmarkPath(row) === "")
                return;
            {
                const point = row.mapToItem(sidebar, row.width / 2, row.height / 2);
                bookmarkMenuRequested(row.dropPath, point.x, point.y);
            }
            break;
        case Qt.Key_Escape:
            leaveRequested();
            break;
        default:
            return;
        }
        event.accepted = true;
    }

    Column {
        id: sections

        width: sidebar.width
        spacing: FilesMetrics.sidebarSectionSpacing

        BookmarksSection {
            id: bookmarksSection

            width: sections.width
            currentPath: sidebar.currentPath
            onNavigateRequested: path => sidebar.navigateRequested(path)
            onTabRequested: path => sidebar.tabRequested(path)
            onDropRequested: (path, drop) => sidebar.dropRequested(path, drop)
            onMenuRequested: (path, pointX, pointY) => {
                const point = bookmarksSection.mapToItem(sidebar, pointX, pointY);
                sidebar.bookmarkMenuRequested(path, point.x, point.y);
            }
            onTrashMenuRequested: (pointX, pointY) => {
                const point = bookmarksSection.mapToItem(sidebar, pointX, pointY);
                sidebar.trashMenuRequested(point.x, point.y);
            }
            onRecentMenuRequested: (pointX, pointY) => {
                const point = bookmarksSection.mapToItem(sidebar, pointX, pointY);
                sidebar.recentMenuRequested(point.x, point.y);
            }
        }

        DevicesSection {
            width: sections.width
            currentPath: sidebar.currentPath
            onNavigateRequested: path => sidebar.navigateRequested(path)
            onTabRequested: path => sidebar.tabRequested(path)
            onUnlockRequested: device => sidebar.unlockRequested(device)
            onDropRequested: (path, drop) => sidebar.dropRequested(path, drop)
        }

        NetworkSection {
            width: sections.width
            onNavigateRequested: path => sidebar.navigateRequested(path)
            onTabRequested: path => sidebar.tabRequested(path)
            onConnectRequested: sidebar.connectRequested()
            onDropRequested: (path, drop) => sidebar.dropRequested(path, drop)
        }
    }
}
