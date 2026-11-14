import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FilePlaceRow {
    id: row

    required property var modelData
    required property var host
    property bool network: false

    label: modelData.name
    iconName: modelData.iconName
    actionIcon: network ? (modelData.mounted ? "eject" : "play_arrow") : "eject"
    actionTooltip: network ? (modelData.mounted ? I18n.tr("Disconnect", "network mount action") : I18n.tr("Mount", "network mount action")) : I18n.tr("Disconnect", "network mount action")
    busy: host.busyId === modelData.id
    dropPath: network ? (modelData.mounted ? modelData.path : "") : modelData.path

    TapHandler {
        enabled: row.network
        acceptedButtons: Qt.RightButton
        onTapped: point => row.host.showMountMenu(row, modelData.id, modelData.path, modelData.mounted === true, point)
    }

    onClicked: {
        if (network) {
            host.activateMount(modelData.id, modelData.path, modelData.mounted === true);
            return;
        }
        host.navigateRequested(modelData.path);
    }
    onMiddleClicked: host.tabRequested(modelData.path)
    onDropped: drop => host.dropRequested(modelData.path, drop)
    onSpringLoaded: {
        if (!network || modelData.mounted)
            host.navigateRequested(modelData.path);
    }
    onActionTriggered: {
        if (network) {
            host.toggleMount(modelData.id, modelData.mounted === true);
            return;
        }
        host.busyId = modelData.id;
        NetworkService.disconnect(modelData.id, () => host.busyId = "");
    }

    Rectangle {
        visible: row.network && modelData.mounted === true
        width: Theme.spacingS
        height: width
        radius: width / 2
        anchors.left: parent.left
        anchors.leftMargin: Theme.spacingS + Theme.iconSize + Theme.spacingXXS
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Theme.spacingS + Theme.spacingXXS
        z: 2
        color: Theme.success
        border.color: Theme.surfaceContainerLowest
        border.width: Theme.outlineWidth
    }
}
