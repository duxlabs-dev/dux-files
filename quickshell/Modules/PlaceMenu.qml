import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FileMenu {
    id: menu

    property string path: ""
    property string place: "folder"

    readonly property bool bookmark: place === "bookmark"
    readonly property bool trash: place === "trash"
    readonly property bool recent: place === "recent"
    readonly property bool folder: place === "folder"

    signal tabRequested(string path)
    signal windowRequested(string path)
    signal renameRequested(string path)
    signal removeRequested(string path)
    signal copyPathRequested(string path)
    signal propertiesRequested(string path)
    signal emptyTrashRequested
    signal clearRecentRequested

    function present(kind, target, source, pointX, pointY) {
        place = kind;
        path = target;
        const point = source.mapToItem(anchorItem, pointX, pointY);
        openAt(point.x, point.y);
    }

    FileMenuItem {
        text: FileActions.label("openInNewTab")
        iconName: FileActions.icon("openInNewTab")
        onClicked: {
            menu.close();
            menu.tabRequested(menu.path);
        }
    }

    FileMenuItem {
        text: FileActions.label("openInNewWindow")
        iconName: FileActions.icon("openInNewWindow")
        onClicked: {
            menu.close();
            menu.windowRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.bookmark
        text: FileActions.label("rename")
        iconName: FileActions.icon("rename")
        onClicked: {
            menu.close();
            menu.renameRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.bookmark
        text: FileActions.label("removeBookmark")
        iconName: FileActions.icon("removeBookmark")
        onClicked: {
            menu.close();
            menu.removeRequested(menu.path);
        }
    }

    FileMenuDivider {
        visible: !menu.bookmark
    }

    FileMenuItem {
        visible: menu.folder
        text: FileActions.label("copyPath")
        iconName: FileActions.icon("copyPath")
        onClicked: {
            menu.close();
            menu.copyPathRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.folder
        text: FileActions.label("properties")
        iconName: FileActions.icon("properties")
        onClicked: {
            menu.close();
            menu.propertiesRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.trash
        text: FileActions.label("emptyTrash")
        iconName: FileActions.icon("emptyTrash")
        danger: FileActions.danger("emptyTrash")
        enabled: TrashService.count > 0
        onClicked: {
            menu.close();
            menu.emptyTrashRequested();
        }
    }

    FileMenuItem {
        visible: menu.recent
        text: FileActions.label("clearRecent")
        iconName: FileActions.icon("clearRecent")
        enabled: RecentService.count !== 0
        onClicked: {
            menu.close();
            menu.clearRecentRequested();
        }
    }
}
