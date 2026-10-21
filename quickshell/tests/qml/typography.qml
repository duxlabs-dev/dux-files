pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Common
import qs.Services
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

    FileView {
        id: publisher
        path: Theme.dmsTypographyPath
        blockWrites: true
        printErrors: false
    }

    Component {
        id: sourceComponent

        DC.FontSource {
            family: "Own Sans"
            weight: 300
            scale: 0.9
        }
    }

    Component.onCompleted: {
        check("the app defaults to the bundled fonts", [Theme.fontFamily, Theme.monoFontFamily, Theme.displayFontFamily], [DC.Fonts.sans, DC.Fonts.mono, DC.Fonts.display]);

        const custom = sourceComponent.createObject(fixture, {
            "source": "custom"
        });
        check("custom uses the app's own family", custom.fontFamily, "Own Sans");
        check("custom falls back to the bundled mono and display", [custom.monoFontFamily, custom.displayFontFamily], [DC.Fonts.mono, DC.Fonts.display]);
        check("custom uses the app's own weight and scale", [custom.fontWeight, custom.fontScale], [300, 0.9]);

        const system = sourceComponent.createObject(fixture, {
            "source": "system"
        });
        check("system uses the Qt platform font", [system.fontFamily, system.monoFontFamily], [Qt.application.font.family, "monospace"]);
        check("system keeps the app's own weight and scale", [system.fontWeight, system.fontScale], [300, 0.9]);

        const missing = sourceComponent.createObject(fixture, {
            "dmsPath": Theme.dmsTypographyPath + ".missing"
        });
        check("auto without the shell falls back to the app's own fonts", [missing.followingDms, missing.fontFamily, missing.fontWeight], [false, "Own Sans", 300]);

        publisher.setText(JSON.stringify({
            "fontFamily": "Shell Sans",
            "monoFontFamily": "",
            "displayFontFamily": "Shell Serif",
            "fontWeight": 500,
            "fontScale": 1.25
        }));
        const following = sourceComponent.createObject(fixture, {
            "dmsPath": Theme.dmsTypographyPath
        });
        following.dmsChanged.connect(() => {
            check("auto follows the shell", following.followingDms, true);
            check("auto takes the shell families", [following.fontFamily, following.displayFontFamily], ["Shell Sans", "Shell Serif"]);
            check("an empty shell family resolves to the bundled font", following.monoFontFamily, DC.Fonts.mono);
            check("auto takes the shell weight and scale", [following.fontWeight, following.fontScale], [500, 1.25]);
            following.source = "custom";
            check("leaving auto stops following the shell", [following.followingDms, following.fontFamily], [false, "Own Sans"]);
            fixture.finish();
        });
    }
}
