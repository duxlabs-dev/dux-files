pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services

ListingView {
    id: trashView

    signal restoreRequested
    signal emptyRequested

    function refresh() {
        TrashService.refresh();
    }

    title: I18n.tr("Trash", "trash view header and sidebar entry")
    listColumns: ["origin", "deleted", "size"]
    emptyText: host.nameFilter !== "" ? I18n.tr("No results", "search results header when nothing matched") : I18n.tr("Trash is empty", "trash view placeholder and sidebar tooltip when nothing is trashed")
    navigable: false
    dropPath: TrashService.filesPath
    model: TrashModel {
        nameFilter: trashView.host.nameFilter
        thumbnailsEnabled: SettingsData.thumbnails
        thumbnailSizeLimit: trashView.host.thumbnailSizeLimit
    }

    HeaderAction {
        compact: trashView.compact
        label: I18n.tr("Restore", "trash action returning items to where they were deleted from")
        iconName: "restore_from_trash"
        enabled: trashView.selection.count > 0
        onTriggered: {
            trashView.host.focusRequested();
            trashView.restoreRequested();
        }
    }

    HeaderAction {
        compact: trashView.compact
        label: I18n.tr("Empty Trash", "action deleting everything in the trash")
        iconName: "delete_forever"
        containerColor: Theme.errorContainer
        contentColor: Theme.onErrorContainer
        enabled: TrashService.count > 0
        onTriggered: {
            trashView.host.focusRequested();
            trashView.emptyRequested();
        }
    }
}
