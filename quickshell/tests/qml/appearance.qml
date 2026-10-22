pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.Common as DC
import qs.Modules

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

        finish();
    }
}
