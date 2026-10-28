import QtQuick
import qs.Common
import qs.DCommon.Widgets

DFlickable {
    id: root

    readonly property bool isSettingsPage: true

    default property alias content: column.data
    property alias spacing: column.spacing
    property int contentMaxWidth: SettingsMetrics.contentMaxWidth

    function revealRow(row) {
        const top = row.mapToItem(root.contentItem, 0, 0).y;
        revealRange(top - Theme.spacingL, top + row.height + Theme.spacingL);
    }

    anchors.fill: parent
    clip: true
    contentHeight: column.height + Theme.spacingXL
    contentWidth: width

    Column {
        id: column
        topPadding: Theme.spacingXS
        width: Math.min(root.contentMaxWidth, parent.width - Theme.spacingL * 2)
        bottomPadding: SettingsMetrics.pagePaddingV
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: SettingsMetrics.cardGap
    }
}
