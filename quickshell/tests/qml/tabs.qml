pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
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

    component FakePane: QtObject {
        property string path: ""
        function saveState() {
            return {
                "path": path
            };
        }
        function restoreState(state) {
            path = state?.path ?? "";
        }
    }

    readonly property FakePane primaryPane: FakePane {}
    property var extraPane: null
    property int activePaneIndex: 0

    readonly property TabManager manager: TabManager {
        panes: [fixture.primaryPane, fixture.extraPane]
        activePane: fixture.activePaneIndex === 1 && fixture.extraPane ? fixture.extraPane : fixture.primaryPane
        activePaneIndex: fixture.activePaneIndex
        primary: false
        onActivePaneRequested: index => fixture.activePaneIndex = index
    }

    function setupTabs(paths, current) {
        extraPane = null;
        activePaneIndex = 0;
        manager.tabs = paths.map(p => ({
                    "panes": [
                        {
                            "path": p
                        },
                        null],
                    "active": 0
                }));
        manager.currentTab = current;
        manager.applyTab(current);
    }

    Component.onCompleted: {
        const home = Paths.strip(Paths.home);

        check("exact mount path matches", manager.pathInMount("/mnt/usb", "/mnt/usb"), true);
        check("trailing slash matches", manager.pathInMount("/mnt/usb/", "/mnt/usb"), true);
        check("nested subdirectory matches", manager.pathInMount("/mnt/usb/photos/2026", "/mnt/usb"), true);
        check("sibling prefix does not match", manager.pathInMount("/mnt/usb2", "/mnt/usb"), false);
        check("sibling prefix subdir does not match", manager.pathInMount("/mnt/usb2/files", "/mnt/usb"), false);
        check("unrelated path does not match", manager.pathInMount("/home/user", "/mnt/usb"), false);
        check("empty path does not match", manager.pathInMount("", "/mnt/usb"), false);
        check("virtual path does not match", manager.pathInMount("recent://", "/mnt/usb"), false);

        setupTabs(["/home/user", "/mnt/usb", "/mnt/usb/photos/2026", "/mnt/usb2"], 1);
        manager.closeMounts(["/mnt/usb"]);
        check("tabs inside mount closed, sibling prefix kept", manager.tabs.map(t => t.panes[0].path), ["/home/user", "/mnt/usb2"]);
        check("active pane moved to remaining tab", primaryPane.path, "/mnt/usb2");

        setupTabs(["/home/user", "/mnt/usb/docs"], 0);
        manager.closeMounts(["/mnt/usb"]);
        check("inactive tab inside mount closed", manager.tabs.map(t => t.panes[0].path), ["/home/user"]);
        check("current tab outside mount stays", [manager.currentTab, primaryPane.path], [0, "/home/user"]);

        setupTabs(["/mnt/usb/sub"], 0);
        manager.closeMounts(["/mnt/usb"]);
        check("last tab inside mount resets to home", [manager.tabs.length, primaryPane.path, manager.tabs[0].panes[0].path], [1, home, home]);

        setupTabs(["/mnt/usb", "/mnt/usb/sub"], 1);
        manager.closeMounts(["/mnt/usb"]);
        check("all tabs inside mount collapses to one tab at home", [manager.tabs.length, manager.currentTab, primaryPane.path], [1, 0, home]);

        const splitSecond = Qt.createQmlObject('import QtQuick; QtObject { property string path: "/mnt/usb/files"; function saveState() { return {"path": path}; } function restoreState(s) { path = s?.path ?? ""; } }', fixture);
        extraPane = splitSecond;
        activePaneIndex = 0;
        manager.tabs = [
            {
                "panes": [
                    {
                        "path": "/home/user"
                    },
                    {
                        "path": "/mnt/usb/files"
                    }
                ],
                "active": 0
            }
        ];
        manager.currentTab = 0;
        manager.applyTab(0);
        manager.closeMounts(["/mnt/usb"]);
        check("split pane outside mount stays while pane inside mount resets", [manager.tabs.length, primaryPane.path, splitSecond.path], [1, "/home/user", home]);

        manager.tabs = [
            {
                "split": false,
                "panes": [
                    {
                        "path": "/tmp"
                    },
                    {
                        "path": ""
                    }
                ],
                "active": 0
            },
            {
                "split": true,
                "panes": [
                    {
                        "path": "/home/user"
                    },
                    {
                        "path": "/mnt/usb/files"
                    }
                ],
                "active": 1
            }
        ];
        manager.currentTab = 1;
        manager.applyTab(1);
        manager.closeMounts(["/mnt/usb"]);
        check("split pane tab stays open even when active pane is inside mount", [manager.tabs.length, primaryPane.path, splitSecond.path], [2, "/home/user", home]);

        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }
}
