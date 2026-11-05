pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services

ListingView {
    id: recentView

    signal removeRequested
    signal clearRequested

    title: I18n.tr("Recent", "recent files view header and sidebar entry")
    listColumns: ["location", "used", "size"]
    columnSortKeys: ({
            "location": "location"
        })
    emptyText: host.nameFilter !== "" ? I18n.tr("No results", "search results header when nothing matched") : I18n.tr("No recent files", "recent files view placeholder when nothing was opened yet")
    dragEnabled: host.dragEnabled
    model: RecentModel {
        nameFilter: recentView.host.nameFilter
        thumbnailsEnabled: SettingsData.thumbnails
        thumbnailSizeLimit: recentView.host.thumbnailSizeLimit
    }

    HeaderAction {
        compact: recentView.compact
        label: I18n.tr("Remove from Recent", "action dropping the selected files from the recently used list without touching the files")
        iconName: "delete_history"
        enabled: recentView.selection.count > 0
        onTriggered: {
            recentView.host.focusRequested();
            recentView.removeRequested();
        }
    }

    HeaderAction {
        compact: recentView.compact
        label: I18n.tr("Clear Recent", "action emptying the recently used list without touching the files")
        iconName: "delete_sweep"
        enabled: recentView.model.total > 0
        onTriggered: {
            recentView.host.focusRequested();
            recentView.clearRequested();
        }
    }
}
