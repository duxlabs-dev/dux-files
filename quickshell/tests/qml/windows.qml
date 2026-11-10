pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var routed: []
    property var added: []

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    Connections {
        target: WindowService
        function onOpenInWindow(windowId, locations) {
            fixture.routed = fixture.routed.concat([[windowId, locations.map(l => l.path)]]);
        }
        function onWindowAdded(windowId) {
            fixture.added = fixture.added.concat([windowId]);
        }
    }

    Component.onCompleted: {
        const docs = {
            "kind": "folder",
            "path": "/home/me/docs",
            "select": []
        };
        const music = {
            "kind": "folder",
            "path": "/home/me/music",
            "select": []
        };

        check("an open with no window creates one", WindowService.open([docs], false), 1);
        check("the new window is the primary and the last used one", [WindowService.count, WindowService.primaryId, WindowService.lastActiveId], [1, 1, 1]);
        check("its locations wait until the window picks them up", WindowService.takePendingLocations(1).map(l => l.path), ["/home/me/docs"]);
        check("pending locations are handed out once", WindowService.takePendingLocations(1), []);
        check("nothing was routed to an existing window", routed, []);

        check("an open without newWindow goes to the last used window", WindowService.open([music], false), 1);
        check("the existing window receives the locations", routed, [[1, ["/home/me/music"]]]);
        check("an empty open routes nothing", [WindowService.open([], false), routed.length], [1, 1]);

        check("newWindow creates a second window", WindowService.open([music], true), 2);
        check("the primary stays the first window", [WindowService.count, WindowService.primaryId, WindowService.lastActiveId], [2, 1, 2]);
        check("the second window gets its own locations", WindowService.takePendingLocations(2).map(l => l.path), ["/home/me/music"]);
        check("every creation is announced", added, [1, 2]);

        WindowService.activate(1);
        check("activating a window makes it the target", WindowService.open([docs], false), 1);
        WindowService.activate(99);
        check("activating an unknown window changes nothing", WindowService.lastActiveId, 1);

        WindowService.close(1);
        check("closing the primary promotes the next window", [WindowService.count, WindowService.primaryId, WindowService.lastActiveId, WindowService.target()], [1, 2, 2, 2]);
        WindowService.close(1);
        check("closing an unknown window changes nothing", WindowService.count, 1);

        WindowService.close(2);
        check("closing the last window leaves nothing to target", [WindowService.count, WindowService.target(), WindowService.primaryId], [0, 0, 0]);
        check("a window opened after that is primary again", [WindowService.open([], true), WindowService.primaryId], [3, 3]);

        WindowService.create([docs]);
        WindowService.closeAll();
        check("closeAll drops every window and its pending locations", [WindowService.count, WindowService.lastActiveId, WindowService.takePendingLocations(4)], [0, 0, []]);

        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }
}
