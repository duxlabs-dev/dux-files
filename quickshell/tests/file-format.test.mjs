import assert from "node:assert/strict";
import test from "node:test";
import vm from "node:vm";
import { lift, source } from "./qml-script.mjs";

const fileFormat = source("../DCommon/FileBrowser/FileFormat.qml");
const filesFormat = source("../Common/FilesFormat.qml");

function context() {
    const ctx = vm.createContext({ I18n: { tr: term => term } });
    for (const name of ["size", "operationError"])
        lift(fileFormat, name, ctx);
    for (const name of ["elevationUnavailable", "featureUnavailable", "isRunnable", "extractable"])
        lift(filesFormat, name, ctx);
    return ctx;
}

test("size scales by 1000 and hides negative sizes", () => {
    const ctx = context();
    assert.equal(ctx.size(-1), "");
    assert.equal(ctx.size(0), "0 B");
    assert.equal(ctx.size(999), "999 B");
    assert.equal(ctx.size(1000), "1.0 kB");
    assert.equal(ctx.size(1536000), "1.5 MB");
    assert.equal(ctx.size(2.5e12), "2.5 TB");
    assert.equal(ctx.size(9e15), "9000.0 TB");
});

test("operationError keeps the daemon message only for unmapped codes", () => {
    const ctx = context();
    assert.equal(ctx.operationError("NOTSUPPORTED", "raw"), "raw");
    assert.equal(ctx.operationError("", "raw"), "raw");
    assert.equal(ctx.operationError("", ""), "This item could not be handled");
    assert.equal(ctx.operationError("EACCES", "raw"), "You do not have permission to do this");
    assert.equal(ctx.operationError("SANDBOXED", "raw"), "This item could not be handled");
});

test("every sandbox reason code has its own explanation", () => {
    const ctx = context();
    const codes = ["SANDBOXED", "NOHOSTFS", "NOUDISKS", "NOGVFS", "NOHOSTSPAWN", "PORTALCHOOSER"];
    const texts = codes.map(code => ctx.featureUnavailable(code));
    assert.equal(new Set(texts).size, codes.length);
    assert.equal(ctx.featureUnavailable("SOMETHING_NEW"), "Not available");
    assert.notEqual(ctx.elevationUnavailable("SANDBOXED"), ctx.featureUnavailable("SANDBOXED"));
});

test("isRunnable needs the execute bit and a program-like type", () => {
    const ctx = context();
    assert.equal(ctx.isRunnable("application/x-executable", false), false);
    assert.equal(ctx.isRunnable("application/x-executable", true), true);
    assert.equal(ctx.isRunnable("application/x-shellscript", true), true);
    assert.equal(ctx.isRunnable("text/x-python", true), true);
    assert.equal(ctx.isRunnable("", true), true);
    assert.equal(ctx.isRunnable("image/png", true), false);
});

test("extractable mirrors the formats the daemon unpacks", () => {
    const ctx = context();
    for (const mime of ["application/zip", "application/x-compressed-tar", "application/x-xz-compressed-tar", "application/x-zstd-compressed-tar", "application/x-7z-compressed", "application/vnd.rar", "application/x-cpio", "application/gzip", "application/x-lzip", "application/x-lzip-compressed-tar"])
        assert.equal(ctx.extractable(mime), true, mime);
    for (const mime of ["application/x-iso9660-image", "application/x-lzip-compressed-zip", "application/pdf", ""])
        assert.equal(ctx.extractable(mime), false, mime);
});

function timestampContext() {
    const ctx = vm.createContext({ I18n: { tr: term => term }, Qt: { locale: () => "C" }, Locale: { ShortFormat: 0 } });
    vm.runInContext(`String.prototype.arg = function (value) {
        const lowest = Math.min(...[...this.matchAll(/%(\\d)/g)].map(match => Number(match[1])));
        return this.split("%" + lowest).join(String(value));
    };
    Date.prototype.toLocaleTimeString = function () { return "T"; };
    Date.prototype.toLocaleDateString = function (locale, format) { return format === "dddd" ? "Weekday" : "D"; };
    Date.prototype.toLocaleString = function () { return "D T"; };`, ctx);
    for (const name of ["timestamp", "daysBefore", "relativeTimestamp"])
        lift(fileFormat, name, ctx);
    return ctx;
}

