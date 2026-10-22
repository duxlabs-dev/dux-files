pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Pdf

Item {
    id: root

    property url source: ""
    property real zoom: 1
    property real angle: 0
    property real fitScale: 1

    readonly property int status: document.status
    readonly property bool ready: document.status === PdfDocument.Ready
    readonly property bool failed: document.status === PdfDocument.Error

    function applyView() {
        if (document.status !== PdfDocument.Ready || width < 2 || height < 2)
            return;
        view.scaleToPage(width, height);
        fitScale = view.renderScale;
        view.renderScale = fitScale * zoom;
        view.pageRotation = angle;
    }

    onZoomChanged: applyView()
    onAngleChanged: applyView()
    onWidthChanged: applyView()
    onHeightChanged: applyView()

    PdfDocument {
        id: document

        source: root.source
    }

    PdfMultiPageView {
        id: view

        anchors.fill: parent
        document: document
        focus: false
        activeFocusOnTab: false
    }

    Connections {
        target: document

        function onStatusChanged() {
            root.applyView();
        }
    }
}
