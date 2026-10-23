pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.Common as DC
import qs.Modules
import qs.DCommon.FileBrowser
import qs.DCommon.Widgets

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function finish() {
        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }

    // A row binds the look, the way a sidebar place or a file tile does.
    QtObject {
        id: row

        readonly property color tint: FolderAppearances.tintFor("/tmp/looks", "transparent")
        readonly property string icon: FolderAppearances.iconFor("/tmp/looks", "folder")
    }

    FolderAppearanceDialog {
        id: dialog
    }

    DGridView {
        id: iconGrid

        width: 256
        height: 128
        cellWidth: 64
        cellHeight: 64
        model: 40
        delegate: Item {
            width: 56
            height: 56
        }
    }

    FileIcon {
        id: customIcon

        iconName: "folder"
        iconOverride: "folder-icon-that-no-theme-has"
    }

    FileIcon {
        id: plainIcon

        iconName: "folder"
    }

    Component.onCompleted: {
        DC.Style.theme = Theme;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;
        DC.Host.appearance = FolderAppearances;

        check("a folder starts with no look", [row.tint.toString(), row.icon], ["#00000000", "folder"]);

        FolderAppearances.set("/tmp/looks", {
            "color": "#44AAFF",
            "icon": "folder-red"
        });
        check("a look reaches a bound row without rebuilding it", [row.tint.toString(), row.icon], ["#44aaff", "folder-red"]);

        FolderAppearances.set("/tmp/looks", {
            "color": "not a color",
            "icon": "folder-red"
        });
        check("a bad colour is dropped and the icon stays", [row.tint.toString(), row.icon], ["#00000000", "folder-red"]);

        FolderAppearances.reset("/tmp/looks");
        check("resetting drops the look", [row.tint.toString(), row.icon], ["#00000000", "folder"]);

        dialog.show({
            "path": "/tmp/looks",
            "name": "looks"
        });
        dialog.apply({
            "color": "#123456"
        });
        check("the dialog applies a colour outside the palette", [FolderAppearances.lookup("/tmp/looks").color, dialog.customColor], ["#123456", true]);
        dialog.apply({
            "emblem": "https://example.com/logo.png"
        });
        check("a badge keeps the colour", FolderAppearances.lookup("/tmp/looks"), {
            "color": "#123456",
            "emblem": "https://example.com/logo.png"
        });
        FolderAppearances.reset("/tmp/looks");

        check("an icon the theme lacks falls back to the base name", [customIcon.themeIconPath !== "", customIcon.themeIconPath], [true, plainIcon.themeIconPath]);

        iconGrid.contentY = iconGrid.maximumContentY;
        check("the icon grid can scroll past its first rows", [iconGrid.contentHeight > iconGrid.height, iconGrid.contentY > 0], [true, true]);

        finish();
    }
}
