pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Item {
    id: overview

    property var tabs: []
    property int currentIndex: 0
    property var titleFor: state => ""
    property var pathFor: (state, index) => ""
    property bool showHidden: false
    property var listings: ({})

    signal tabSelected(int index)
    signal tabClosed(int index)
    signal tabAdded
    signal tabDropped(int index, var drop)
    signal newTabDropped(var drop)

    function load(path) {
        if (path === "" || listings[path] !== undefined)
            return;
        listings[path] = [];
        DirectoryService.list(path, {
            "limit": FilesMetrics.overviewEntries,
            "includeHidden": showHidden,
            "dirsFirst": SettingsData.foldersFirst,
            "hiddenLast": SettingsData.hiddenLast,
            "sortKey": "name"
        }, result => {
            const next = Object.assign({}, overview.listings);
            next[path] = result.error ? [] : (result.entries ?? []);
            overview.listings = next;
        });
    }

    component Card: StyledButton {
        id: card

        property bool current: false
        property bool hot: false

        readonly property color contentColor: current ? Theme.onSelectedContainer : Theme.surfaceText

        focusPolicy: Qt.TabFocus
        width: FilesMetrics.overviewCardWidth
        height: FilesMetrics.overviewCardHeight
        radius: Theme.cornerRadiusL
        color: hot ? Theme.primaryContainer : current ? Theme.selectedContainer : Theme.chipSurface
        border.width: hot || current ? Theme.outlineWidthFocused : Theme.outlineWidth
        border.color: hot ? Theme.primary : current ? Theme.outline : Theme.outlineVariant

        StateLayer {
            control: card
            stateColor: card.contentColor
        }

        FocusRing {
            visible: card.visualFocus
        }
    }

    DFlickable {
        anchors.fill: parent
        anchors.margins: Theme.spacingM
        contentHeight: sheet.height
        clip: true

        Column {
            id: sheet

            width: parent.width
            spacing: Theme.spacingM

            StyledText {
                width: parent.width
                visible: DragService.active
                text: I18n.tr("Drop on a tab to move files there", "tab overview hint shown while files are dragged")
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                horizontalAlignment: Text.AlignHCenter
            }

            Flow {
                width: parent.width
                spacing: Theme.spacingM

                Repeater {
                    model: overview.tabs.length

                    Card {
                        id: tabCard

                        required property int index

                        readonly property string path: overview.pathFor(overview.tabs[index], index)
                        readonly property var entries: overview.listings[path] ?? []

                        current: index === overview.currentIndex
                        hot: tabDrop.containsDrag
                        Accessible.name: title.text
                        onPathChanged: overview.load(path)
                        Component.onCompleted: {
                            overview.load(path);
                            if (current)
                                forceActiveFocus();
                        }
                        onClicked: overview.tabSelected(index)

                        TapHandler {
                            acceptedButtons: Qt.MiddleButton
                            onTapped: {
                                if (overview.tabs.length > 1)
                                    overview.tabClosed(tabCard.index);
                            }
                        }

                        Rectangle {
                            id: glyph

                            anchors.left: parent.left
                            anchors.leftMargin: Theme.spacingS
                            anchors.verticalCenter: closeButton.verticalCenter
                            width: Theme.buttonHeightXXS
                            height: width
                            radius: Theme.fullRadius(width, height)
                            color: Theme.withAlpha(tabCard.contentColor, Theme.tonalTintAlpha)

                            DIcon {
                                anchors.centerIn: parent
                                name: "folder"
                                size: Theme.iconSizeSmall
                                filled: true
                                color: tabCard.contentColor
                            }
                        }

                        StyledText {
                            id: title

                            anchors.left: glyph.right
                            anchors.right: closeButton.left
                            anchors.leftMargin: Theme.spacingS
                            anchors.rightMargin: Theme.spacingXS
                            anchors.verticalCenter: closeButton.verticalCenter
                            text: overview.titleFor(overview.tabs[tabCard.index])
                            color: tabCard.contentColor
                            font.pixelSize: Theme.fontSizeMedium
                            font.weight: Theme.fontWeightMedium
                            horizontalAlignment: Text.AlignLeft
                            elide: Text.ElideMiddle
                        }

                        DActionButton {
                            id: closeButton

                            anchors.top: parent.top
                            anchors.right: parent.right
                            anchors.margins: Theme.spacingXS
                            visible: overview.tabs.length > 1
                            buttonSize: Theme.buttonHeightXS
                            iconName: "close"
                            iconSize: Theme.iconSizeSmall
                            iconColor: tabCard.contentColor
                            tooltipText: I18n.tr("Close tab", "tooltip on the button closing a tab")
                            onClicked: overview.tabClosed(tabCard.index)
                        }

                        Rectangle {
                            anchors.top: closeButton.bottom
                            anchors.bottom: parent.bottom
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.margins: Theme.spacingXS
                            radius: Math.max(0, tabCard.radius - Theme.spacingXS)
                            color: Theme.foregroundColor(Theme.cardSurface, true)
                            clip: true

                            Grid {
                                id: thumbs

                                readonly property real cell: Math.floor((parent.width - Theme.spacingS * 2) / columns)

                                anchors.top: parent.top
                                anchors.horizontalCenter: parent.horizontalCenter
                                anchors.topMargin: Theme.spacingS
                                columns: FilesMetrics.overviewColumns

                                Repeater {
                                    model: tabCard.entries

                                    Column {
                                        id: thumb

                                        required property var modelData

                                        width: thumbs.cell
                                        topPadding: Theme.spacingXS
                                        bottomPadding: Theme.spacingXS
                                        spacing: Theme.spacingXXS

                                        FileIcon {
                                            anchors.horizontalCenter: parent.horizontalCenter
                                            iconName: thumb.modelData.iconName ?? (thumb.modelData.isDir ? "folder" : "")
                                            size: Theme.iconSize
                                        }

                                        StyledText {
                                            width: parent.width - Theme.spacingXS
                                            anchors.horizontalCenter: parent.horizontalCenter
                                            text: thumb.modelData.name
                                            color: Theme.surfaceVariantText
                                            font.pixelSize: Theme.fontSizeSmall
                                            horizontalAlignment: Text.AlignHCenter
                                            elide: Text.ElideMiddle
                                        }
                                    }
                                }
                            }
                        }

                        FileDropArea {
                            id: tabDrop

                            anchors.fill: parent
                            dropPath: tabCard.path
                            onDropped: drop => overview.tabDropped(tabCard.index, drop)
                        }
                    }
                }

                Card {
                    id: newCard

                    hot: newDrop.containsDrag
                    color: hot ? Theme.primaryContainer : "transparent"
                    Accessible.name: newLabel.text
                    onClicked: overview.tabAdded()

                    Column {
                        anchors.centerIn: parent
                        spacing: Theme.spacingS

                        DIcon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            name: "add"
                            size: Theme.iconSize
                            color: newCard.contentColor
                        }

                        StyledText {
                            id: newLabel

                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("New tab", "tooltip on the button opening a new tab")
                            color: newCard.contentColor
                            font.pixelSize: Theme.fontSizeMedium
                        }
                    }

                    DropArea {
                        id: newDrop

                        anchors.fill: parent
                        onDropped: drop => overview.newTabDropped(drop)
                    }
                }
            }
        }
    }
}
