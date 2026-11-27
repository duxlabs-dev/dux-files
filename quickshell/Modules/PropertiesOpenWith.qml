pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

Column {
    id: openWith

    property string mime: ""
    property string path: ""
    property string resolvedMime: ""
    property var apps: []
    property int requestSerial: 0

    readonly property bool canSetDefault: SandboxService.available("setDefault")

    onMimeChanged: Qt.callLater(reload)
    onPathChanged: {
        if (mime === "")
            Qt.callLater(reload);
    }
    Component.onCompleted: reload()

    function reload() {
        const token = ++requestSerial;
        apps = [];
        resolvedMime = mime;
        if (mime === "" && path === "")
            return;
        AppsService.appsForMime(mime, path, response => {
            if (response.error || token !== openWith.requestSerial)
                return;
            openWith.resolvedMime = response.mimeType || "";
            openWith.apps = response.apps || [];
        });
    }

    spacing: Theme.spacingM

    StyledText {
        width: parent.width
        visible: text !== ""
        text: openWith.resolvedMime
        color: Theme.surfaceVariantText
        font.pixelSize: Theme.fontSizeSmall
        elide: Text.ElideRight
    }

    StyledText {
        width: parent.width
        visible: !openWith.canSetDefault
        text: FilesFormat.featureUnavailable("PORTALCHOOSER")
        color: Theme.surfaceVariantText
        font.pixelSize: Theme.fontSizeSmall
        wrapMode: Text.Wrap
    }

    DListView {
        width: parent.width
        height: Math.min(count, 4) * Theme.listItemHeight + Math.max(0, Math.min(count, 4) - 1) * spacing
        clip: true
        spacing: Theme.groupedListGap
        model: openWith.apps

        delegate: AppRow {
            onClicked: {
                if (!openWith.canSetDefault)
                    return;
                AppsService.setDefault(openWith.resolvedMime, modelData.id, () => openWith.reload());
            }
        }
    }
}
