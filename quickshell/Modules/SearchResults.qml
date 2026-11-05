pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

ListingView {
    id: results

    property bool indexAvailable: false
    readonly property alias chips: chips
    readonly property string scopeText: {
        switch (chips.scope) {
        case "home":
            return Paths.shortenHome(Paths.strip(Paths.home));
        case "everywhere":
            return I18n.tr("Everywhere", "search scope chip covering home and every mounted local drive");
        default:
            return Paths.shortenHome(results.model.rootPath);
        }
    }

    function sortBy(key) {
        model.setSort(key);
    }

    model: host.searchModel
    title: model.query !== "" ? I18n.tr("Results for “%1” in %2", "search results header, query then folder").arg(model.query).arg(scopeText) : I18n.tr("Results in %1", "search results header without a query").arg(scopeText)
    summary: {
        if (model.error !== "")
            return FileFormat.listingError(model.errorCode);
        const notes = [];
        switch (true) {
        case model.loading:
            notes.push(I18n.tr("Searching…", "search results header while the daemon walks the tree"));
            break;
        case model.truncated:
            notes.push(I18n.tr("Showing the first %1 results", "search results header when the cap was hit").arg(model.count));
            break;
        case model.count === 0:
            notes.push(I18n.tr("No results", "search results header when nothing matched"));
            break;
        default:
            notes.push(FileFormat.count(model.count));
            break;
        }
        if (model.unreadable > 0)
            notes.push(I18n.tr("%1 folders could not be read", "search results header note").arg(model.unreadable));
        if (model.partial)
            notes.push(I18n.tr("Some matches may be missing", "search results header note when the index could not return every candidate"));
        if (model.servedBy === "index") {
            notes.push(model.indexPhase === "idle" || model.indexPhase === "" ? I18n.tr("From the index", "search results header note when danksearch served the search") : I18n.tr("Index is still building, results may be incomplete", "search results header note while danksearch is indexing"));
            if (model.depthLimit > 0)
                notes.push(I18n.tr("%1 levels deep", "search results header note giving how far below the folder the index reaches").arg(model.depthLimit));
        }
        return notes.join(" · ");
    }
    listColumns: ["location", "size", "modified"]
    columnSortKeys: ({
            "location": "location"
        })
    showHidden: chips.includeHidden
    nameHighlight: model.query
    filtering: false
    dragEnabled: host.dragEnabled
    backgroundMenu: false

    DActionButton {
        buttonSize: Theme.buttonHeightXS
        iconName: "close"
        iconSize: Theme.iconSizeSmall
        iconColor: Theme.surfaceVariantText
        tooltipText: I18n.tr("Close", "dialog button to close")
        onClicked: {
            results.host.closeSearch();
            results.host.focusBody();
        }
    }

    extras: SearchChips {
        id: chips

        width: parent.width
        contentsAvailable: results.indexAvailable
        onChanged: results.host.startSearch(results.model.query, chips.filterState)
    }

    Connections {
        target: results.model

        function onServedByChanged() {
            if (results.model.servedBy === "")
                return;
            if (chips.direct)
                return;
            results.indexAvailable = results.model.servedBy === "index";
            if (!results.indexAvailable)
                chips.contents = false;
        }
    }
}
