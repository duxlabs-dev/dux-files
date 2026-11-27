pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets

Flow {
    id: chips

    property string scope: "folder"
    property var types: []
    property string modified: ""
    property string size: ""
    property bool includeHidden: SettingsData.searchHidden
    property bool includeIgnored: false
    property bool fuzzy: false
    property bool contents: false
    property bool contentsAvailable: false
    property bool regex: false
    property bool direct: false

    readonly property var filterState: ({
            "scope": scope,
            "types": types,
            "modified": modified,
            "size": size,
            "includeHidden": includeHidden,
            "includeIgnored": includeIgnored,
            "fuzzy": fuzzy,
            "contents": contents && contentsAvailable,
            "regex": regex,
            "direct": direct
        })

    signal changed

    function resetScope() {
        scope = "folder";
        contents = false;
    }

    function focusFirst() {
        scopeChips.requestFocus(false);
    }

    function indexOf(entries, value) {
        return Math.max(0, entries.findIndex(entry => entry.value === value));
    }

    onScopeChanged: scopeChips.currentIndex = indexOf(SearchFilters.scopes, scope)
    onModifiedChanged: modifiedChips.currentIndex = indexOf(SearchFilters.modified, modified)
    onSizeChanged: sizeChips.currentIndex = indexOf(SearchFilters.sizes, size)

    // A Flow only knows its single-row width once it has one, so the chips are summed by hand.
    function rowWidth(group) {
        let total = 0;
        const items = group.children;
        for (let i = 0; i < items.length; i++) {
            const width = items[i].width;
            if (!(width > 0))
                continue;
            total += width + (total > 0 ? group.spacing : 0);
        }
        return total;
    }

    spacing: Theme.spacingS
    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    DFilterChips {
        id: scopeChips

        width: chips.rowWidth(scopeChips)
        model: SearchFilters.scopes
        currentIndex: chips.indexOf(SearchFilters.scopes, chips.scope)
        showCounts: false
        onSelectionChanged: index => {
            chips.scope = SearchFilters.scopes[index].value;
            chips.changed();
        }
    }

    Rectangle {
        width: Theme.dividerWidth
        height: Theme.buttonHeightXS
        color: Theme.outlineVariant
    }

    DFilterChips {
        id: typeChips

        width: chips.rowWidth(typeChips)
        model: SearchFilters.types
        multiSelect: true
        selectedValues: chips.types
        showCounts: false
        onSelectionToggled: (index, selected) => {
            const value = SearchFilters.types[index].value;
            chips.types = selected ? chips.types.concat([value]) : chips.types.filter(entry => entry !== value);
            chips.changed();
        }
    }

    Rectangle {
        width: Theme.dividerWidth
        height: Theme.buttonHeightXS
        color: Theme.outlineVariant
    }

    DFilterChips {
        id: modifiedChips

        width: chips.rowWidth(modifiedChips)
        model: SearchFilters.modified
        currentIndex: chips.indexOf(SearchFilters.modified, chips.modified)
        showCounts: false
        onSelectionChanged: index => {
            chips.modified = SearchFilters.modified[index].value;
            chips.changed();
        }
    }

    Rectangle {
        width: Theme.dividerWidth
        height: Theme.buttonHeightXS
        color: Theme.outlineVariant
    }

    DFilterChips {
        id: sizeChips

        width: chips.rowWidth(sizeChips)
        model: SearchFilters.sizes
        currentIndex: chips.indexOf(SearchFilters.sizes, chips.size)
        showCounts: false
        onSelectionChanged: index => {
            chips.size = SearchFilters.sizes[index].value;
            chips.changed();
        }
    }

    Rectangle {
        width: Theme.dividerWidth
        height: Theme.buttonHeightXS
        color: Theme.outlineVariant
    }

    DFilterChips {
        id: toggleChips

        readonly property var options: [
            {
                "value": "fuzzy",
                "label": I18n.tr("Fuzzy", "search chip also matching names whose letters appear in order")
            },
            {
                "value": "regex",
                "label": I18n.tr("Regex", "search chip treating the query as a regular expression")
            },
            {
                "value": "hidden",
                "label": I18n.tr("Show hidden files", "menu entry listing dot files")
            },
            {
                "value": "ignored",
                "label": I18n.tr("Include ignored", "search chip walking gitignored entries and heavy folders too")
            },
            {
                "value": "direct",
                "label": I18n.tr("Search the disk", "search chip bypassing the danksearch index and walking the folder")
            }
        ]

        width: chips.rowWidth(toggleChips)
        model: chips.contentsAvailable ? options.concat([
            {
                "value": "contents",
                "label": I18n.tr("Contents", "search chip matching the text inside indexed files")
            }
        ]) : options
        multiSelect: true
        selectedValues: [chips.fuzzy ? "fuzzy" : "", chips.regex ? "regex" : "", chips.includeHidden ? "hidden" : "", chips.includeIgnored ? "ignored" : "", chips.direct ? "direct" : "", chips.contents ? "contents" : ""].filter(value => value !== "")
        showCounts: false
        onSelectionToggled: (index, selected) => {
            switch (toggleChips.model[index].value) {
            case "fuzzy":
                chips.fuzzy = selected;
                break;
            case "regex":
                chips.regex = selected;
                break;
            case "hidden":
                SettingsData.searchHidden = selected;
                break;
            case "ignored":
                chips.includeIgnored = selected;
                break;
            case "contents":
                chips.contents = selected;
                break;
            case "direct":
                chips.direct = selected;
                break;
            }
            chips.changed();
        }
    }
}
