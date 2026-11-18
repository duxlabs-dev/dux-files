import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FilePlaceRow {
    id: row

    required property var modelData
    required property var host

    label: modelData.name
    iconName: modelData.iconName
    actionIcon: "eject"
    actionTooltip: I18n.tr("Disconnect", "network mount action")
    busy: host.busyId === modelData.id
    dropPath: modelData.path
    onClicked: host.navigateRequested(modelData.path)
    onMiddleClicked: host.tabRequested(modelData.path)
    onDropped: drop => host.dropRequested(modelData.path, drop)
    onSpringLoaded: host.navigateRequested(modelData.path)
    onActionTriggered: {
        host.busyId = modelData.id;
        NetworkService.disconnect(modelData.id, () => host.busyId = "");
    }
}
