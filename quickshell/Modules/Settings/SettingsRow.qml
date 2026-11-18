pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import qs.Common
import qs.DCommon.Widgets

T.Control {
    id: root

    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    readonly property bool isSettingsRow: true

    property string title: ""
    property string subtitle: ""
    property string keywords: ""
    property bool flashing: false
    property color subtitleColor: Theme.surfaceVariantText
    property string iconName: ""
    property bool clickable: false
    property bool showChevron: false
    property string trailingBadge: ""
    property color trailingBadgeColor: Theme.surfaceVariantText
    property real paddingH: SettingsMetrics.rowPaddingH
    property real paddingV: SettingsMetrics.rowPaddingV
    property color rowColor: SettingsMetrics.rowColor
    property color iconColor: Theme.primary
    property color titleColor: Theme.surfaceText
    property string settingKey: ""
    readonly property bool modified: settingKey !== "" && SettingsData.hasDefault(settingKey) && !SettingsData.isDefault(settingKey)

    default property alias trailing: trailingSlot.data
    property alias leading: leadingSlot.data
    property alias body: bodySlot.data

    readonly property bool hasBody: bodySlot.visibleChildren.length > 0
    readonly property bool hasText: title !== "" || subtitle !== ""
    readonly property bool isFirstInGroup: edgeRow(true) === root
    readonly property bool isLastInGroup: edgeRow(false) === root
    property real topRadius: isFirstInGroup ? Theme.groupedListOuterRadius : Theme.groupedListInnerRadius
    property real bottomRadius: isLastInGroup ? Theme.groupedListOuterRadius : Theme.groupedListInnerRadius
    readonly property real minHeight: subtitle !== "" ? SettingsMetrics.rowMinHeightTwoLine : SettingsMetrics.rowMinHeight

    signal clicked

    function flash() {
        flashing = true;
        flashTimer.restart();
    }

    Timer {
        id: flashTimer

        interval: SettingsMetrics.flashHold
        onTriggered: root.flashing = false
    }

    focusPolicy: clickable ? Qt.StrongFocus : Qt.NoFocus
    Accessible.role: clickable ? Accessible.Button : Accessible.NoRole
    Accessible.name: title
    Accessible.description: subtitle
    Accessible.onPressAction: {
        if (clickable && enabled)
            clicked();
    }

    Keys.onPressed: event => {
        if (!clickable || !enabled)
            return;
        switch (event.key) {
        case Qt.Key_Space:
        case Qt.Key_Return:
        case Qt.Key_Enter:
            root.clicked();
            event.accepted = true;
            break;
        }
    }

    function edgeRow(first) {
        const siblings = parent?.visibleChildren;
        if (!siblings)
            return root;
        let last = null;
        for (let i = 0; i < siblings.length; i++) {
            const sibling = siblings[i];
            if (sibling.isSettingsRow !== true)
                continue;
            if (first)
                return sibling;
            last = sibling;
        }
        return last;
    }

    width: parent?.width ?? 0
    height: implicitHeight
    implicitHeight: paddingV * 2 + headerLine.height + (hasBody ? bodySlot.height + (headerLine.height > 0 ? Theme.spacingM : 0) : 0)

    Rectangle {
        anchors.fill: parent
        color: root.flashing ? Theme.blend(root.rowColor, Theme.primary, SettingsMetrics.highlightBlend) : root.rowColor
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius

        Behavior on color {
            enabled: Theme.animationsEnabled
            ColorAnimation {
                duration: SettingsMetrics.fadeDuration
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveEffects
            }
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: root.clickable
        color: Theme.surfaceText
        opacity: !root.enabled ? 0 : (clickControl.down ? Theme.stateLayerPressed : (clickControl.hovered ? Theme.stateLayerHover : 0))
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius

        Behavior on opacity {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: SettingsMetrics.fadeDuration
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveEffects
            }
        }
    }

    DRipple {
        id: ripple
        visible: root.clickable
        rippleColor: Theme.surfaceText
        topLeftRadius: root.topRadius
        topRightRadius: root.topRadius
        bottomLeftRadius: root.bottomRadius
        bottomRightRadius: root.bottomRadius
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: Theme.focusRingWidth / 2
        topLeftRadius: Math.max(0, root.topRadius - Theme.focusRingWidth / 2)
        topRightRadius: topLeftRadius
        bottomLeftRadius: Math.max(0, root.bottomRadius - Theme.focusRingWidth / 2)
        bottomRightRadius: bottomLeftRadius
        color: "transparent"
        border.width: Theme.focusRingWidth
        border.color: Theme.focusRingColor
        visible: root.visualFocus && root.clickable
    }

    StyledButton {
        id: clickControl
        anchors.fill: parent
        enabled: root.clickable && root.enabled
        hoverEnabled: root.clickable
        focusPolicy: Qt.ClickFocus
        background: null
        Accessible.ignored: true
        onPressedChanged: {
            if (pressed)
                ripple.trigger(pressX, pressY);
        }
        onClicked: root.clicked()

        HoverHandler {
            cursorShape: root.clickable ? Qt.PointingHandCursor : Qt.ArrowCursor
        }
    }

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.topMargin: root.paddingV
        anchors.leftMargin: root.paddingH
        anchors.rightMargin: root.paddingH
        spacing: Theme.spacingM

        Item {
            id: headerLine
            visible: height > 0
            width: parent.width
            implicitHeight: Math.max(textColumn.implicitHeight, trailingArea.implicitHeight, leadingArea.visible ? leadingArea.implicitHeight : 0)
            height: Math.max(implicitHeight, (root.hasText || trailingSlot.children.length > 0 ? root.minHeight : 0) - root.paddingV * 2)

            Row {
                id: leadingArea
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Theme.spacingL
                visible: leadingSlot.children.length > 0 || root.iconName !== ""
                opacity: root.enabled ? 1 : SettingsMetrics.disabledOpacity

                Row {
                    id: leadingSlot
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Theme.spacingS
                    visible: children.length > 0
                }

                DIcon {
                    id: leadingIcon
                    anchors.verticalCenter: parent.verticalCenter
                    name: root.iconName
                    size: Theme.iconSize
                    color: root.iconColor
                    visible: root.iconName !== ""
                }
            }

            Column {
                id: textColumn
                anchors.left: leadingArea.visible ? leadingArea.right : parent.left
                anchors.leftMargin: leadingArea.visible ? Theme.spacingL : 0
                anchors.right: trailingArea.visible ? trailingArea.left : parent.right
                anchors.rightMargin: trailingArea.visible ? SettingsMetrics.rowContentSpacing : 0
                anchors.verticalCenter: parent.verticalCenter
                spacing: Theme.spacingXXS
                opacity: root.enabled ? 1 : SettingsMetrics.disabledOpacity
                visible: root.hasText

                Item {
                    id: titleLine

                    readonly property bool resettable: root.settingKey !== "" && SettingsData.hasDefault(root.settingKey)
                    readonly property real chipTrailing: Theme.spacingXS + Theme.iconSizeSmall + SettingsMetrics.resetChipPaddingH

                    width: parent.width
                    height: titleText.implicitHeight
                    visible: root.title !== ""

                    // The chip bleeds past the title box on every side, so the title never moves when a row turns modified
                    Loader {
                        id: resetChip
                        anchors.left: parent.left
                        anchors.leftMargin: -SettingsMetrics.resetChipPaddingH
                        y: -SettingsMetrics.resetChipPaddingV
                        width: SettingsMetrics.resetChipPaddingH + titleText.contentWidth + titleLine.chipTrailing
                        height: titleText.implicitHeight + SettingsMetrics.resetChipPaddingV * 2
                        active: root.modified && root.title !== ""

                        sourceComponent: StyledButton {
                            id: chip
                            radius: Theme.fullRadius(width, height)
                            color: SettingsMetrics.controlColor
                            enabled: root.enabled
                            Accessible.name: I18n.tr("Reset to default", "settings row button restoring the density default")
                            Accessible.description: root.title
                            onClicked: SettingsData.resetToDefault(root.settingKey)

                            FocusRing {
                                visible: chip.visualFocus
                            }

                            DIcon {
                                anchors.right: parent.right
                                anchors.rightMargin: SettingsMetrics.resetChipPaddingH - Theme.spacingXXS
                                anchors.verticalCenter: parent.verticalCenter
                                name: "restart_alt"
                                size: Theme.iconSizeSmall
                                color: Theme.surfaceVariantText
                            }

                            StateLayer {
                                control: chip
                                stateColor: root.titleColor
                                tooltipText: I18n.tr("Reset to default", "settings row button restoring the density default")
                            }
                        }
                    }

                    StyledText {
                        id: titleText
                        anchors.left: parent.left
                        width: parent.width - (titleLine.resettable ? titleLine.chipTrailing : 0)
                        text: root.title
                        font.pixelSize: Theme.fontSizeMedium
                        font.weight: Theme.fontWeightMedium
                        color: root.titleColor
                        wrapMode: Text.WordWrap
                        horizontalAlignment: Text.AlignLeft
                    }
                }

                StyledText {
                    width: parent.width
                    text: root.subtitle
                    font.pixelSize: Theme.fontSizeSmall
                    color: root.subtitleColor
                    wrapMode: Text.WordWrap
                    visible: root.subtitle !== ""
                    horizontalAlignment: Text.AlignLeft
                }
            }

            Row {
                id: trailingArea
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Theme.spacingS
                visible: trailingSlot.children.length > 0 || root.showChevron || root.trailingBadge !== ""

                Row {
                    id: trailingSlot
                    spacing: Theme.spacingS
                    anchors.verticalCenter: parent.verticalCenter
                }

                StyledText {
                    text: root.trailingBadge
                    font.pixelSize: Theme.fontSizeSmall
                    color: root.trailingBadgeColor
                    visible: root.trailingBadge !== ""
                    anchors.verticalCenter: parent.verticalCenter
                }

                DIcon {
                    name: "chevron_right"
                    size: Theme.iconSize
                    color: Theme.surfaceVariantText
                    rotation: I18n.isRtl ? 180 : 0
                    visible: root.showChevron
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }

        Column {
            id: bodySlot
            width: parent.width
            spacing: Theme.spacingM
        }
    }
}
