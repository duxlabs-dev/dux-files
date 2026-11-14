import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Item {
    id: controller

    required property Item anchorItem
    required property string opScope
    required property string currentPath

    signal trashRequested(var paths, string subject)

    function applyDrop(paths, dest, action) {
        if (dest === TrashService.filesPath) {
            trashDropped(paths);
            return;
        }
        switch (action) {
        case "copy":
            FileOpsService.copy(paths, dest, opScope);
            return;
        case "move":
            if (DragService.allInside(paths, dest))
                return;
            FileOpsService.move(paths, dest, opScope);
            return;
        case "ask":
            dropMenu.present(paths, dest);
            return;
        }
        if (SettingsData.dragAction !== "auto") {
            applyDrop(paths, dest, SettingsData.dragAction);
            return;
        }
        DragService.plainDrop(paths, dest, resolved => controller.applyDrop(paths, dest, resolved));
    }

    function beginDrag(paths, entry) {
        if (paths.length === 0)
            return;
        DragService.begin(paths, currentPath);
        const drag = DragService.generation;
        dragGhost.count = paths.length;
        dragGhost.iconName = entry?.iconName ?? "";
        dragGhost.thumbnail = entry?.thumbnail ?? "";
        dragGhost.mime = entry?.mime ?? "";
        dragGhost.path = entry?.path ?? "";
        dragGhost.isDir = entry?.isDir ?? false;

        let grabDone = false;
        let portalDone = false;
        let grabUrl = "";
        let portalKey = "";

        function launch() {
            if (!grabDone || !portalDone || drag !== DragService.generation)
                return;
            dragSource.Drag.mimeData = DragService.mimeData(portalKey);
            dragSource.Drag.imageSource = grabUrl;
            dragSource.Drag.active = true;
            dragSource.Drag.startDrag();
            dragSource.Drag.active = false;
            DragService.end();
        }

        dragGhost.grabToImage(result => {
            grabUrl = result.url;
            grabDone = true;
            launch();
        });
        DragService.exportPortal(paths, key => {
            portalKey = key;
            portalDone = true;
            launch();
        });
    }

    function trashDropped(paths) {
        const outside = paths.filter(path => !TrashService.holds(path));
        trashRequested(outside, outside.length === 1 ? FilePaths.baseName(outside[0]) : FileFormat.count(outside.length));
    }

    DragActionChip {
        readonly property point pointer: FileDragHover.point

        x: Math.max(0, Math.min(controller.anchorItem.width - width, pointer.x + dragGhost.width + Theme.spacingXS))
        y: Math.max(0, Math.min(controller.anchorItem.height - height, pointer.y + (dragGhost.height - height) / 2))
        visible: DragService.action !== "" && ItemTree.isInside(FileDragHover.area, controller.anchorItem)
        action: DragService.action
    }

    Item {
        id: dragSource

        Drag.dragType: Drag.None
        Drag.supportedActions: Qt.CopyAction | Qt.MoveAction | Qt.LinkAction
    }

    DragGhost {
        id: dragGhost

        x: -width * 2
        y: -height * 2
    }

    FileMenu {
        id: dropMenu

        property var pending: []
        property string dest: ""

        anchorItem: controller.anchorItem

        function present(paths, target) {
            pending = paths;
            dest = target;
            openAt(controller.anchorItem.width / 2 - width / 2, controller.anchorItem.height / 2 - height / 2);
        }

        FileMenuItem {
            text: I18n.tr("Copy", "context menu entry copying the selection")
            iconName: "content_copy"
            onClicked: {
                dropMenu.close();
                FileOpsService.copy(dropMenu.pending, dropMenu.dest, controller.opScope);
            }
        }

        FileMenuItem {
            text: I18n.tr("Move", "drag label for moving files")
            iconName: "drive_file_move"
            onClicked: {
                dropMenu.close();
                FileOpsService.move(dropMenu.pending, dropMenu.dest, controller.opScope);
            }
        }

        FileMenuItem {
            text: I18n.tr("Link", "drag label for creating a symlink")
            iconName: "add_link"
            onClicked: {
                dropMenu.close();
                FileOpsService.link(dropMenu.pending, dropMenu.dest, controller.opScope);
            }
        }

        FileMenuItem {
            text: I18n.tr("Cancel", "dialog button to cancel")
            iconName: "close"
            onClicked: dropMenu.close()
        }
    }
}
