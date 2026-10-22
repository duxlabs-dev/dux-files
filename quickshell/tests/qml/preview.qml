pragma ComponentBehavior: Bound

import QtQuick
import QtTest
import Quickshell
import qs.Common
import qs.Services
import qs.Modules
import qs.DCommon.Common as DC
import qs.DCommon.FileBrowser

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0
    property var view: null
    property bool ready: false
    property bool pdfChecked: false

    function check(label, actual, expected) {
        const ok = JSON.stringify(actual) === JSON.stringify(expected);
        if (!ok) {
            failures++;
            log.error(`FAIL ${label}: got ${JSON.stringify(actual)} want ${JSON.stringify(expected)}`);
            return;
        }
        log.info(`ok ${label}`);
    }

    function finish() {
        if (failures === 0)
            log.info("FIXTURE PASS");
        Qt.callLater(Qt.quit);
    }

    Item {
        id: host

        width: 800
        height: 600
    }

    Component {
        id: overlayComponent

        PreviewOverlay {
            width: host.width
            height: host.height
        }
    }

    Component.onCompleted: {
        DC.Style.theme = Theme;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;

        const view = overlayComponent.createObject(host);
        fixture.view = view;
        check("the overlay starts closed", view.open, false);
        check("pdf availability is unknown before opening a pdf", [PdfService.available, PdfService.unavailable], [false, false]);

        view.present({
            "path": "/nonexistent/photo.png",
            "name": "photo.png",
            "mime": "text/plain"
        });
        check("an unsupported format does not open it", view.open, false);

        view.present({
            "path": "/nonexistent/photo.png",
            "name": "photo.png",
            "mime": "image/png"
        });
        check("an image opens it", [view.open, view.kind], [true, "image"]);
        check("the view starts unzoomed and upright", [view.zoom, view.angle, view.panX, view.panY], [1, 0, 0, 0]);

        view.zoomBy(1);
        check("zooming in steps up", view.zoom, 1.25);
        view.zoomBy(-1);
        check("zooming out steps back", view.zoom, 1);

        for (let step = 0; step < 20; step++)
            view.zoomBy(1);
        check("zooming in stops at the ceiling", view.zoom, 8);
        for (let step = 0; step < 40; step++)
            view.zoomBy(-1);
        check("zooming out stops at the floor", view.zoom, 0.25);

        view.rotateBy(90);
        check("rotating right turns a quarter", view.angle, 90);
        view.rotateBy(-180);
        check("rotating left turns back and past zero", view.angle, 270);
        view.rotateBy(90);
        check("a full turn normalizes to zero", view.angle, 0);

        view.reset();
        view.zoomBy(1);
        view.panTo(100000, 100000);
        check("panning stops at the image edge", [view.panX > 0, view.panY > 0], [true, true]);
        view.panTo(-100000, -100000);
        check("panning stops at the other edge", [view.panX < 0, view.panY < 0], [true, true]);

        view.reset();
        check("resetting drops zoom, angle and pan", [view.zoom, view.angle, view.panX, view.panY], [1, 0, 0, 0]);

        fixture.ready = true;
    }

    function findItem(item, predicate) {
        for (const child of item.children ?? []) {
            if (predicate(child))
                return child;
            const match = findItem(child, predicate);
            if (match)
                return match;
        }
        return null;
    }

    TestCase {
        name: "PdfPreview"
        when: fixture.ready

        function test_optional_pdf() {
            const view = fixture.view;
            view.present({
                "path": FilePaths.fromUrl(Qt.resolvedUrl("preview.pdf")),
                "name": "preview.pdf",
                "mime": "application/pdf"
            });
            compare(view.open, true);
            compare(view.kind, "pdf");

            if (PdfService.available) {
                verify(view.pageView !== null);
                tryCompare(view.pageView, "ready", true);
                compare(view.errorMessage, "");
                view.zoom = 2;
                compare(view.pageView.zoom, 2);
                view.angle = 90;
                compare(view.pageView.angle, 90);

                view.present({
                    "path": FilePaths.fromUrl(Qt.resolvedUrl("missing.pdf")),
                    "name": "missing.pdf",
                    "mime": "application/pdf"
                });
                tryCompare(view.pageView, "failed", true);
                compare(view.errorMessage, "This file could not be opened");
            } else {
                compare(PdfService.unavailable, true);
                compare(view.pageView, null);
                compare(view.errorMessage, "PDF preview is not available. Install Qt PDF support to view this file.");
            }

            const label = fixture.findItem(view, item => item.text === view.errorMessage);
            verify(label !== null);
            compare(label.visible, true);
            view.dismiss();
            compare(view.open, false);
            compare(view.entry, null);
            compare(view.errorMessage, "");
            view.present({"path": FilePaths.fromUrl(Qt.resolvedUrl("preview.png")), "name": "preview.png", "mime": "image/png"});
            compare(view.kind, "image");
            compare(view.open, true);
            compare(view.pageView, null);
            const image = fixture.findItem(view, item => item.source !== undefined && item.paintedWidth !== undefined);
            verify(image !== null);
            tryCompare(image, "status", Image.Ready);
            compare(view.errorMessage, "");
            fixture.pdfChecked = true;
        }

        function cleanupTestCase() {
            if (!fixture.pdfChecked)
                fixture.failures++;
            fixture.view.destroy();
            fixture.finish();
        }
    }
}
