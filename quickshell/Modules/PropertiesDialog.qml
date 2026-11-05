pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var entries: []
    property int tab: 0
    property string scanId: ""
    property real scannedBytes: -1
    property int scannedFiles: 0
    property bool scanning: false

    readonly property var primary: entries.length > 0 ? entries[0] : null
    readonly property bool multiple: entries.length > 1
    readonly property bool anyDirectory: entries.some(entry => entry.isDir)

    readonly property real plainBytes: {
        let total = 0;
        for (const entry of entries) {
            if (entry.size < 0)
                return -1;
            total += entry.size;
        }
        return total;
    }

    signal renameRequested(string path, string name)

    function show(targets) {
        entries = targets;
        tab = 0;
        scannedBytes = -1;
        scannedFiles = 0;
        permissions.reset(primary);
        open();
        if (anyDirectory)
            startScan();
    }

    function startScan() {
        stopScan();
        scanning = true;
        scannedBytes = 0;
        DirectoryService.scanSize(entries.map(entry => entry.path), result => {
            if (result.error) {
                root.scanning = false;
                return;
            }
            root.scanId = result.scanId || "";
        });
    }

    function stopScan() {
        if (scanId === "")
            return;
        DirectoryService.cancelScan(scanId);
        scanId = "";
        scanning = false;
    }

    function timeRow(label, millis) {
        return {
            "label": label,
            "value": millis > 0 ? FileFormat.modified(millis) : ""
        };
    }

    readonly property var basicRows: {
        if (!primary)
            return [];
        const rows = [
            {
                "label": I18n.tr("Type", "file list column header"),
                "value": multiple ? FileFormat.count(entries.length) : FileFormat.typeLabel(primary)
            },
            {
                "label": I18n.tr("Size", "file list column header"),
                "value": sizeText
            },
            {
                "label": I18n.tr("Location", "properties row showing the folder an item lives in"),
                "value": Paths.shortenHome(FilePaths.parentOf(primary.path) || "/")
            }
        ];
        if (multiple)
            return rows;
        if (primary.isSymlink)
            rows.push({
                "label": I18n.tr("Links to", "properties row showing a symlink target"),
                "value": primary.symlinkTarget
            });
        rows.push(timeRow(I18n.tr("Modified", "file list column header"), primary.mtimeMs));
        rows.push(timeRow(I18n.tr("Created", "file list column header"), primary.ctimeMs));
        rows.push(timeRow(I18n.tr("Accessed", "properties row showing when an item was last read"), primary.atimeMs));
        return rows;
    }

    readonly property string sizeText: {
        if (scannedBytes >= 0) {
            const measured = I18n.tr("%1 · %2", "properties size row: bytes then item count").arg(FileFormat.size(scannedBytes)).arg(FileFormat.count(scannedFiles));
            return scanning ? I18n.tr("%1 …", "properties size row while the folder is still being measured").arg(measured) : measured;
        }
        return plainBytes >= 0 ? FileFormat.size(plainBytes) : "";
    }

    title: multiple ? FileFormat.count(entries.length) : primary?.name ?? ""
    maximumWidth: Theme.dialogMaxWidth
    onRejected: stopScan()

    Connections {
        target: DirectoryService

        function onSizeEvent(event) {
            if (event.scanId !== root.scanId)
                return;
            root.scannedBytes = event.bytes;
            root.scannedFiles = event.files;
            if (!event.done)
                return;
            root.scanning = false;
            root.scanId = "";
        }
    }

    DTabBar {
        width: parent.width
        currentIndex: root.tab
        showIcons: false
        model: [
            {
                "text": I18n.tr("Details", "properties tab with the basic facts about an item")
            },
            {
                "text": I18n.tr("Permissions", "file list column header")
            },
            {
                "text": I18n.tr("Open with", "open with dialog header")
            }
        ]
        onTabClicked: index => root.tab = index
    }

    Item {
        width: parent.width
        height: {
            switch (root.tab) {
            case 1:
                return permissions.implicitHeight;
            case 2:
                return openWith.implicitHeight;
            default:
                return basics.implicitHeight;
            }
        }

        Column {
            id: basics

            width: parent.width
            visible: root.tab === 0
            spacing: Theme.spacingM

            Row {
                width: parent.width
                spacing: Theme.spacingM

                FileItemVisual {
                    anchors.verticalCenter: parent.verticalCenter
                    iconSize: FilesMetrics.propertiesIconSize
                    iconName: root.primary?.iconName ?? ""
                    thumbnail: root.primary?.thumbnail ?? ""
                    path: root.primary?.path ?? ""
                    mime: root.primary?.mime ?? ""
                    isDir: root.primary?.isDir ?? false
                    isSymlink: root.primary?.isSymlink ?? false
                    symlinkBroken: root.primary?.symlinkBroken ?? false
                }

                DTextField {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - FilesMetrics.propertiesIconSize - Theme.spacingM - (stopButton.visible ? stopButton.width + Theme.spacingM : 0)
                    visible: !root.multiple
                    outlined: true
                    labelText: I18n.tr("Name", "label for a file name input")
                    text: root.primary?.name ?? ""
                    onFocusStateChanged: hasFocus => {
                        if (hasFocus && text === root.primary?.name)
                            select(0, FileFormat.stemLength(text, root.primary.isDir));
                    }
                    onAccepted: {
                        const trimmed = text.trim();
                        if (trimmed === "" || trimmed === root.primary.name || trimmed.includes("/"))
                            return;
                        root.renameRequested(root.primary.path, trimmed);
                    }
                }

                DActionButton {
                    id: stopButton

                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.scanning
                    iconName: "stop"
                    tooltipText: I18n.tr("Stop measuring", "properties action halting the recursive size scan")
                    onClicked: root.stopScan()
                }
            }

            Repeater {
                model: root.basicRows

                Row {
                    required property var modelData

                    width: basics.width
                    visible: modelData.value !== ""
                    spacing: Theme.spacingM

                    StyledText {
                        width: FilesMetrics.propertiesLabelWidth
                        text: modelData.label
                        color: Theme.surfaceVariantText
                        font.pixelSize: Theme.fontSizeSmall
                    }

                    StyledText {
                        width: parent.width - FilesMetrics.propertiesLabelWidth - Theme.spacingM
                        text: modelData.value
                        color: Theme.surfaceText
                        font.pixelSize: Theme.fontSizeSmall
                        elide: Text.ElideMiddle
                    }
                }
            }
        }

        PropertiesPermissions {
            id: permissions

            width: parent.width
            visible: root.tab === 1
            paths: root.entries.map(entry => entry.path)
            recursiveAllowed: root.anyDirectory
        }

        PropertiesOpenWith {
            id: openWith

            width: parent.width
            visible: root.tab === 2
            mime: root.primary?.mime ?? ""
            path: root.primary?.path ?? ""
        }
    }

    cancelText: I18n.tr("Close", "dialog button dismissing a dialog")
    cancelAction: () => {
        root.stopScan();
        root.close();
    }
}
