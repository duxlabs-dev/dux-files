pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    readonly property var types: [
        {
            "value": "documents",
            "label": I18n.tr("Documents", "search filter chip for text and office files"),
            "icon": "description"
        },
        {
            "value": "images",
            "label": I18n.tr("Images", "search filter chip for pictures"),
            "icon": "image"
        },
        {
            "value": "video",
            "label": I18n.tr("Video", "search filter chip for video files"),
            "icon": "movie"
        },
        {
            "value": "audio",
            "label": I18n.tr("Audio", "search filter chip for sound files"),
            "icon": "music_note"
        },
        {
            "value": "archives",
            "label": I18n.tr("Archives", "search filter chip for compressed files"),
            "icon": "folder_zip"
        },
        {
            "value": "folders",
            "label": I18n.tr("Folders", "search filter chip for directories"),
            "icon": "folder"
        }
    ]

    readonly property var scopes: [
        {
            "value": "folder",
            "label": I18n.tr("This folder", "search scope chip limiting the search to the current folder"),
            "icon": "folder"
        },
        {
            "value": "home",
            "label": I18n.tr("Home", "sidebar entry for the user's home folder"),
            "icon": "home"
        },
        {
            "value": "everywhere",
            "label": I18n.tr("Everywhere", "search scope chip covering home and every mounted local drive"),
            "icon": "hard_drive"
        }
    ]

    readonly property var modified: [
        {
            "value": "",
            "label": I18n.tr("Any time", "search filter chip clearing the date range")
        },
        {
            "value": "today",
            "label": I18n.tr("Today", "search filter chip for files changed today")
        },
        {
            "value": "week",
            "label": I18n.tr("This week", "search filter chip for files changed in the last seven days")
        },
        {
            "value": "month",
            "label": I18n.tr("This month", "search filter chip for files changed in the last thirty days")
        }
    ]

    readonly property var sizes: [
        {
            "value": "",
            "label": I18n.tr("Any size", "search filter chip clearing the size range")
        },
        {
            "value": "small",
            "label": I18n.tr("Under 1 MB", "search filter chip for small files"),
            "max": 1000000
        },
        {
            "value": "medium",
            "label": I18n.tr("1 MB to 100 MB", "search filter chip for medium files"),
            "min": 1000000,
            "max": 100000000
        },
        {
            "value": "large",
            "label": I18n.tr("Over 100 MB", "search filter chip for large files"),
            "min": 100000000
        }
    ]

    function modifiedAfterMs(value) {
        const now = new Date();
        switch (value) {
        case "today":
            return new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
        case "week":
            return now.getTime() - 7 * 86400000;
        case "month":
            return now.getTime() - 30 * 86400000;
        default:
            return 0;
        }
    }

    function sizeRange(value) {
        const spec = sizes.find(entry => entry.value === value);
        return {
            "minSize": spec?.min ?? 0,
            "maxSize": spec?.max ?? 0
        };
    }

    function options(state) {
        const range = sizeRange(state.size ?? "");
        return {
            "scope": state.scope ?? "folder",
            "includeHidden": state.includeHidden === true,
            "includeIgnored": state.includeIgnored === true,
            "fuzzy": state.fuzzy === true,
            "contents": state.contents === true,
            "regex": state.regex === true,
            "direct": state.direct === true,
            "crossDevice": state.crossDevice === true,
            "types": state.types ?? [],
            "modifiedAfterMs": modifiedAfterMs(state.modified ?? ""),
            "minSize": range.minSize,
            "maxSize": range.maxSize
        };
    }
}
