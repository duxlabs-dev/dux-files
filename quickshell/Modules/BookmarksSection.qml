pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FilePlaceSection {
    id: section

    property string currentPath: ""

    signal navigateRequested(string path)
    signal tabRequested(string path)
    signal dropRequested(string path, var drop)
    signal menuRequested(string path, real pointX, real pointY)
    signal trashMenuRequested(real pointX, real pointY)
    signal recentMenuRequested(real pointX, real pointY)

    readonly property var bookmarks: BookmarksService.entries.filter(entry => entry.path !== RecentService.path || RecentService.shown)

    function kindOf(path) {
        switch (path) {
        case RecentService.path:
            return "recent";
        case TrashService.path:
            return "trash";
        default:
            return "bookmark";
        }
    }

    function targetOf(entry) {
        return entry.path === TrashService.path ? TrashService.filesPath : entry.path;
    }

    function fullOrder(visible) {
        const all = BookmarksService.paths;
        const order = visible.slice();
        all.forEach((path, index) => {
            if (!order.includes(path))
                order.splice(index, 0, path);
        });
        return order;
    }

    function placeFor(path) {
        return DirectoryService.userDirs.find(dir => dir.path === path) ?? null;
    }

    function labelFor(entry) {
        const place = placeFor(entry.path);
        if (place?.key === "home" && entry.label === place.name)
            return I18n.tr("Home", "sidebar entry for the user's home folder");
        return entry.label;
    }

    function iconFor(entry) {
        return entry.icon || placeFor(entry.path)?.iconName || "folder";
    }

    title: I18n.tr("Places", "sidebar section listing the user's places and saved folders")

    DReorderList {
        id: reorderList

        width: section.width
        model: section.bookmarks
        spacing: FileBrowserMetrics.sidebarRowGap
        onReordered: indices => BookmarksService.reorder(section.fullOrder(indices.map(index => section.bookmarks[index].path)))

        delegate: FilePlaceRow {
            id: row

            required property var modelData
            required property int index

            readonly property string kind: section.kindOf(modelData.path)
            readonly property bool recent: kind === "recent"
            readonly property bool trash: kind === "trash"
            readonly property bool saved: kind === "bookmark"
            readonly property string target: section.targetOf(modelData)
            readonly property bool dragging: reorderList.draggingIndex === index

            width: reorderList.width
            label: recent ? I18n.tr("Recent", "recent files view header and sidebar entry") : trash ? I18n.tr("Trash", "trash view header and sidebar entry") : section.labelFor(modelData)
            glyph: recent ? "history" : ""
            iconName: recent ? "" : trash ? (TrashService.count > 0 ? "user-trash-full" : "user-trash") : section.iconFor(modelData)
            tooltipText: {
                if (recent)
                    return RecentService.count < 0 ? "" : RecentService.count > 0 ? FileFormat.count(RecentService.count) : I18n.tr("No recent files", "recent files view placeholder when nothing was opened yet");
                if (trash)
                    return TrashService.count > 0 ? FileFormat.count(TrashService.count) : I18n.tr("Trash is empty", "trash view placeholder and sidebar tooltip when nothing is trashed");
                return "";
            }
            selected: target === section.currentPath
            dropPath: recent ? "" : target
            springLoad: saved
            z: dragging ? 1 : 0
            opacity: dragging ? Theme.pendingOpacity : 1
            onClicked: section.navigateRequested(target)
            onMiddleClicked: section.tabRequested(target)
            onDropped: drop => section.dropRequested(target, drop)
            onSpringLoaded: section.navigateRequested(target)

            Behavior on y {
                enabled: reorderList.animateLayout && !row.dragging && Theme.animationsEnabled
                NumberAnimation {
                    duration: Theme.expressiveDurations.expressiveDefaultSpatial
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: Theme.expressiveCurves.expressiveFastSpatial
                }
            }

            TapHandler {
                acceptedButtons: Qt.RightButton
                onTapped: point => {
                    const mapped = row.mapToItem(section, point.position.x, point.position.y);
                    switch (row.kind) {
                    case "recent":
                        section.recentMenuRequested(mapped.x, mapped.y);
                        return;
                    case "trash":
                        section.trashMenuRequested(mapped.x, mapped.y);
                        return;
                    default:
                        section.menuRequested(row.modelData.path, mapped.x, mapped.y);
                    }
                }
            }

            DragHandler {
                target: null
                xAxis.enabled: false
                enabled: reorderList.count > 1
                onGrabChanged: (transition, point) => {
                    switch (transition) {
                    case PointerDevice.GrabExclusive:
                        reorderList.begin(row.index, reorderList.mapFromItem(null, point.scenePressPosition.x, point.scenePressPosition.y));
                        reorderList.dragTo(reorderList.mapFromItem(null, point.scenePosition.x, point.scenePosition.y));
                        return;
                    case PointerDevice.UngrabExclusive:
                        reorderList.finish();
                        return;
                    case PointerDevice.CancelGrabExclusive:
                        reorderList.cancel();
                        return;
                    }
                }
                onCentroidChanged: {
                    if (!active || !row.dragging)
                        return;
                    reorderList.dragTo(reorderList.mapFromItem(null, centroid.scenePosition.x, centroid.scenePosition.y));
                }
            }
        }
    }
}
