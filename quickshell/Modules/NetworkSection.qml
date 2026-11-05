pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FilePlaceSection {
    id: section

    property string busyId: ""

    signal navigateRequested(string path)
    signal tabRequested(string path)
    signal dropRequested(string path, var drop)
    signal connectRequested
    signal refreshRequested
    signal menuRequested(string id, string path, real pointX, real pointY)

    visible: SettingsData.showNetworkSection
    title: I18n.tr("Network", "sidebar section listing remote locations")
    emptyText: {
        if (NetworkService.available)
            return "";
        if (!SandboxService.loaded)
            return "";
        if (!SandboxService.available("network"))
            return FilesFormat.featureUnavailable(SandboxService.reason("network"));
        return I18n.tr("Network locations are unavailable because gvfs is not running", "sidebar explanation when the network service is missing");
    }
    showEmpty: true

    Timer {
        id: mountRefresh
        interval: 100
        onTriggered: section.refreshRequested()
    }

    Repeater {
        model: NetworkService.visibleNetworkMounts

        Item {
            required property var modelData
            width: section.width
            height: row.implicitHeight

            FilePlaceRow {
                id: row

                TapHandler {
                    acceptedButtons: Qt.RightButton
                    onTapped: point => {
                        if (!modelData.mounted)
                            return;
                        const mapped = row.mapToItem(section, point.position.x, point.position.y);
                        section.menuRequested(modelData.id, modelData.path, mapped.x, mapped.y);
                    }
                }

                anchors.fill: parent
                label: modelData.name
                iconName: modelData.iconName
                actionIcon: modelData.mounted ? "eject" : "play_arrow"
                actionTooltip: modelData.mounted ? I18n.tr("Disconnect", "network mount action") : I18n.tr("Mount", "network mount action")
                busy: section.busyId === modelData.id
                dropPath: modelData.mounted ? modelData.path : ""
                onClicked: {
                    if (modelData.mounted) {
                        section.navigateRequested(modelData.path);
                        return;
                    }
                    const id = modelData.id;
                    const path = modelData.path;
                    section.busyId = id;
                    NetworkService.mount(id, result => {
                        section.busyId = "";
                        if (result.error)
                            return;
                        section.navigateRequested(path);
                        mountRefresh.restart();
                    });
                }
                onMiddleClicked: section.tabRequested(modelData.path)
                onDropped: drop => section.dropRequested(modelData.path, drop)
                onSpringLoaded: {
                    if (modelData.mounted)
                        section.navigateRequested(modelData.path);
                }
                onActionTriggered: {
                    const id = modelData.id;
                    const mounted = modelData.mounted === true;
                    section.busyId = id;
                    const done = result => {
                        section.busyId = "";
                        if (!result?.error)
                            mountRefresh.restart();
                    };
                    if (mounted)
                        NetworkService.disconnect(id, done);
                    else
                        NetworkService.mount(id, done);
                }
            }

            Rectangle {
                visible: modelData.mounted === true
                width: 8
                height: width
                radius: width / 2
                anchors.left: parent.left
                anchors.leftMargin: 36
                anchors.bottom: parent.bottom
                anchors.bottomMargin: Style.spacingS + 1
                z: 2
                color: "#4CAF50"
                border.color: Style.surfaceContainerLowest
                border.width: 1
                opacity: 0.9
            }
        }
    }

    FilePlaceRow {
        visible: NetworkService.available
        label: I18n.tr("Connect to Server…", "sidebar entry opening the connect to server dialog")
        iconName: "folder-remote"
        busy: NetworkService.connecting
        onClicked: section.connectRequested()
    }
}
