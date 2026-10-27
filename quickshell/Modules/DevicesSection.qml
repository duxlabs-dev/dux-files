pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

FilePlaceSection {
    id: section

    property string busyId: ""
    property string currentPath: ""
    readonly property var shownDevices: DevicesService.devices.filter(device => device.mounted || !device.hidden || SettingsData.showSystemPartitions)

    signal navigateRequested(string path)
    signal tabRequested(string path)
    signal unlockRequested(var device)
    signal dropRequested(string path, var drop)

    function activate(device) {
        if (busyId !== "")
            return;
        if (device.encrypted && !device.unlocked) {
            unlockRequested(device);
            return;
        }
        if (device.mounted) {
            navigateRequested(device.mountPoint);
            return;
        }

        busyId = device.id;
        DevicesService.mount(device.id, result => {
            section.busyId = "";
            if (result.error)
                return;
            section.navigateRequested(result.mountPoint);
        });
    }

    function detach(device) {
        if (busyId !== "" || device.system || device.protected)
            return;
        busyId = device.id;
        const done = () => section.busyId = "";
        if (device.encrypted && device.unlocked) {
            DevicesService.lock(device.id, done);
            return;
        }
        if (device.ejectable || device.canPowerOff) {
            DevicesService.eject(device.id, done);
            return;
        }
        DevicesService.unmount(device.id, false, done);
    }

    function actionIconFor(device) {
        switch (true) {
        case device.system || device.protected:
            return "";
        case device.encrypted && !device.unlocked:
            return "lock_open";
        case !device.mounted:
            return "play_arrow";
        case device.encrypted && device.unlocked:
            return "lock";
        case device.ejectable || device.canPowerOff:
            return "eject";
        default:
            return "stop_circle";
        }
    }

    function actionTooltipFor(device) {
        switch (actionIconFor(device)) {
        case "lock_open":
            return I18n.tr("Unlock", "unlock dialog header for an encrypted device");
        case "play_arrow":
            return I18n.tr("Mount", "device action");
        case "lock":
            return I18n.tr("Lock", "device action");
        case "eject":
            return I18n.tr("Eject", "device action");
        case "stop_circle":
            return I18n.tr("Unmount", "device action");
        default:
            return "";
        }
    }

    title: I18n.tr("Devices", "sidebar section listing drives and volumes")
    emptyText: {
        if (DevicesService.available)
            return "";
        if (!SandboxService.loaded)
            return "";
        if (!SandboxService.available("devices"))
            return FilesFormat.featureUnavailable(SandboxService.reason("devices"));
        return I18n.tr("Drives are unavailable because udisks2 is not running", "sidebar explanation when the device service is missing");
    }
    showEmpty: !DevicesService.available
    visible: SettingsData.showDevicesSection && (shownDevices.length > 0 || NetworkService.portables.length > 0 || NetworkService.deviceMounts.length > 0 || (showEmpty && emptyText !== ""))

    function activateOrMount(device) {
        if (busyId !== "")
            return;
        if (device.mounted || device.encrypted || SettingsData.mountOnClick) {
            activate(device);
            return;
        }
        busyId = device.id;
        DevicesService.mount(device.id, () => section.busyId = "");
    }

    Repeater {
        model: section.shownDevices

        FilePlaceRow {
            required property var modelData

            label: modelData.mounted && modelData.mountPoint ? modelData.mountPoint : modelData.name || modelData.label || I18n.tr("File system", "storage volume containing the root directory")
            subtitle: modelData.device || ""
            iconName: modelData.encrypted && !modelData.unlocked ? "lock" : modelData.iconName
            ringValue: modelData.mounted ? DevicesService.usageRatio(modelData.mountPoint) : -1
            selected: modelData.mounted && (modelData.mountPoints?.length ? modelData.mountPoints : [modelData.mountPoint]).includes(section.currentPath)
            tooltipText: {
                const usage = DevicesService.usage[modelData.mountPoint];
                const location = [modelData.mountPoint, modelData.device].filter(value => value).join("\n");
                if (!usage?.total || usage.code)
                    return location;
                return location + "\n" + I18n.tr("%1 used · %2 free · %3 total", "storage volume usage, free space and total capacity").arg(FileFormat.size(usage.used)).arg(FileFormat.size(usage.free)).arg(FileFormat.size(usage.total));
            }
            actionIcon: section.actionIconFor(modelData)
            actionTooltip: section.actionTooltipFor(modelData)
            busy: section.busyId === modelData.id
            dropPath: modelData.mounted ? modelData.mountPoint : ""
            onClicked: section.activateOrMount(modelData)
            onMiddleClicked: {
                if (modelData.mounted && modelData.mountPoint)
                    section.tabRequested(modelData.mountPoint);
            }
            onDropped: drop => section.dropRequested(modelData.mountPoint, drop)
            onSpringLoaded: section.navigateRequested(modelData.mountPoint)
            onActionTriggered: {
                if (modelData.system || modelData.protected)
                    return;
                if (!modelData.mounted) {
                    section.activate(modelData);
                    return;
                }
                section.detach(modelData);
            }
        }
    }

    Repeater {
        model: NetworkService.portables

        FilePlaceRow {
            required property var modelData

            label: modelData.name
            iconName: modelData.iconName
            actionIcon: "play_arrow"
            actionTooltip: I18n.tr("Mount", "device action")
            busy: section.busyId === modelData.id
            onClicked: {
                section.busyId = modelData.id;
                NetworkService.mountPortable(modelData.monitor, modelData.id, () => section.busyId = "");
            }
            onActionTriggered: clicked()
        }
    }

    Repeater {
        model: NetworkService.deviceMounts

        FilePlaceRow {
            required property var modelData

            label: modelData.name
            iconName: modelData.iconName
            actionIcon: "eject"
            actionTooltip: I18n.tr("Disconnect", "network mount action")
            busy: section.busyId === modelData.id
            dropPath: modelData.path
            onClicked: section.navigateRequested(modelData.path)
            onMiddleClicked: section.tabRequested(modelData.path)
            onDropped: drop => section.dropRequested(modelData.path, drop)
            onSpringLoaded: section.navigateRequested(modelData.path)
            onActionTriggered: {
                section.busyId = modelData.id;
                NetworkService.disconnect(modelData.id, () => section.busyId = "");
            }
        }
    }
}
