pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

Column {
    id: permissions

    property var paths: []
    property bool recursiveAllowed: false
    property var triplets: FilesFormat.permissionBits("----------")
    property string owner: ""
    property string group: ""
    property bool recursive: false

    readonly property string octal: FilesFormat.octalFor(triplets)
    readonly property bool canChangeOwner: FileOpsService.canElevate

    function reset(entry) {
        triplets = FilesFormat.permissionBits(entry?.mode ?? "----------");
        owner = entry?.owner ?? "";
        group = entry?.group ?? "";
        ownerField.text = owner;
        groupField.text = group;
        recursive = false;
    }

    function setBit(index, key, value) {
        const next = triplets.map(triplet => Object.assign({}, triplet));
        next[index][key] = value;
        triplets = next;
    }

    function applyMode() {
        FileOpsService.setPermissions(paths, octal, recursive);
    }

    function applyOwner() {
        FileOpsService.setOwner(paths, owner, group, recursive);
    }

    spacing: Theme.spacingM

    readonly property var bits: [
        {
            "key": "read",
            "label": I18n.tr("Read", "permission bit")
        },
        {
            "key": "write",
            "label": I18n.tr("Write", "permission bit")
        },
        {
            "key": "execute",
            "label": I18n.tr("Execute", "permission bit")
        }
    ]

    readonly property var rows: [
        {
            "slot": 0,
            "label": I18n.tr("Owner", "file list column header")
        },
        {
            "slot": 1,
            "label": I18n.tr("Group", "permissions row for the owning group")
        },
        {
            "slot": 2,
            "label": I18n.tr("Everyone", "permissions row for other users")
        }
    ]

    readonly property real labelWidth: Theme.spacingXL * 3
    readonly property real cellWidth: Theme.switchTrackWidth + Theme.spacingM

    Row {
        width: parent.width
        spacing: 0

        Item {
            width: permissions.labelWidth
            height: 1
        }

        Repeater {
            model: permissions.bits

            StyledText {
                required property var modelData

                width: permissions.cellWidth
                text: modelData.label
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                elide: Text.ElideRight
            }
        }
    }

    Repeater {
        model: permissions.rows

        Row {
            id: line

            required property var modelData

            width: permissions.width
            spacing: 0

            StyledText {
                anchors.verticalCenter: parent.verticalCenter
                width: permissions.labelWidth
                text: line.modelData.label
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
            }

            Repeater {
                model: permissions.bits

                Item {
                    required property var modelData

                    width: permissions.cellWidth
                    height: Theme.switchTrackHeight

                    DToggle {
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        hideText: true
                        checked: permissions.triplets[line.modelData.slot][modelData.key]
                        onToggled: checked => permissions.setBit(line.modelData.slot, modelData.key, checked)
                    }
                }
            }
        }
    }

    Row {
        width: parent.width
        spacing: Theme.spacingM

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            width: permissions.labelWidth
            text: I18n.tr("Octal", "permissions row showing the numeric mode")
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
        }

        StyledText {
            anchors.verticalCenter: parent.verticalCenter
            text: permissions.octal
            color: Theme.surfaceText
            font.family: Theme.monoFontFamily
            font.pixelSize: Theme.fontSizeMedium
        }

        DButton {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Apply", "button applying the edited permissions")
            onClicked: permissions.applyMode()
        }
    }

    DToggle {
        width: parent.width
        visible: permissions.recursiveAllowed
        text: I18n.tr("Apply to everything inside", "permissions option recursing into a folder")
        checked: permissions.recursive
        onToggled: checked => permissions.recursive = checked
    }

    Row {
        width: parent.width
        visible: permissions.canChangeOwner
        spacing: Theme.spacingM

        DTextField {
            id: ownerField

            width: (parent.width - Theme.spacingM * 2 - applyOwnerButton.width) / 2
            outlined: true
            labelText: I18n.tr("Owner", "file list column header")
            onTextEdited: permissions.owner = text
        }

        DTextField {
            id: groupField

            width: ownerField.width
            outlined: true
            labelText: I18n.tr("Group", "permissions row for the owning group")
            onTextEdited: permissions.group = text
        }

        DButton {
            id: applyOwnerButton

            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Apply", "button applying the edited permissions")
            variant: "tonal"
            onClicked: permissions.applyOwner()
        }
    }

    StyledText {
        width: parent.width
        visible: !permissions.canChangeOwner
        text: FilesFormat.elevationUnavailable(FileOpsService.elevation.reason)
        color: Theme.surfaceVariantText
        font.pixelSize: Theme.fontSizeSmall
        wrapMode: Text.Wrap
    }
}
