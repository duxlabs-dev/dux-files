pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser
import "../Common/PreviewView.js" as PreviewView
import "../DCommon/FileBrowser/FilePreview.js" as FilePreview

FocusScope {
    id: overlay

    property var entry: null
    property bool open: false
    property real zoom: 1
    property real angle: 0
    property real panX: 0
    property real panY: 0
    property var backend: DirectoryService
    property Component mediaComponent: null
    readonly property QtObject sound: QtObject {
        property real volume: 1
        property bool muted: false
    }
    property Component highlighterComponent: null

    readonly property string path: entry?.path ?? ""
    readonly property string mime: entry?.mime ?? ""
    readonly property string kind: open && entry ? PreviewView.kindFor(mime, FilePreview.isText(mime)) : ""
    readonly property string shownKind: PreviewView.readsText(kind) && reader.binary ? "generic" : kind
    readonly property bool transforms: PreviewView.transforms(kind)
    readonly property bool showsDetails: errorMessage === "" && (shownKind === "generic" || (kind === "audio" && mediaView !== null))
    readonly property bool turned: PreviewView.normalizeAngle(angle) % 180 !== 0
    readonly property real devicePixelRatio: Window.window?.devicePixelRatio ?? Screen.devicePixelRatio
    readonly property int rasterBucket: Math.min(4, Math.max(1, Math.ceil(zoom)))
    readonly property alias pageView: pages.item
    readonly property alias mediaView: media.item
    readonly property alias textView: reader
    readonly property string errorMessage: {
        if (kind === "pdf" && PdfService.unavailable)
            return I18n.tr("PDF preview is not available. Install Qt PDF support to view this file.", "large preview warning when the optional Qt PDF module is missing");
        if (PreviewView.playsMedia(kind) && mediaComponent?.status === Component.Error)
            return I18n.tr("Video and audio preview is not available. Install Qt Multimedia to play this file.", "large preview warning when the optional Qt Multimedia module is missing");
        if ((kind === "image" && picture.status === Image.Error) || (kind === "pdf" && (pages.status === Loader.Error || pageView?.failed)) || (PreviewView.playsMedia(kind) && mediaView?.failed) || (PreviewView.readsText(kind) && reader.failed))
            return I18n.tr("This file could not be opened", "large preview placeholder");
        return "";
    }

    signal closed
    signal stepRequested(var event)

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
        loadOptional();
        forceActiveFocus();
    }

    function supports(target) {
        return !!target;
    }

    function loadOptional() {
        switch (kind) {
        case "pdf":
            PdfService.ensureProbed();
            break;
        case "video":
        case "audio":
            if (!mediaComponent)
                mediaComponent = Qt.createComponent(Qt.resolvedUrl("PreviewMedia.qml"), Component.PreferSynchronous, overlay);
            break;
        case "text":
            if (!highlighterComponent)
                highlighterComponent = Qt.createComponent(Qt.resolvedUrl("PreviewHighlighter.qml"), Component.PreferSynchronous, overlay);
            break;
        }
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
        event.accepted = true;
        if (event.matches(StandardKey.Copy)) {
            reader.copySelection();
            return;
        }
        if (event.modifiers & (Qt.ControlModifier | Qt.AltModifier))
            return;
        switch (event.key) {
        case Qt.Key_Escape:
            if (pageField.inputFocused) {
                pageField.cancel();
                leavePageField();
                break;
            }
            dismiss();
            break;
        case Qt.Key_Up:
        case Qt.Key_Down:
            if (pageField.inputFocused) {
                pageBy(event.key === Qt.Key_Down ? 1 : -1);
                pageField.show();
                break;
            }
            stepRequested(event);
            break;
        case Qt.Key_Left:
        case Qt.Key_Right:
            stepRequested(event);
            break;
        case Qt.Key_PageUp:
        case Qt.Key_PageDown:
            pageBy(event.key === Qt.Key_PageDown ? 1 : -1);
            break;
        case Qt.Key_Home:
        case Qt.Key_End:
            if (kind === "pdf")
                pageView?.goToPage(event.key === Qt.Key_End ? pageView.pageCount - 1 : 0);
            break;
        default:
            transformKey(event);
        }
    }

    function leavePageField() {
        pageField.setFocus(false);
        forceActiveFocus();
    }

    function pageBy(step) {
        if (kind === "pdf") {
            pageView?.goToPage(pageView.currentPage + step);
            return;
        }
        reader.scrollPage(step);
    }

    function transformKey(event) {
        if (!transforms)
            return;
        switch (event.key) {
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
        }
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
        color: Theme.foregroundColor(Theme.surfaceContainer, true)
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

            Column {
                anchors.left: parent.left
                anchors.right: closeButton.left
                anchors.rightMargin: Theme.spacingM
                anchors.verticalCenter: parent.verticalCenter

                StyledText {
                    width: parent.width
                    text: overlay.entry?.name ?? ""
                    color: Theme.surfaceText
                    font.pixelSize: Theme.fontSizeLarge
                    font.weight: Theme.fontWeightMedium
                    elide: Text.ElideMiddle
                }

                StyledText {
                    width: parent.width
                    visible: text !== ""
                    text: overlay.shownKind === overlay.kind ? reader.note : ""
                    color: Theme.surfaceVariantText
                    font.pixelSize: Theme.fontSizeSmall
                    elide: Text.ElideRight
                }
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
            anchors.bottom: overlay.transforms ? controls.top : parent.bottom
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

            PreviewText {
                id: reader

                anchors.fill: parent
                anchors.margins: Theme.spacingS
                visible: overlay.shownKind === "text" || overlay.shownKind === "markdown"
                path: PreviewView.readsText(overlay.kind) ? overlay.path : ""
                fileName: overlay.entry?.name ?? ""
                markdown: overlay.kind === "markdown"
                highlighter: overlay.highlighterComponent
                backend: overlay.backend
            }

            Column {
                anchors.centerIn: parent
                width: parent.width - Theme.spacingXL * 2
                spacing: Theme.spacingM
                visible: overlay.showsDetails

                FileItemVisual {
                    anchors.horizontalCenter: parent.horizontalCenter
                    iconSize: Math.min(FileBrowserMetrics.previewIconSize, stage.height / 2)
                    iconName: overlay.entry?.iconName ?? ""
                    path: overlay.path
                    thumbnail: overlay.entry?.thumbnail ?? ""
                    mime: overlay.mime
                    isDir: overlay.entry?.isDir ?? false
                    isSymlink: overlay.entry?.isSymlink ?? false
                    symlinkBroken: overlay.entry?.symlinkBroken ?? false
                    untrusted: overlay.entry?.untrusted ?? false
                    unreadable: overlay.entry?.unreadable ?? false
                    hidden: overlay.entry?.hidden ?? false
                }

                StyledText {
                    width: parent.width
                    text: overlay.showsDetails ? ["type", "size", "modified"].map(id => FileColumns.value(id, overlay.entry)).filter(value => value !== "").join(" · ") : ""
                    color: Theme.surfaceVariantText
                    font.pixelSize: Theme.fontSizeMedium
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.Wrap
                }
            }

            Loader {
                id: media

                anchors.fill: parent
                anchors.margins: Theme.spacingS
                active: PreviewView.playsMedia(overlay.kind) && overlay.mediaComponent?.status === Component.Ready
                sourceComponent: active ? overlay.mediaComponent : null
                onLoaded: {
                    item.sound = overlay.sound;
                    item.source = Qt.binding(() => FilePaths.toFileUrl(overlay.path));
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

            visible: overlay.transforms
            enabled: overlay.kind !== "pdf" || overlay.pageView?.ready === true
            anchors.bottom: parent.bottom
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottomMargin: Theme.spacingM
            spacing: Theme.spacingXS

            DActionButton {
                visible: overlay.kind === "pdf"
                iconName: "first_page"
                enabled: (overlay.pageView?.currentPage ?? 0) > 0
                tooltipText: I18n.tr("First page", "tooltip on the button jumping to the first page of a pdf in the large preview")
                onClicked: overlay.pageView.goToPage(0)
            }

            DActionButton {
                visible: overlay.kind === "pdf"
                iconName: "chevron_left"
                enabled: (overlay.pageView?.currentPage ?? 0) > 0
                tooltipText: I18n.tr("Previous page", "tooltip on the button going back one page of a pdf in the large preview")
                onClicked: overlay.pageBy(-1)
            }

            DTextField {
                id: pageField

                readonly property string shownPage: String((overlay.pageView?.currentPage ?? 0) + 1)
                property int startPage: 0

                function show() {
                    text = Qt.binding(() => shownPage);
                }

                function cancel() {
                    overlay.pageView?.goToPage(startPage);
                    show();
                }

                visible: overlay.kind === "pdf"
                Layout.preferredWidth: Theme.spacingXL * 2.5
                text: shownPage
                font.pixelSize: Theme.fontSizeSmall
                validator: RegularExpressionValidator {
                    regularExpression: /[0-9]{1,6}/
                }
                onFocusStateChanged: hasFocus => {
                    if (!hasFocus)
                        return;
                    startPage = overlay.pageView?.currentPage ?? 0;
                    Qt.callLater(selectAll);
                }
                onTextEdited: {
                    if (inputFocused && text !== "" && text !== shownPage)
                        overlay.pageView?.goToPage(parseInt(text, 10) - 1);
                }
                onAccepted: overlay.leavePageField()
                onEditingFinished: show()
            }

            StyledText {
                visible: overlay.kind === "pdf"
                text: `/ ${overlay.pageView?.pageCount ?? 0}`
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
            }

            DActionButton {
                visible: overlay.kind === "pdf"
                iconName: "chevron_right"
                enabled: (overlay.pageView?.currentPage ?? 0) < (overlay.pageView?.pageCount ?? 0) - 1
                tooltipText: I18n.tr("Next page", "tooltip on the button going forward one page of a pdf in the large preview")
                onClicked: overlay.pageBy(1)
            }

            DActionButton {
                visible: overlay.kind === "pdf"
                iconName: "last_page"
                enabled: (overlay.pageView?.currentPage ?? 0) < (overlay.pageView?.pageCount ?? 0) - 1
                tooltipText: I18n.tr("Last page", "tooltip on the button jumping to the last page of a pdf in the large preview")
                onClicked: overlay.pageView.goToPage(overlay.pageView.pageCount - 1)
            }

            Item {
                visible: overlay.kind === "pdf"
                Layout.preferredWidth: Theme.spacingM
            }

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
            sequences: ["Escape", "Space", "F4"]
            enabled: overlay.open && !pageField.inputFocused
            onActivated: overlay.dismiss()
        }
    }
}
