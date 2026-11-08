import assert from "node:assert/strict";
import test from "node:test";
import vm from "node:vm";
import { source } from "./qml-script.mjs";

const ctx = vm.createContext({});
vm.runInContext(source("../Common/PreviewView.js").replace(/^\.pragma.*$/gm, ""), ctx);
const view = vm.runInContext("({kindFor, clampZoom, steppedZoom, normalizeAngle, panLimit, clampPan})", ctx);

test("only pdfs and images get a large view", () => {
    assert.equal(view.kindFor("application/pdf"), "pdf");
    assert.equal(view.kindFor("image/png"), "image");
    assert.equal(view.kindFor("text/plain"), "");
    assert.equal(view.kindFor(""), "");
    assert.equal(view.kindFor(null), "");
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
