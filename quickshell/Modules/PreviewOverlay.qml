pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser
import "../Common/PreviewView.js" as PreviewView

FocusScope {
    id: overlay

    property var entry: null
    property bool open: false
    property real zoom: 1
    property real angle: 0
    property real panX: 0
    property real panY: 0

    readonly property string path: entry?.path ?? ""
    readonly property string kind: open && entry ? PreviewView.kindFor(entry.mime) : ""
    readonly property bool turned: PreviewView.normalizeAngle(angle) % 180 !== 0
    readonly property real devicePixelRatio: Window.window?.devicePixelRatio ?? Screen.devicePixelRatio
    readonly property int rasterBucket: Math.min(4, Math.max(1, Math.ceil(zoom)))
    readonly property alias pageView: pages.item
    readonly property string errorMessage: {
        if (kind === "pdf" && PdfService.unavailable)
            return I18n.tr("PDF preview is not available. Install Qt PDF support to view this file.", "large preview warning when the optional Qt PDF module is missing");
        if ((kind === "image" && picture.status === Image.Error) || (kind === "pdf" && (pages.status === Loader.Error || pageView?.failed)))
            return I18n.tr("This file could not be opened", "large preview placeholder");
        return "";
    }

    signal closed

    visible: open
    opacity: open ? 1 : 0
    z: 2
    focus: open

    Behavior on opacity {
        enabled: Theme.animationsEnabled
        NumberAnimation {
            duration: Theme.expressiveDurations.expressiveEffects
        }
    }

    function present(target) {
        if (!supports(target))
            return;
        entry = target;
        reset();
        open = true;
        if (kind === "pdf")
            PdfService.ensureProbed();
        forceActiveFocus();
    }

    function supports(target) {
        return PreviewView.kindFor(target?.mime ?? "") !== "";
    }

    function dismiss() {
        if (!open)
            return;
        open = false;
        entry = null;
        closed();
    }

    function reset() {
        zoom = 1;
        angle = 0;
        panX = 0;
        panY = 0;
    }

    function panLimitX() {
        return PreviewView.panLimit(stage.width, stage.height, zoom, angle);
    }

    function panLimitY() {
        return PreviewView.panLimit(stage.height, stage.width, zoom, angle);
    }

    function panTo(x, y) {
        panX = PreviewView.clampPan(x, panLimitX());
        panY = PreviewView.clampPan(y, panLimitY());
    }

    function zoomBy(direction) {
        zoom = PreviewView.steppedZoom(zoom, direction);
        panTo(panX, panY);
    }

    function rotateBy(step) {
        angle = PreviewView.normalizeAngle(angle + step);
        panTo(panX, panY);
    }

    Keys.onPressed: event => {
        switch (event.key) {
        case Qt.Key_Escape:
            dismiss();
            break;
        case Qt.Key_Plus:
        case Qt.Key_Equal:
            zoomBy(1);
            break;
        case Qt.Key_Minus:
            zoomBy(-1);
            break;
        case Qt.Key_0:
            reset();
            break;
        case Qt.Key_R:
            rotateBy(event.modifiers & Qt.ShiftModifier ? -90 : 90);
            break;
        case Qt.Key_Left:
            panTo(panX + 48, panY);
            break;
        case Qt.Key_Right:
            panTo(panX - 48, panY);
            break;
        case Qt.Key_Up:
            panTo(panX, panY + 48);
            break;
        case Qt.Key_Down:
            panTo(panX, panY - 48);
            break;
        default:
            return;
        }
        event.accepted = true;
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.scrimColor
        opacity: 0.7
    }

    MouseArea {
        anchors.fill: parent
        onClicked: overlay.dismiss()
    }

    Rectangle {
        id: card

        anchors.fill: parent
        anchors.margins: Theme.spacingXL
        radius: Theme.cornerRadiusL
        color: Theme.surfaceContainer
        border.width: Theme.outlineWidth
        border.color: Theme.outlineVariant

        MouseArea {
            anchors.fill: parent
            onClicked: {}
        }

        Item {
            id: header

            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: Theme.spacingM
            height: Theme.buttonHeightS

            StyledText {
                anchors.left: parent.left
                anchors.right: closeButton.left
                anchors.rightMargin: Theme.spacingM
                anchors.verticalCenter: parent.verticalCenter
                text: overlay.entry?.name ?? ""
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeLarge
                font.weight: Theme.fontWeightMedium
                elide: Text.ElideMiddle
            }

            DActionButton {
                id: closeButton

                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                iconName: "close"
                tooltipText: I18n.tr("Close", "tooltip on the button closing the large preview")
                onClicked: overlay.dismiss()
            }
        }

        Item {
            id: stage

            anchors.top: header.bottom
            anchors.bottom: controls.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: Theme.spacingM
            clip: true

            Image {
                id: picture

                anchors.fill: parent
                anchors.margins: Theme.spacingS
                source: visible && overlay.path !== "" ? FilePaths.toFileUrl(overlay.path) : ""
                sourceSize.width: Math.max(1, Math.ceil(width * overlay.devicePixelRatio * overlay.rasterBucket))
                sourceSize.height: Math.max(1, Math.ceil(height * overlay.devicePixelRatio * overlay.rasterBucket))
                visible: overlay.kind === "image"
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                cache: false
                smooth: true
                scale: overlay.zoom
                rotation: overlay.angle

                transform: Translate {
                    x: overlay.panX
                    y: overlay.panY
                }
            }

            Loader {
                id: pages

                anchors.fill: parent
                active: overlay.kind === "pdf" && PdfService.available
                source: active ? "PreviewPdf.qml" : ""
                onLoaded: {
                    item.source = Qt.binding(() => FilePaths.toFileUrl(overlay.path));
                    item.zoom = Qt.binding(() => overlay.zoom);
                    item.angle = Qt.binding(() => overlay.angle);
                }
            }

            MouseArea {
                id: pans

                anchors.fill: parent
                enabled: overlay.kind === "image" && (overlay.zoom > 1 || overlay.turned)
                cursorShape: pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                acceptedButtons: Qt.LeftButton

                property real originX: 0
                property real originY: 0
                property real startX: 0
                property real startY: 0

                onPressed: mouse => {
                    originX = mouse.x;
                    originY = mouse.y;
                    startX = overlay.panX;
                    startY = overlay.panY;
                }
                onPositionChanged: mouse => {
                    if (!pressed)
                        return;
                    overlay.panTo(startX + mouse.x - originX, startY + mouse.y - originY);
                }
            }

            WheelHandler {
                enabled: overlay.kind === "image"
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: event => {
                    const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                    if (delta !== 0)
                        overlay.zoomBy(delta > 0 ? 1 : -1);
                }
            }

            StyledText {
                anchors.centerIn: parent
                width: parent.width - Theme.spacingXL * 2
                visible: overlay.errorMessage !== ""
                text: overlay.errorMessage
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeMedium
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.Wrap
            }
        }

        RowLayout {
            id: controls

            enabled: overlay.kind !== "pdf" || overlay.pageView?.ready === true
            anchors.bottom: parent.bottom
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottomMargin: Theme.spacingM
            spacing: Theme.spacingXS

            DActionButton {
                iconName: "zoom_out"
                enabled: overlay.zoom > 0.25
                tooltipText: I18n.tr("Zoom out", "tooltip on the button zooming the large preview out")
                onClicked: overlay.zoomBy(-1)
            }

            StyledText {
                Layout.preferredWidth: Theme.spacingXL * 2
                text: Math.round(overlay.zoom * 100) + "%"
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                horizontalAlignment: Text.AlignHCenter
            }

            DActionButton {
                iconName: "zoom_in"
                enabled: overlay.zoom < 8
                tooltipText: I18n.tr("Zoom in", "tooltip on the button zooming the large preview in")
                onClicked: overlay.zoomBy(1)
            }

            DActionButton {
                iconName: "restart_alt"
                tooltipText: I18n.tr("Reset the view", "tooltip on the button restoring zoom and rotation")
                onClicked: overlay.reset()
            }

            DActionButton {
                iconName: "rotate_left"
                tooltipText: I18n.tr("Rotate left", "tooltip on the button turning the large preview left")
                onClicked: overlay.rotateBy(-90)
            }

            DActionButton {
                iconName: "rotate_right"
                tooltipText: I18n.tr("Rotate right", "tooltip on the button turning the large preview right")
                onClicked: overlay.rotateBy(90)
            }
        }

        Shortcut {
            sequences: ["Escape"]
            enabled: overlay.open
            onActivated: overlay.dismiss()
        }
    }
}
