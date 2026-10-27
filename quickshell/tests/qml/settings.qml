pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Common
import qs.Services
import qs.Modules.Settings
import qs.DCommon.Common as DC

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

    SettingsContent {
        id: content
    }

    QtObject {
        id: boundIcon
        property string source: Paths.resolveIconPath("folder")
    }

    Component.onCompleted: {
        DC.Style.theme = Theme;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;
        DC.Host.files = null;
        DC.Host.session = null;
        DC.Host.cache = null;
        DC.Log.backend = Log;

        content.currentIndex = 1;
        check("the appearance page builds", content.children.length > 0, true);
        check("the icon theme starts on automatic", content.iconThemeSetting, "auto");
        check("the theme list is empty until the daemon answers", content.iconThemes.length, 0);
        Paths.applyIconTheme({setting: "A", effective: "A"});
        Paths.themePaths = {folder: "/tmp/theme-a/folder.svg"};
        Qt.callLater(() => {
            check("bound icon uses theme A", boundIcon.source, "file:///tmp/theme-a/folder.svg");
            Paths.applyIconTheme({setting: "B", effective: "B"});
            Paths.themePaths = {folder: "/tmp/theme-b/folder.svg"};
            Qt.callLater(() => {
                check("bound icon changes without restart", boundIcon.source, "file:///tmp/theme-b/folder.svg");
                check("settings follows the active theme", content.iconThemeSetting, "B");
                finish();
            });
        });
    }
}
