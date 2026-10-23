pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var entry: null
    property int tab: 0
    property string filter: ""
    property bool searching: false
    property bool typingAddress: false
    property bool pickingColor: false
    property real hue: 0
    property real saturation: 1
    property real brightness: 1

    readonly property string path: entry?.path ?? ""
    readonly property var look: FolderAppearances.lookup(path)
    readonly property var names: tab === 0 ? FolderAppearances.badges : FolderAppearances.icons
    readonly property bool customColor: (look?.color ?? "") !== "" && !FolderAppearances.colors.includes(look.color)
    readonly property var shown: filter === "" ? names : names.filter(name => name.toLowerCase().includes(filter.toLowerCase()))

    maximumWidth: 520
    title: I18n.tr("Folder appearance", "folder appearance dialog title")

    function show(target) {
        entry = target;
        tab = 0;
        filter = "";
        search.text = "";
        searching = false;
        typingAddress = false;
        pickingColor = false;
        address.text = FolderAppearances.emblemFor(target.path).startsWith("http") ? FolderAppearances.emblemFor(target.path) : "";
        FolderAppearances.refreshCatalog();
        open();
    }

    signal browseRequested

    function openColorPicker() {
        const current = Qt.color(look?.color || "#448aff");
        hue = Math.max(0, current.hsvHue);
        saturation = current.hsvSaturation;
        brightness = current.hsvValue;
        pickingColor = !pickingColor;
    }

    function applyAddress() {
        const value = address.text.trim();
        if (/^https?:\/\/\S+$/.test(value))
            apply({
                "emblem": value,
                "icon": ""
            });
    }

    function applyPicked() {
        apply({
            "color": Qt.hsva(hue, saturation, brightness, 1).toString()
        });
    }

    function apply(values) {
        FolderAppearances.set(path, Object.assign({}, root.look ?? {}, values));
    }

    ColumnLayout {
        width: parent.width
        spacing: Theme.spacingM

        RowLayout {
            spacing: Theme.spacingM

            FileItemVisual {
                Layout.preferredWidth: 64
                Layout.preferredHeight: 64
                iconSize: 64
                iconName: "folder"
                path: root.path
                isDir: true
            }

            StyledText {
                Layout.fillWidth: true
                text: root.entry?.name ?? ""
                color: Theme.surfaceText
                elide: Text.ElideMiddle
            }
        }

        RowLayout {
            spacing: Theme.spacingS

            Repeater {
                model: FolderAppearances.colors

                Rectangle {
                    required property string modelData

                    width: 32
                    height: 32
                    radius: width / 2
                    color: modelData === "" ? Theme.surfaceContainerHigh : modelData
                    border.width: (root.look?.color ?? "") === modelData ? Theme.focusRingWidth : 1
                    border.color: Theme.outlineVariant

                    TapHandler {
                        onTapped: {
                            root.pickingColor = false;
                            root.apply({
                                "color": modelData
                            });
                        }
                    }
                }
            }

            Rectangle {
                width: 32
                height: 32
                radius: width / 2
                color: root.customColor ? root.look.color : Theme.surfaceContainerHigh
                border.width: root.customColor || root.pickingColor ? Theme.focusRingWidth : 1
                border.color: Theme.outlineVariant

                DIcon {
                    anchors.centerIn: parent
                    name: "colorize"
                    size: 18
                    color: Theme.surfaceText
                }

                TapHandler {
                    onTapped: root.openColorPicker()
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            visible: root.pickingColor
            spacing: Theme.spacingM

            DSaturationValuePicker {
                Layout.preferredWidth: 160
                Layout.preferredHeight: 120
                hue: root.hue
                saturation: root.saturation
                value: root.brightness
                onColorChanged: (s, v) => {
                    root.saturation = s;
                    root.brightness = v;
                    root.applyPicked();
                }
            }

            DSlider {
                Layout.fillWidth: true
                minimum: 0
                maximum: 359
                value: Math.round(root.hue * 360)
                showValue: false
                fillColor: Qt.hsva(root.hue, 1, 1, 1)
                trackGradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0 / 6; color: Qt.hsva(0 / 6, 1, 1, 1) }
                    GradientStop { position: 1 / 6; color: Qt.hsva(1 / 6, 1, 1, 1) }
                    GradientStop { position: 2 / 6; color: Qt.hsva(2 / 6, 1, 1, 1) }
                    GradientStop { position: 3 / 6; color: Qt.hsva(3 / 6, 1, 1, 1) }
                    GradientStop { position: 4 / 6; color: Qt.hsva(4 / 6, 1, 1, 1) }
                    GradientStop { position: 5 / 6; color: Qt.hsva(5 / 6, 1, 1, 1) }
                    GradientStop { position: 6 / 6; color: Qt.hsva(0, 1, 1, 1) }
                }
                onSliderValueChanged: newValue => {
                    root.hue = newValue / 360;
                    root.applyPicked();
                }
            }
        }

        RowLayout {
            spacing: Theme.spacingS

            DFilterChips {
                Layout.fillWidth: true
                model: [{
                    "value": "badges",
                    "label": I18n.tr("Badges", "folder appearance tab listing badge icons")
                }, {
                    "value": "icons",
                    "label": I18n.tr("Icons", "folder appearance tab listing folder icons")
                }]
                currentIndex: root.tab
                showCounts: false
                onSelectionChanged: index => root.tab = index
            }

            DIconButton {
                iconName: "search"
                checkable: true
                checked: root.searching
                tooltipText: I18n.tr("Search icons", "folder appearance button toggling the icon search")
                onClicked: {
                    root.searching = !root.searching;
                    if (root.searching)
                        search.forceActiveFocus();
                    else
                        search.text = "";
                }
            }

            DIconButton {
                visible: root.tab === 0
                iconName: "folder_open"
                tooltipText: I18n.tr("Use an image from this computer", "folder appearance button picking a local badge image")
                onClicked: root.browseRequested()
            }

            DIconButton {
                visible: root.tab === 0
                iconName: "link"
                checkable: true
                checked: root.typingAddress
                tooltipText: I18n.tr("Use an image from a web address", "folder appearance button typing a web badge address")
                onClicked: {
                    root.typingAddress = !root.typingAddress;
                    if (root.typingAddress)
                        address.forceActiveFocus();
                }
            }

            TextButton {
                text: I18n.tr("Clear", "folder appearance action clearing the badge")
                visible: String(root.look?.emblem ?? "") !== "" || String(root.look?.icon ?? "") !== ""
                onClicked: root.apply({
                    "emblem": "",
                    "icon": ""
                })
            }
        }

        DSearchField {
            id: search

            Layout.fillWidth: true
            visible: root.searching
            placeholderText: I18n.tr("Search icon", "placeholder of the folder appearance icon search")
            onTextChanged: root.filter = text
        }

        RowLayout {
            Layout.fillWidth: true
            visible: root.typingAddress && root.tab === 0
            spacing: Theme.spacingS

            DTextField {
                id: address

                Layout.fillWidth: true
                outlined: true
                labelText: I18n.tr("Image web address", "placeholder of the folder appearance badge web address field")
                onAccepted: root.applyAddress()
            }

            DButton {
                text: I18n.tr("Use", "folder appearance button applying the typed badge web address")
                enabled: /^https?:\/\/\S+$/.test(address.text.trim())
                backgroundColor: Theme.primary
                textColor: Theme.primaryText
                onClicked: root.applyAddress()
            }
        }

        DGridView {
            Layout.fillWidth: true
            Layout.preferredHeight: 280
            clip: true
            cellWidth: 64
            cellHeight: 64
            model: root.shown

            delegate: Rectangle {
                required property string modelData

                width: 56
                height: 56
                radius: Theme.cornerRadiusS
                color: (root.tab === 0 ? root.look?.emblem : root.look?.icon) === modelData ? Theme.primaryContainer : "transparent"

                DSVGIcon {
                    anchors.centerIn: parent
                    size: 40
                    source: Paths.resolveIconPath(modelData)
                }

                TapHandler {
                    onTapped: root.apply(root.tab === 0 ? {
                        "emblem": modelData,
                        "icon": ""
                    } : {
                        "icon": modelData,
                        "emblem": ""
                    })
                }
            }
        }
    }

    actions: [
        TextButton {
            text: I18n.tr("Restore default", "folder appearance action dropping color and badge")
            onClicked: FolderAppearances.reset(root.path)
        },
        DButton {
            text: I18n.tr("Close", "dialog button closing the dialog")
            backgroundColor: Theme.primary
            textColor: Theme.primaryText
            onClicked: root.close()
        }
    ]
}
