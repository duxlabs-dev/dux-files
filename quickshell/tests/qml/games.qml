pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.Modules

Item {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var sent: []
    property var handler: null
    property var response: ({})

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: true

        function subscribeTopic(topic, cb) {
            if (topic === "devices")
                fixture.handler = cb;
        }

        function unsubscribeTopic(topic, cb) {
        }

        function call(method, params, callback) {
            fixture.sent = fixture.sent.concat([
                {
                    "method": method
                }
            ]);
            if (callback)
                callback(fixture.response);
        }
    }

    GamesSection {
        id: section

        width: 260
        currentPath: "/mnt/jojos/SteamLibrary/steamapps/common"
    }

    Component.onCompleted: {
        response = {
            "groups": [
                {
                    "key": "steam",
                    "places": [
                        {
                            "path": "/home/me/.local/share/Steam/steamapps/common",
                            "kind": "library",
                            "detail": "Steam"
                        },
                        {
                            "path": "/mnt/jojos/SteamLibrary/steamapps/common",
                            "kind": "library",
                            "detail": "SteamLibrary"
                        }
                    ]
                },
                {
                    "key": "lutris",
                    "places": [
                        {
                            "path": "/home/me/Games/pes",
                            "kind": "prefix",
                            "detail": "pes"
                        },
                        {
                            "path": "/gone/SteamLibrary/steamapps/common",
                            "kind": "library",
                            "detail": "gone",
                            "missing": true
                        }
                    ]
                }
            ]
        };
        GamesService.transport = fakeTransport;
        check("the list request runs on attach", sent[0], {
            "method": "games.list"
        });
        check("both launchers with a game are offered", GamesService.groups.map(group => group.key), ["steam", "lutris"]);

        check("section titles", [
            section.groupTitle("steam"),
            section.groupTitle("heroic"),
            section.groupTitle("lutris")
        ], ["Steam", "Heroic", "Lutris"]);
        check("row titles", [
            section.placeTitle({
                "kind": "library",
                "detail": "Steam"
            }),
            section.placeTitle({
                "kind": "proton",
                "detail": "Steam"
            }),
            section.placeTitle({
                "kind": "prefix",
                "detail": "pes"
            }),
            section.placeTitle({
                "kind": "games"
            }),
            section.placeTitle({
                "kind": "config"
            })
        ], ["Library · Steam", "Proton · Steam", "Prefix · pes", "Games folder", "Settings folder"]);

        check("the section shows its groups", [section.visible, section.shown.length], [true, 2]);
        section.toggle("steam");
        check("a group expands", section.expanded("steam"), true);
        section.toggle("steam");
        check("a group collapses", section.expanded("steam"), false);

        SettingsData.showSteamGames = false;
        check("hiding a launcher drops its group", section.shown.map(group => group.key), ["lutris"]);
        SettingsData.showSteamGames = true;

        SettingsData.showGameSettingsFolders = false;
        SettingsData.showGamePrefixes = false;
        check("hidden kinds leave only the games folders", section.shown.map(group => [group.key, group.places.map(place => place.kind)]), [
            ["steam", ["library", "library"]],
            ["lutris", ["library"]]
        ]);
        SettingsData.showGameFolders = false;
        check("a group with every row hidden disappears", section.shown.length, 0);
        SettingsData.showGameSettingsFolders = true;
        SettingsData.showGamePrefixes = true;
        SettingsData.showGameFolders = true;
        check("showing everything again", section.shown.length, 2);

        SettingsData.showGamesSidebar = false;
        check("the sidebar setting hides the section", section.shown.length, 0);
        SettingsData.showGamesSidebar = true;

        fixture.handler({
            "kind": "changed"
        });
        check("a device change asks again", sent.length, 2);

        response = {
            "groups": []
        };
        GamesService.refresh();
        check("nothing installed leaves no group", [GamesService.groups.length, section.visible], [0, false]);

        log.info(failures === 0 ? "FIXTURE PASS" : `FIXTURE FAIL (${failures})`);
        Qt.callLater(Qt.quit);
    }
}
