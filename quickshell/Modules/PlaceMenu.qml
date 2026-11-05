import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FileMenu {
    id: menu

    property string path: ""
    property string place: "folder"
    property string networkMountId: ""
    property bool networkMountMounted: false

    readonly property bool bookmark: place === "bookmark"
    readonly property bool trash: place === "trash"
    readonly property bool recent: place === "recent"
    readonly property bool folder: place === "folder"
    readonly property bool network: place === "network"

    signal tabRequested(string path)
    signal windowRequested(string path)
    signal renameRequested(string path)
    signal removeRequested(string path)
    signal copyPathRequested(string path)
    signal propertiesRequested(string path)
    signal unmountRequested(string id)
    signal emptyTrashRequested
    signal clearRecentRequested

    function present(kind, target, source, pointX, pointY, mountId, mountMounted) {
        place = kind;
        path = target;
        networkMountId = mountId || "";
        networkMountMounted = mountMounted === true;
        const point = source.mapToItem(anchorItem, pointX, pointY);
        openAt(point.x, point.y);
    }

    FileMenuItem {
        text: I18n.tr("Open in new tab", "context menu entry opening a folder in a new tab")
        iconName: "tab"
        onClicked: {
            menu.close();
            menu.tabRequested(menu.path);
        }
    }

    FileMenuItem {
        text: I18n.tr("Open in new window", "context menu entry opening a folder in another window")
        iconName: "open_in_new"
        onClicked: {
            menu.close();
            menu.windowRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.bookmark
        text: I18n.tr("Rename", "rename dialog header")
        iconName: "edit"
        onClicked: {
            menu.close();
            menu.renameRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.bookmark
        text: I18n.tr("Remove bookmark", "tooltip on the bookmark button")
        iconName: "bookmark_remove"
        onClicked: {
            menu.close();
            menu.removeRequested(menu.path);
        }
    }

    FileMenuDivider {
        visible: !menu.bookmark
    }

    FileMenuItem {
        visible: menu.network && menu.networkMountMounted
        text: I18n.tr("Unmount", "context menu action for a mounted network folder")
        iconName: "eject"
        onClicked: {
            menu.close();
            menu.unmountRequested(menu.networkMountId);
        }
    }

    FileMenuItem {
        visible: menu.folder
        text: I18n.tr("Copy location path", "menu entry copying the current path to the clipboard")
        iconName: "link"
        onClicked: {
            menu.close();
            menu.copyPathRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.folder
        text: I18n.tr("Properties", "menu entry showing details about the current folder")
        iconName: "info"
        onClicked: {
            menu.close();
            menu.propertiesRequested(menu.path);
        }
    }

    FileMenuItem {
        visible: menu.trash
        text: I18n.tr("Empty Trash", "action deleting everything in the trash")
        iconName: "delete_forever"
        danger: true
        enabled: TrashService.count > 0
        onClicked: {
            menu.close();
            menu.emptyTrashRequested();
        }
    }

    FileMenuItem {
        visible: menu.recent
        text: I18n.tr("Clear Recent", "action emptying the recently used list without touching the files")
        iconName: "delete_sweep"
        enabled: RecentService.count !== 0
        onClicked: {
            menu.close();
            menu.clearRecentRequested();
        }
    }
}
