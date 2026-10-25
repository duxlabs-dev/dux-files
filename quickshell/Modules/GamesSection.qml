pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FilePlaceSection {
    id: section

    property string currentPath: ""
    property var expandedGroups: []

    signal navigateRequested(string path)
    signal tabRequested(string path)
    signal dropRequested(string path, var drop)

    readonly property var shown: visibleGroups()

    readonly property var launchers: ({
            "steam": SettingsData.showSteamGames,
            "heroic": SettingsData.showHeroicGames,
            "lutris": SettingsData.showLutrisGames
        })

    function showsLauncher(key) {
        return launchers[key] !== false;
    }

    function showsPlace(place) {
        switch (place.kind) {
        case "config":
            return SettingsData.showGameSettingsFolders;
        case "prefix":
        case "proton":
            return SettingsData.showGamePrefixes;
        default:
            return SettingsData.showGameFolders;
        }
    }

    function visibleGroups() {
        if (!SettingsData.showGamesSidebar)
            return [];
        const groups = [];
        for (const group of GamesService.groups) {
            if (!showsLauncher(group.key))
                continue;
            const places = group.places.filter(showsPlace);
            if (places.length === 0)
                continue;
            groups.push({
                "key": group.key,
                "places": places
            });
        }
        return groups;
    }

    function groupTitle(key) {
        switch (key) {
        case "steam":
            return I18n.tr("Steam", "games sidebar group holding steam libraries");
        case "heroic":
            return I18n.tr("Heroic", "games sidebar group holding heroic games");
        case "lutris":
            return I18n.tr("Lutris", "games sidebar group holding lutris games");
        default:
            return key;
        }
    }

    function placeTitle(place) {
        switch (place.kind) {
        case "library":
            return I18n.tr("Library · %1", "games sidebar row for a steam library").arg(place.detail);
        case "proton":
            return I18n.tr("Proton · %1", "games sidebar row for the proton prefixes of a steam library").arg(place.detail);
        case "prefix":
            return I18n.tr("Prefix · %1", "games sidebar row for one game's wine prefix").arg(place.detail);
        case "games":
            return I18n.tr("Games folder", "games sidebar row for the launcher's games folder");
        default:
            return I18n.tr("Settings folder", "games sidebar row for the launcher's settings folder");
        }
    }

    function expanded(key) {
        return expandedGroups.includes(key);
    }

    function toggle(key) {
        const next = expandedGroups.slice();
        const index = next.indexOf(key);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(key);
        expandedGroups = next;
    }

    visible: shown.length > 0
    title: I18n.tr("Games", "sidebar section listing game libraries")

    Repeater {
        model: section.shown

        Column {
            id: group

            required property var modelData

            width: section.width
            spacing: FileBrowserMetrics.sidebarRowGap

            FilePlaceRow {
                label: section.groupTitle(group.modelData.key)
                glyph: "sports_esports"
                expandable: true
                expanded: section.expanded(group.modelData.key)
                onClicked: section.toggle(group.modelData.key)
            }

            Item {
                id: placesBox

                width: group.width
                height: section.expanded(group.modelData.key) ? placesList.height : 0
                clip: true

                Behavior on height {
                    enabled: Theme.animationsEnabled
                    NumberAnimation {
                        duration: Theme.expressiveDurations.expressiveFastSpatial
                        easing.bezierCurve: Theme.expressiveCurves.expressiveDefaultSpatial
                    }
                }

                Column {
                    id: placesList

                    width: placesBox.width
                    spacing: FileBrowserMetrics.sidebarRowGap

                    Repeater {
                        model: group.modelData.places

                        FilePlaceRow {
                            required property var modelData

                            visible: section.expanded(group.modelData.key)
                            label: section.placeTitle(modelData)
                            iconName: "folder"
                            enabled: !modelData.missing
                            tooltipText: modelData.missing ? I18n.tr("Unavailable until its drive is mounted", "games sidebar row for a library on a drive that is not mounted") : modelData.path
                            selected: !modelData.missing && section.currentPath === modelData.path
                            dropPath: modelData.missing ? "" : modelData.path
                            onClicked: section.navigateRequested(modelData.path)
                            onMiddleClicked: section.tabRequested(modelData.path)
                            onDropped: drop => section.dropRequested(modelData.path, drop)
                            onSpringLoaded: section.navigateRequested(modelData.path)
                        }
                    }
                }
            }
        }
    }
}
