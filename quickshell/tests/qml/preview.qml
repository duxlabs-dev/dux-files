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

    property var nextPreview: ({})
    property int leakedKeys: 0
    property var lastPreview: null

    readonly property QtObject fakeBackend: QtObject {
        function preview(path, maxBytes, callback) {
            fixture.lastPreview = {
                "path": path,
                "maxBytes": maxBytes
            };
            callback(fixture.nextPreview);
        }
    }

    FloatingWindow {
        implicitWidth: 800
        implicitHeight: 600
        visible: true

        Item {
            id: host

            anchors.fill: parent
            Keys.onPressed: fixture.leakedKeys++
        }
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

        view.present(null);
        check("nothing selected does not open it", view.open, false);

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

        view.backend = fakeBackend;
        fixture.nextPreview = {
            "text": "package main\n",
            "binary": false,
            "truncated": true,
            "size": 1048576
        };
        view.present({
            "path": "/nonexistent/main.go",
            "name": "main.go",
            "mime": "text/x-go"
        });
        check("a code file shows its text", [view.kind, view.shownKind, view.textView.failed], ["text", "text", false]);
        check("the code file asks for the size cap", fixture.lastPreview, {
            "path": "/nonexistent/main.go",
            "maxBytes": 262144
        });
        check("a cut text file says so", view.textView.note !== "", true);

        fixture.nextPreview = {
            "text": "",
            "binary": true,
            "size": 10
        };
        view.present({
            "path": "/nonexistent/data.json",
            "name": "data.json",
            "extension": "json",
            "size": 10,
            "mtimeMs": 0,
            "mime": "application/json"
        });
        check("binary content falls back to the file details", [view.kind, view.shownKind], ["text", "generic"]);

        fixture.nextPreview = {
            "error": "permission denied"
        };
        view.present({
            "path": "/nonexistent/notes.md",
            "name": "notes.md",
            "mime": "text/markdown"
        });
        check("an unreadable text file reports it", [view.kind, view.errorMessage], ["markdown", "This file could not be opened"]);

        view.present({
            "path": "/nonexistent/archive.zip",
            "name": "archive.zip",
            "extension": "zip",
            "size": 2048,
            "mtimeMs": 0,
            "mime": "application/zip"
        });
        check("other files open on their details", [view.open, view.kind, view.errorMessage], [true, "generic", ""]);
        view.dismiss();

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
                compare(view.pageView.pageCount, 1);
                view.pageBy(1);
                view.pageView.goToPage(5);
                compare(view.pageView.currentPage, 0);
                checkPageField(view);
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

        function test_space_closes_after_a_control_took_focus() {
            const view = fixture.view;
            view.present({
                "path": FilePaths.fromUrl(Qt.resolvedUrl("preview.png")),
                "name": "preview.png",
                "mime": "image/png"
            });
            const zoomIn = fixture.findItem(view, item => item.iconName === "zoom_in");
            zoomIn.forceActiveFocus();
            compare(zoomIn.activeFocus, true);
            keyClick(Qt.Key_Space);
            fixture.check("space closes the preview even when a control has focus", [view.open, view.zoom], [false, 1]);
        }

        function test_sound_survives_other_files() {
            const view = fixture.view;
            const clip = {
                "path": "/nonexistent/clip.mp4",
                "name": "clip.mp4",
                "mime": "video/mp4"
            };
            view.present(clip);
            if (view.mediaComponent.status !== Component.Ready) {
                view.dismiss();
                return;
            }
            view.sound.volume = 0.3;
            view.sound.muted = true;
            view.present({
                "path": FilePaths.fromUrl(Qt.resolvedUrl("preview.png")),
                "name": "preview.png",
                "mime": "image/png"
            });
            fixture.check("a picture between videos unloads the player", view.mediaView, null);
            view.present(clip);
            const volume = fixture.findItem(view.mediaView, item => item.maximum === 100 && item.sliderValueChanged !== undefined);
            fixture.check("the next video keeps the volume and mute", [view.mediaView.sound.volume, view.mediaView.sound.muted, volume.value], [0.3, true, 30]);
            view.dismiss();
        }

        function checkPageField(view) {
            view.present({
                "path": FilePaths.fromUrl(Qt.resolvedUrl("pages.pdf")),
                "name": "pages.pdf",
                "mime": "application/pdf"
            });
            tryCompare(view.pageView, "ready", true);
            compare(view.pageView.pageCount, 3);
            const field = fixture.findItem(view, item => item.validator !== undefined && item.inputFocused !== undefined);
            verify(field !== null);
            const jump = (key, page) => {
                field.forceActiveFocus();
                tryCompare(field, "inputFocused", true);
                wait(0);
                keyClick(key);
                tryCompare(view.pageView, "currentPage", page);
                keyClick(Qt.Key_Return);
                compare(field.inputFocused, false);
                compare(field.text, String(page + 1));
            };
            fixture.leakedKeys = 0;
            jump(Qt.Key_3, 2);
            jump(Qt.Key_1, 0);
            jump(Qt.Key_9, 2);
            field.forceActiveFocus();
            tryCompare(field, "inputFocused", true);
            wait(0);
            keyClick(Qt.Key_1);
            tryCompare(view.pageView, "currentPage", 0);
            keyClick(Qt.Key_Escape);
            tryCompare(view.pageView, "currentPage", 2);
            compare([view.open, field.inputFocused, field.text], [true, false, "3"]);
            fixture.check("enter in the page field stays inside the preview", fixture.leakedKeys, 0);
            view.present({
                "path": FilePaths.fromUrl(Qt.resolvedUrl("preview.pdf")),
                "name": "preview.pdf",
                "mime": "application/pdf"
            });
            tryCompare(view.pageView, "ready", true);
        }

        function cleanupTestCase() {
            if (!fixture.pdfChecked)
                fixture.failures++;
            fixture.view.destroy();
            fixture.finish();
        }
    }
}
