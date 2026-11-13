pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Item {
    id: bar

    property int total: 0
    property int hidden: 0
    property string hint: ""
    property string viewMode: "list"
    property var viewModes: []
    property bool previewOpen: false
    property string previewLabel: ""
    property bool showSort: true
    property var sortKeys: []
    property string sortKey: "name"
    property bool sortDescending: false

    signal viewModeSelected(string mode)
    signal previewToggled
    signal sortKeySelected(string key)
    signal sortDirectionToggled

    implicitHeight: FilesMetrics.statusBarHeight
    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    PaneStatus {
        anchors.left: parent.left
        anchors.leftMargin: Theme.spacingS
        anchors.verticalCenter: parent.verticalCenter
        total: bar.total
        hidden: bar.hidden
        hint: bar.hint
        textRoom: bar.width - controls.width - Theme.spacingS * 2
        compact: bar.width - controls.width < FilesMetrics.searchMinWidth
    }

    Row {
        id: controls

        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: FilesMetrics.topbarSpacing

        FileSortButton {
            anchors.verticalCenter: parent.verticalCenter
            visible: bar.showSort
            keys: bar.sortKeys
            sortKey: bar.sortKey
            descending: bar.sortDescending
            showLabel: bar.width >= Theme.smallBreakpoint
            tooltipSide: "top"
            onSortKeySelected: key => bar.sortKeySelected(key)
            onDirectionToggled: bar.sortDirectionToggled()
        }

        Row {
            anchors.verticalCenter: parent.verticalCenter
            spacing: FileBrowserMetrics.navPairGap

            Repeater {
                model: bar.viewModes

                ControlSegment {
                    required property var modelData
                    required property int index

                    readonly property bool active: bar.viewMode === modelData.value

                    joinStart: !active && index > 0
                    joinEnd: !active && index < bar.viewModes.length - 1
                    color: active ? Theme.selectedContainer : Theme.chipSurface
                    iconName: modelData.icon
                    iconColor: active ? Theme.onSelectedContainer : Theme.surfaceText
                    tooltipText: modelData.label
                    tooltipSide: "top"
                    Accessible.checked: active
                    onClicked: bar.viewModeSelected(modelData.value)
                }
            }
        }

        ControlSegment {
            anchors.verticalCenter: parent.verticalCenter
            width: FilesMetrics.topbarControlSize
            color: bar.previewOpen ? Theme.selectedContainer : Theme.chipSurface
            iconName: bar.previewOpen ? "preview_off" : "preview"
            iconColor: bar.previewOpen ? Theme.onSelectedContainer : Theme.surfaceText
            tooltipText: bar.previewLabel
            tooltipSide: "top"
            Accessible.checked: bar.previewOpen
            onClicked: bar.previewToggled()
        }
    }
}