const at = (y, m, d, h = 12) => `new Date(${y}, ${m - 1}, ${d}, ${h}, 30)`;

test("the relative format buckets by calendar day, not by elapsed hours", () => {
    const ctx = timestampContext();
    const now = at(2026, 10, 29, 0);
    const rel = date => vm.runInContext(`relativeTimestamp(${date}, ${now})`, ctx);
    assert.equal(rel(at(2026, 10, 29, 23)), "Today, T");
    assert.equal(rel(at(2026, 10, 28, 23)), "Yesterday, T");
    assert.equal(rel(at(2026, 10, 27)), "Weekday, T");
    assert.equal(rel(at(2026, 10, 23)), "Weekday, T");
    assert.equal(rel(at(2026, 10, 22)), "D");
    assert.equal(rel(at(2026, 10, 30)), "D");
});

test("daysBefore survives a month boundary and a daylight saving shift", () => {
    const ctx = timestampContext();
    assert.equal(vm.runInContext(`daysBefore(${at(2026, 9, 30)}, ${at(2026, 10, 1)})`, ctx), 1);
    assert.equal(vm.runInContext(`daysBefore(${at(2026, 10, 25)}, ${at(2026, 11, 1)})`, ctx), 7);
});

test("unknown formats fall back to the locale string", () => {
    const ctx = timestampContext();
    assert.equal(vm.runInContext(`timestamp(${at(2026, 10, 29)}, "locale", ${at(2026, 10, 29)})`, ctx), "D T");
    assert.equal(vm.runInContext(`timestamp(${at(2026, 10, 29)}, "", ${at(2026, 10, 29)})`, ctx), "D T");
    assert.equal(vm.runInContext(`timestamp(${at(2026, 10, 29)}, "relative", ${at(2026, 10, 29)})`, ctx), "Today, T");
});

function hintContext() {
    const ctx = vm.createContext({ I18n: { tr: term => term } });
    vm.runInContext(`String.prototype.arg = function (value) {
        const lowest = Math.min(...[...this.matchAll(/%(\\d)/g)].map(match => Number(match[1])));
        return this.split("%" + lowest).join(String(value));
    };`, ctx);
    lift(fileFormat, "size", ctx);
    vm.runInContext("var FileFormat = { size: size };", ctx);
    for (const name of ["timeLeft", "transferHint"])
        lift(filesFormat, name, ctx);
    return ctx;
}

const progress = (done, total, rate, estimating = false) => ({ bytesDone: done, bytesTotal: total, bytesPerSec: rate, estimating });

test("the transfer hint waits for a known total and a measured rate", () => {
    const ctx = hintContext();
    assert.equal(ctx.transferHint(progress(0, 1e9, 0)), "");
    assert.equal(ctx.transferHint(progress(0, 0, 12e6)), "");
    assert.equal(ctx.transferHint(progress(0, 1e9, 12e6, true)), "");
    assert.equal(ctx.transferHint(null), "");
});

test("the transfer hint shows speed and time left", () => {
    const ctx = hintContext();
    assert.equal(ctx.transferHint(progress(0, 120e6, 12e6)), "12.0 MB/s · 10 s left");
    assert.equal(ctx.transferHint(progress(0, 1440e6, 12e6)), "12.0 MB/s · 2 min left");
    assert.equal(ctx.transferHint(progress(119e6, 120e6, 12e6)), "12.0 MB/s · 1 s left");
    assert.equal(ctx.transferHint(progress(0, 90000e6, 12e6)), "12.0 MB/s · 2 h 5 min left");
});
