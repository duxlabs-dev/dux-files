import QtQuick
import qs.Common
import qs.DCommon.Widgets

Item {
    id: root

    property int currentIndex: 0
    property int highlightIndex: -1
    property var matchCounts: []
    property bool fieldOnly: false
    property alias query: searchField.text

    signal tabSelected(int index)
    signal searchAccepted

    function clearSearch() {
        if (searchField.text === "")
            return false;
        searchField.text = "";
        return true;
    }

    function tabOrder() {
        return groups.reduce((all, group) => all.concat(group.map(tab => tab.index)), []);
    }

    function moveHighlight(delta) {
        const order = tabOrder();
        let position = order.indexOf(highlightIndex);
        if (position < 0)
            position = order.indexOf(currentIndex);
        highlightIndex = order[(position + delta + order.length) % order.length];
    }

    function selectHighlighted() {
        if (highlightIndex < 0)
            return false;
        tabSelected(highlightIndex);
        highlightIndex = -1;
        return true;
    }

    function clearHighlight() {
        if (highlightIndex < 0)
            return false;
        highlightIndex = -1;
        return true;
    }

    readonly property var groups: [[
            {
                accent: "blue",
                index: 0,
                label: I18n.tr("General", "settings sidebar tab label"),
                hint: I18n.tr("Views, sorting and defaults", "settings sidebar tab hint"),
                icon: "tune"
            },
            {
                accent: "blue",
                index: 1,
                label: I18n.tr("Appearance", "settings sidebar tab label"),
                hint: I18n.tr("Theme, shape and motion", "settings sidebar tab hint"),
                icon: "palette"
            },
            {
                accent: "blue",
                index: 2,
                label: I18n.tr("Behaviour", "settings sidebar tab label"),
                hint: I18n.tr("Terminal, drag and thumbnails", "settings sidebar tab hint"),
                icon: "touch_app"
            }
        ], [
            {
                accent: "green",
                index: 3,
                label: I18n.tr("Devices", "sidebar section listing drives and volumes"),
                hint: I18n.tr("Drives and mounting", "settings sidebar tab hint"),
                icon: "hard_drive"
            }
        ], [
            {
                accent: "purple",
                index: 4,
                label: I18n.tr("About", "settings sidebar tab label"),
                hint: I18n.tr("Version and capabilities", "settings sidebar tab hint"),
                icon: "info"
            }
        ]]

    implicitWidth: SettingsMetrics.sidebarWidth
    implicitHeight: fieldOnly ? searchField.height + navColumn.padding * 2 : 0

    Rectangle {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: Theme.dividerWidth
        color: Theme.outlineVariant
    }

    DFlickable {
        anchors.fill: parent
        anchors.rightMargin: Theme.dividerWidth
        clip: true
        contentWidth: width
        contentHeight: navColumn.height

        Column {
            id: navColumn
            width: parent.width
            padding: Theme.spacingL
            spacing: SettingsMetrics.sidebarGroupGap

            DSearchField {
                id: searchField

                width: navColumn.width - navColumn.padding * 2
                backgroundColor: Theme.surfaceContainerLowest
                placeholderText: I18n.tr("Search settings", "placeholder in the settings search field")
                onAccepted: root.searchAccepted()
            }

            Repeater {
                model: root.fieldOnly ? [] : root.groups

                Column {
                    id: groupColumn
                    required property var modelData
                    width: navColumn.width - navColumn.padding * 2
                    spacing: Theme.groupedListGap

                    Repeater {
                        model: groupColumn.modelData

                        SettingsSidebarItem {
                            required property var modelData
                            required property int index
                            width: groupColumn.width
                            iconName: modelData.icon
                            title: modelData.label
                            hint: modelData.hint
                            accent: modelData.accent
                            active: root.currentIndex === modelData.index
                            highlighted: root.highlightIndex === modelData.index
                            matchCount: root.matchCounts[modelData.index] ?? -1
                            isFirstInGroup: index === 0
                            isLastInGroup: index === groupColumn.modelData.length - 1
                            onActiveFocusChanged: {
                                if (activeFocus)
                                    root.highlightIndex = modelData.index;
                            }
                            Keys.onReturnPressed: event => event.accepted = root.selectHighlighted()
                            Keys.onEnterPressed: event => event.accepted = root.selectHighlighted()
                            Keys.onSpacePressed: event => event.accepted = root.selectHighlighted()
                            onClicked: {
                                root.highlightIndex = -1;
                                root.tabSelected(modelData.index);
                            }
                        }
                    }
                }
            }
        }
    }
}
