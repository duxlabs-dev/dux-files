import assert from "node:assert/strict";
import test from "node:test";
import vm from "node:vm";
import { source } from "./qml-script.mjs";

const scope = vm.createContext({});
vm.runInContext(source("../Common/FolderAppearance.js").replace(/^\.pragma.*$/gm, ""), scope);
const look = vm.runInContext("({normalize, isEmpty, tintFor, emblemFor, iconFor})", scope);

test("only well formed values survive a round trip", () => {
    assert.deepEqual({ ...look.normalize({ color: "#448AFF", icon: "folder-red" }) },
        { color: "#448aff", icon: "folder-red" });
});

test("a badge and an icon never share a folder", () => {
    assert.deepEqual({ ...look.normalize({ color: "#448aff", emblem: "steam", icon: "folder-red" }) },
        { color: "#448aff", emblem: "steam" });
});

test("values that would leak into a file path are dropped", () => {
    assert.deepEqual({ ...look.normalize({ color: "blue", emblem: "../../etc/passwd", icon: "folder red" }) }, {});
    assert.deepEqual({ ...look.normalize(null) }, {});
});

test("an empty look is empty", () => {
    assert.equal(look.isEmpty(null), true);
    assert.equal(look.isEmpty({}), true);
    assert.equal(look.isEmpty({ color: "#448aff" }), false);
});

test("the look wins over the default and falls back when absent", () => {
    assert.equal(look.tintFor({ color: "#448aff" }, "#123456"), "#448aff");
    assert.equal(look.tintFor(null, "#123456"), "#123456");
    assert.equal(look.emblemFor({ emblem: "steam" }), "steam");
    assert.equal(look.emblemFor(null), "");
    assert.equal(look.iconFor({ icon: "folder-red" }, "folder"), "folder-red");
    assert.equal(look.iconFor({}, "folder"), "folder");
});

test("a badge can be a theme name, a local path or a web address", () => {
    for (const emblem of ["steam", "/home/me/Pictures/logo.svg", "https://example.com/logo.png"])
        assert.equal(look.normalize({ emblem }).emblem, emblem);
    assert.deepEqual({ ...look.normalize({ emblem: "javascript:alert(1)" }) }, {});
    assert.deepEqual({ ...look.normalize({ emblem: "ftp://host/logo.png" }) }, {});
});
