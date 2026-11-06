import assert from "node:assert/strict";
import test from "node:test";
import vm from "node:vm";
import { source } from "./qml-script.mjs";

const ctx = vm.createContext({});
vm.runInContext(source("../Common/PreviewView.js").replace(/^\.pragma.*$/gm, ""), ctx);
const view = vm.runInContext("({kindFor, clampZoom, steppedZoom, normalizeAngle, panLimit, clampPan, clock, localMarkdown})", ctx);

test("each file type picks its viewer", () => {
    assert.equal(view.kindFor("application/pdf", false), "pdf");
    assert.equal(view.kindFor("image/png", false), "image");
    assert.equal(view.kindFor("video/mp4", false), "video");
    assert.equal(view.kindFor("audio/flac", false), "audio");
    assert.equal(view.kindFor("text/markdown", true), "markdown");
    assert.equal(view.kindFor("text/x-go", true), "text");
    assert.equal(view.kindFor("application/x-ruby", true), "text");
});

test("anything else falls back to the file details", () => {
    assert.equal(view.kindFor("application/zip", false), "generic");
    assert.equal(view.kindFor("inode/directory", false), "generic");
    assert.equal(view.kindFor("", false), "generic");
    assert.equal(view.kindFor(null, false), "generic");
});

test("playback times read like a player clock", () => {
    assert.equal(view.clock(0), "0:00");
    assert.equal(view.clock(65400), "1:05");
    assert.equal(view.clock(3725000), "1:02:05");
    assert.equal(view.clock(-5), "0:00");
});

test("zoom stays inside the rendering window", () => {
    assert.equal(view.clampZoom(100), 8);
    assert.equal(view.clampZoom(0.01), 0.25);
    assert.equal(view.clampZoom(2), 2);
});

test("stepping up and down is symmetric and clamped", () => {
    assert.equal(view.steppedZoom(2, 1), 2.5);
    assert.equal(view.steppedZoom(2, -1), 1.6);
    assert.equal(view.steppedZoom(8, 1), 8);
    assert.equal(view.steppedZoom(0.25, -1), 0.25);
});

test("a full turn returns to zero", () => {
    assert.equal(view.normalizeAngle(360), 0);
    assert.equal(view.normalizeAngle(-90), 270);
    assert.equal(view.normalizeAngle(450), 90);
});

test("a quarter turn swaps the axis that can pan", () => {
    assert.equal(view.panLimit(1000, 500, 2, 0), 500);
    assert.equal(view.panLimit(1000, 500, 1, 0), 0);
    assert.equal(view.panLimit(1000, 500, 2, 90), 0);
    assert.equal(view.panLimit(1000, 500, 1, 90), 0);
    assert.equal(view.panLimit(1000, 500, 1, 180), 0);
});

test("panning never leaves the image", () => {
    assert.equal(view.clampPan(900, 500), 500);
    assert.equal(view.clampPan(-900, 500), -500);
    assert.equal(view.clampPan(120, 500), 120);
});

test("html images become markdown images Qt can lay out", () => {
    assert.equal(view.localMarkdown('Text <img src=x.png alt="x"> more'), "Text ![x](<x.png>) more");
    assert.equal(view.localMarkdown('<div align="center">\n  <img src="logo.svg" alt="Logo" width="160">\n\n### Tagline\n</div>'), "\n\n![Logo](<logo.svg>)\n\n\n### Tagline\n");
});

test("remote images fall back to their alt text", () => {
    assert.equal(view.localMarkdown('<img src="https://x.test/a.png" alt="remote">'), "\nremote\n");
    assert.equal(view.localMarkdown("<img src='//x.test/a.png' alt='relative'>"), "\nrelative\n");
    assert.equal(view.localMarkdown("[![CI](https://x.test/b.svg)](https://ci.test)"), "[CI](https://ci.test)");
    assert.equal(view.localMarkdown("![pixel][p]\n\n[p]: https://x.test/p.png"), "pixel\n\n[p]: https://x.test/p.png");
    assert.equal(view.localMarkdown('![local](pic.png "title")'), '![local](pic.png "title")');
});

test("html cannot pull in remote backgrounds or stylesheets", () => {
    assert.equal(view.localMarkdown('<table background="https://x.test/t.png"><tr><td style="background-image: url(https://x.test/c.png)">cell</td></tr></table>'), '<table><tr><td style="background-image: none">cell</td></tr></table>');
    assert.equal(view.localMarkdown("<style>\n@import url(https://x.test/s.css);\n</style>\nAfter"), "\n\n\nAfter");
    assert.equal(view.localMarkdown('<link rel="stylesheet" href="https://x.test/s.css">'), "");
});

test("code blocks keep their html", () => {
    const fence = '```html\n<img src="a.png">\n```';
    assert.equal(view.localMarkdown(fence), fence);
});
