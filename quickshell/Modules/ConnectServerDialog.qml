pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property string errorText: ""
    property string server: ""
    property var shares: []

    signal connected(string path)

    function ask() {
        reset();
        addressField.text = schemeDropdown.currentValue === "" ? "" : schemeDropdown.currentValue + "://";
        open();
        addressField.forceActiveFocus();
    }

    function reset() {
        errorText = "";
        server = "";
        shares = [];
    }

    function submit(uri) {
        const target = (uri || addressField.text).trim();
        if (target === "")
            return;
        errorText = "";
        NetworkService.connectTo(target, result => root.present(target, result));
    }

    function present(target, result) {
        if (result.shares) {
            showShares(result);
            return;
        }
        const path = localPath(result);
        if (path === "") {
            showFailure(target, result);
            return;
        }
        connected(path);
        close();
    }

    function localPath(result) {
        if (result.error)
            return "";
        return result.location || result.mount?.path || "";
    }

    function showShares(result) {
        errorText = "";
        server = result.server;
        shares = result.shares;
        addressField.text = result.server;
        if (!visible)
            open();
    }

    function showFailure(target, result) {
        errorText = result.error || I18n.tr("Connected, but the share has no local folder. The gvfs FUSE service may not be running.", "connect dialog error when a mounted share has no fuse path");
        if (visible)
            return;
        addressField.text = target;
        open();
    }

    title: I18n.tr("Connect to Server", "connect to server dialog header")
    onAccepted: submit("")

    Column {
        width: parent.width
        spacing: Theme.spacingM

        DDropdown {
            id: schemeDropdown

            width: parent.width
            visible: NetworkService.schemes.length > 0
            text: I18n.tr("Protocol", "label for the network protocol picker")
            options: NetworkService.schemes
            currentValue: NetworkService.schemes[0] ?? ""
            onValueChanged: value => addressField.text = value + "://"
        }

        DTextField {
            id: addressField

            width: parent.width
            outlined: true
            isError: root.errorText !== ""
            supportingText: root.errorText
            labelText: I18n.tr("Address", "label for a server address input")
            placeholderText: "sftp://host/path"
            onAccepted: root.submit("")
        }

        StyledText {
            visible: root.server !== ""
            text: I18n.tr("Shares on %1", "header above the shares of a browsed server, %1 is the server").arg(root.server)
            color: Theme.primary
            font.pixelSize: Theme.fontSizeSmall
            font.weight: Theme.fontWeightMedium
        }

        StyledText {
            visible: root.server !== "" && root.shares.length === 0
            text: I18n.tr("This server offers no shares", "connect dialog note when a browsed server lists nothing")
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
        }

        Repeater {
            model: root.shares

            FilePlaceRow {
                required property var modelData

                width: parent.width
                label: modelData.name
                subtitle: modelData.comment ?? ""
                iconName: "folder-remote"
                busy: NetworkService.connecting
                onClicked: root.submit(modelData.uri)
            }
        }

        StyledText {
            visible: root.server === "" && NetworkService.servers.length > 0
            text: I18n.tr("Recent", "header above previously used servers")
            color: Theme.primary
            font.pixelSize: Theme.fontSizeSmall
            font.weight: Theme.fontWeightMedium
        }

        Repeater {
            model: root.server === "" ? NetworkService.servers : []

            FilePlaceRow {
                required property var modelData

                width: parent.width
                label: modelData.uri
                iconName: "folder-remote"
                actionIcon: "close"
                busy: NetworkService.connecting
                onClicked: root.submit(modelData.uri)
                onActionTriggered: NetworkService.forget(modelData.uri)
            }
        }
    }

    actions: [
        TextButton {
            text: I18n.tr("Cancel", "dialog button to cancel")
            onClicked: root.close()
        },
        DButton {
            text: I18n.tr("Connect", "connect to server dialog action")
            backgroundColor: Theme.primary
            textColor: Theme.primaryText
            enabled: !NetworkService.connecting
            onClicked: root.submit("")
        }
    ]
}
