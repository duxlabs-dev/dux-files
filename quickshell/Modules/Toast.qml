import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

Item {
    id: root

    property bool shown: true

    z: 1000
    visible: bar.opacity > 0

    Rectangle {
        id: bar

        readonly property bool showing: ToastService.active && root.shown

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Theme.spacingL + (showing ? 0 : Theme.spacingS)
        height: Theme.fieldHeightLarge
        width: Math.min(parent.width - Theme.spacingL * 2, contentRow.implicitWidth + Theme.spacingL * 2)
        radius: Theme.cornerRadiusXS
        color: Theme.inverseSurface
        opacity: showing ? 1 : 0

        Behavior on opacity {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: Theme.expressiveDurations.expressiveFastEffects
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveFastEffects
            }
        }
        Behavior on anchors.bottomMargin {
            enabled: Theme.animationsEnabled
            NumberAnimation {
                duration: Theme.expressiveDurations.expressiveFastSpatial
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Theme.expressiveCurves.expressiveFastSpatial
            }
        }

        Row {
            id: contentRow

            anchors.centerIn: parent
            spacing: Theme.spacingM

            StyledText {
                anchors.verticalCenter: parent.verticalCenter
                text: ToastService.message
                font.pixelSize: Theme.fontSizeMedium
                color: Theme.inverseOnSurface
                wrapMode: Text.NoWrap
                maximumLineCount: 1
                elide: Text.ElideRight
            }

            DButton {
                anchors.verticalCenter: parent.verticalCenter
                visible: ToastService.actionLabel !== ""
                text: ToastService.actionLabel
                buttonHeight: Theme.buttonHeightXS
                backgroundColor: "transparent"
                textColor: Theme.inverseOnSurface
                focusPolicy: Qt.NoFocus
                onClicked: ToastService.runAction()
            }
        }
    }
}
