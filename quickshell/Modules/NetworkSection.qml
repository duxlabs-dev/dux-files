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

    function finishMount(id, path, navigate, result) {
        if (busyId === id)
            busyId = "";
        if (result?.error)
            return;
        if (navigate)
            navigateRequested(path);
        mountRefresh.restart();
    }

    function activateMount(id, path, mounted) {
        if (mounted) {
            navigateRequested(path);
            return;
        }
        busyId = id;
        NetworkService.mount(id, result => section.finishMount(id, path, true, result));
    }

    function toggleMount(id, mounted) {
        busyId = id;
        const done = result => section.finishMount(id, "", false, result);
        if (mounted)
            NetworkService.disconnect(id, done);
        else
            NetworkService.mount(id, done);
    }

    function showMountMenu(row, id, path, mounted, point) {
        if (!mounted)
            return;
        const mapped = row.mapToItem(section, point.position.x, point.position.y);
        menuRequested(id, path, mapped.x, mapped.y);
    }

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

        NetworkMountRow {
            host: section
            network: true
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
