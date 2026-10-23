.pragma library

// A look is a tint plus a badge (name, path or web address) or a theme icon.

function normalize(values) {
    const result = {};
    if (values?.color && /^#[0-9a-f]{6}$/i.test(values.color))
        result.color = values.color.toLowerCase();
    if (values?.emblem && /^([A-Za-z0-9._-]+|\/[^\r\n]+|https?:\/\/[^\s]+)$/.test(values.emblem))
        result.emblem = values.emblem;
    if (!result.emblem && values?.icon && /^[A-Za-z0-9._-]+$/.test(values.icon))
        result.icon = values.icon;
    return result;
}

function isEmpty(look) {
    return !look || (!look.color && !look.emblem && !look.icon);
}

function tintFor(look, fallback) {
    return look?.color ?? fallback;
}

function emblemFor(look) {
    return look?.emblem ?? "";
}

function iconFor(look, fallback) {
    return look?.icon ?? fallback;
}
