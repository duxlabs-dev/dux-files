import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Rectangle {
    id: side

    property string label: ""
    property var entry: null

    implicitHeight: content.implicitHeight + Theme.spacingM * 2
    radius: Theme.cornerRadiusM
    color: Theme.chipSurface

    RowLayout {
        id: content

        anchors.fill: parent
        anchors.margins: Theme.spacingM
        spacing: Theme.spacingM

        FileIcon {
            iconName: side.entry ? side.entry.iconName : ""
            thumbnail: side.entry ? side.entry.thumbnail : ""
            size: Theme.iconSize
            color: Theme.surfaceText
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXXS

            StyledText {
                Layout.fillWidth: true
                text: side.label
                color: Theme.primary
                font.pixelSize: Theme.fontSizeSmall
            }

            StyledText {
                Layout.fillWidth: true
                text: side.entry ? side.entry.name : ""
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeMedium
                elide: Text.ElideMiddle
            }

            StyledText {
                Layout.fillWidth: true
                visible: text !== ""
                text: {
                    if (!side.entry)
                        return "";
                    const parts = [FileFormat.size(side.entry.size), FileFormat.modified(side.entry.mtimeMs)];
                    return parts.filter(part => part !== "").join(" · ");
                }
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
            }
        }
    }
}
