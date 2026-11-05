import assert from "node:assert/strict";
import test from "node:test";
import { source } from "./qml-script.mjs";

// The keyboard map is one table: it binds the keys and fills the help overlay.
// These checks are what stops it growing a duplicate binding or a dead entry.
const qml = source("../Common/Shortcuts.qml");

const table = qml.slice(qml.indexOf("readonly property var entries:"), qml.indexOf("readonly property var groups:"));
const entryPattern = /\{\s*"id":\s*"([^"]+)",\s*"keys":\s*\[([^\]]*)\],\s*"group":\s*"([^"]+)",\s*"scope":\s*"([^"]+)",\s*"label":\s*(?:I18n\.tr\("([^"]*)",\s*"([^"]*)"\)|FileActions\.label\("([^"]+)"\))(?:,\s*"vim":\s*\[([^\]]*)\])?(?:,\s*"plainOnly":\s*true)?\s*\}/g;

const keyList = text => [...text.matchAll(/"([^"]*)"/g)].map(key => key[1]);

// Shared actions keep their label in the FileActions catalog.
const catalog = source("../Common/FileActions.qml");
const catalogLabels = new Map([...catalog.matchAll(/"(\w+)": \{\s*"icon": "[^"]*",\s*"label": I18n\.tr\("([^"]*)",\s*"([^"]*)"\)/g)].map(match => [match[1], { label: match[2], context: match[3] }]));

const entries = [...table.matchAll(entryPattern)].map(match => ({
    id: match[1],
    keys: keyList(match[2]),
    group: match[3],
    scope: match[4],
    label: match[5] ?? catalogLabels.get(match[7])?.label ?? "",
    context: match[6] ?? catalogLabels.get(match[7])?.context ?? "",
    vim: match[8] === undefined ? [] : keyList(match[8]),
    plainOnly: match[0].includes('"plainOnly"')
}));

const groups = [...qml.slice(qml.indexOf("readonly property var groups:")).matchAll(/"id":\s*"([^"]+)"/g)].map(match => match[1]);

const boundScopes = ["window", "selection", "path", "tabs"];
const handledScopes = ["view", "sidebar"];

test("every entry in the table parses", () => {
    assert.equal(entries.length, table.split('"id": "').length - 1);
    assert.ok(entries.length > 50);
});

test("no key is bound twice inside the main window", () => {
    const seen = new Map();
    for (const entry of entries.filter(entry => entry.scope !== "sidebar")) {
        for (const key of entry.keys) {
            assert.ok(!seen.has(key), `${key} is bound by both ${seen.get(key)} and ${entry.id}`);
            seen.set(key, entry.id);
        }
    }
});

test("no key is bound twice inside the sidebar", () => {
    const seen = new Map();
    for (const entry of entries.filter(entry => entry.scope === "sidebar")) {
        for (const key of entry.keys) {
            assert.ok(!seen.has(key), `${key} is bound by both ${seen.get(key)} and ${entry.id}`);
            seen.set(key, entry.id);
        }
    }
});

test("no vim letter stands for two actions", () => {
    const seen = new Map();
    for (const entry of entries.filter(entry => entry.scope !== "sidebar")) {
        for (const key of entry.vim) {
            assert.ok(!seen.has(key), `${key} is bound by both ${seen.get(key)} and ${entry.id}`);
            seen.set(key, entry.id);
        }
    }
});

test("vim letters never shadow a plain binding", () => {
    const plain = new Set(entries.flatMap(entry => entry.keys));
    for (const entry of entries)
        for (const key of entry.vim)
            assert.ok(!plain.has(key), `${entry.id}: ${key}`);
});

test("every id is unique", () => {
    const ids = entries.map(entry => entry.id);
    assert.equal(new Set(ids).size, ids.length);
});

test("every scope is one MainWindow binds or a focused view handles", () => {
    for (const entry of entries)
        assert.ok(boundScopes.concat(handledScopes).includes(entry.scope), `${entry.id}: ${entry.scope}`);
});

test("rows type-ahead replaces are the only plain-only ones", () => {
    assert.deepEqual(entries.filter(entry => entry.plainOnly).map(entry => entry.id), ["typeAhead"]);
});

test("every entry lands in a group the overlay renders", () => {
    assert.ok(groups.length > 0);
    for (const entry of entries)
        assert.ok(groups.includes(entry.group), `${entry.id}: ${entry.group}`);
});

test("sidebar keys live in the sidebar group and nowhere else", () => {
    for (const entry of entries)
        assert.equal(entry.scope === "sidebar", entry.group === "sidebar", entry.id);
});

test("every entry has keys and a translated label with a context", () => {
    for (const entry of entries) {
        assert.ok(entry.keys.length > 0, entry.id);
        assert.ok(entry.keys.every(key => key.length > 0), entry.id);
        assert.ok(entry.label.length > 0, entry.id);
        assert.ok(entry.context.length > 0, `${entry.id} has no translator context`);
    }
});
