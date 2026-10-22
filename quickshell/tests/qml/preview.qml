pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.Modules
import qs.DCommon.Common as DC

ShellRoot {
    id: fixture

    readonly property var log: Log.scoped("fixture")
    property int failures: 0

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

    Component {
        id: pdfComponent

        PreviewPdf {
            width: host.width
            height: host.height
        }
    }

    Component.onCompleted: {
        DC.Style.theme = Theme;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;

        const view = overlayComponent.createObject(host);
        check("the overlay starts closed", view.open, false);

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

        view.present({
            "path": "/nonexistent/scan.pdf",
            "name": "scan.pdf",
            "mime": "application/pdf"
        });
        check("a pdf opens the page view", [view.open, view.kind, view.pageView !== null], [true, "pdf", true]);

        view.dismiss();
        check("dismissing closes it and forgets the entry", [view.open, view.entry], [false, null]);

        const pdf = pdfComponent.createObject(fixture);
        check("the pdf page view builds", pdf !== null, true);
        pdf.zoom = 2;
        check("the pdf view keeps its zoom", pdf.zoom, 2);
        pdf.destroy();

        view.destroy();
        finish();
    }
}
