pragma ComponentBehavior: Bound

import QtQuick
import qs.Services
import qs.DCommon.FileBrowser

QtObject {
    id: root

    property bool active: true

    function coded(message, code) {
        ToastService.info(FileFormat.operationError(code, message));
    }

    readonly property Connections _devices: Connections {
        target: DevicesService
        enabled: root.active

        function onRequestFailed(message, code) {
            root.coded(message, code);
        }
    }

    readonly property Connections _network: Connections {
        target: NetworkService
        enabled: root.active

        function onRequestFailed(message, code) {
            root.coded(message, code);
        }
    }

    readonly property Connections _recent: Connections {
        target: RecentService
        enabled: root.active

        function onRequestFailed(message, code) {
            root.coded(message, code);
        }
    }

    readonly property Connections _games: Connections {
        target: GamesService
        enabled: root.active

        function onRequestFailed(message, code) {
            root.coded(message, code);
        }
    }

    readonly property Connections _bookmarks: Connections {
        target: BookmarksService
        enabled: root.active

        function onRequestFailed(message) {
            ToastService.info(message);
        }
    }

    readonly property Connections _fileOps: Connections {
        target: FileOpsService
        enabled: root.active

        function onRequestFailed(message) {
            ToastService.info(message);
        }
    }

    readonly property Connections _clipboard: Connections {
        target: ClipboardService
        enabled: root.active

        function onRequestFailed(message) {
            ToastService.info(message);
        }
    }
}
