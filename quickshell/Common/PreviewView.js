.pragma library

const minZoom = 0.25;
const maxZoom = 8;
const zoomStep = 1.25;

function kindFor(mime, textual) {
    switch (mime ?? "") {
    case "":
    case "inode/directory":
        return "generic";
    case "application/pdf":
        return "pdf";
    case "text/markdown":
    case "text/x-markdown":
        return "markdown";
    }
    switch (mime.split("/")[0]) {
    case "image":
        return "image";
    case "video":
        return "video";
    case "audio":
        return "audio";
    }
    return textual ? "text" : "generic";
}

function readsText(kind) {
    return kind === "text" || kind === "markdown";
}

function playsMedia(kind) {
    return kind === "video" || kind === "audio";
}

function transforms(kind) {
    return kind === "image" || kind === "pdf";
}

function clampZoom(zoom) {
    return Math.min(maxZoom, Math.max(minZoom, zoom));
}

function steppedZoom(zoom, direction) {
    return clampZoom(direction > 0 ? zoom * zoomStep : zoom / zoomStep);
}

function normalizeAngle(angle) {
    return ((angle % 360) + 360) % 360;
}

function panLimit(extent, crossExtent, zoom, angle) {
    const turned = normalizeAngle(angle) % 180 !== 0;
    return Math.max(0, ((turned ? crossExtent : extent) * zoom - extent) / 2);
}

function clampPan(value, limit) {
    return Math.min(limit, Math.max(-limit, value));
}

function clock(ms) {
    const total = Math.max(0, Math.floor(ms / 1000));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor(total / 60) % 60;
    const seconds = String(total % 60).padStart(2, "0");
    return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${seconds}` : `${minutes}:${seconds}`;
}

function attribute(tag, name) {
    const match = tag.match(new RegExp(`\\s${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`, "i"));
    return match ? match[1] ?? match[2] ?? match[3] : "";
}

function isRemote(src) {
    return /^([a-z][a-z0-9+.-]*:|\/\/)/i.test(src) && !/^(file|data):/i.test(src);
}

function imageMarkdown(alt, src) {
    return src === "" || isRemote(src) ? alt : `![${alt}](<${src}>)`;
}

function rewriteImages(line) {
    const rewritten = line.replace(/<img\b[^>]*>/gi, tag => imageMarkdown(attribute(tag, "alt"), attribute(tag, "src"))).replace(/!\[([^\]]*)\]\(\s*<?([^\s)>]+)>?(?:\s+"[^"]*")?\s*\)/g, (whole, alt, src) => isRemote(src) ? alt : whole).replace(/!\[([^\]]*)\](\[[^\]]*\])?(?!\()/g, "$1");
    return /^\s*<img\b[^>]*>\s*$/i.test(line) ? `\n${rewritten.trim()}\n` : rewritten;
}

function stripBackgrounds(line) {
    return line.replace(/<[a-z][^>]*>/gi, tag => tag.replace(/\sbackground\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, "").replace(/url\([^)]*\)/gi, "none"));
}

function localMarkdown(text) {
    let fenced = false;
    let styled = false;
    return text.split("\n").map(line => {
        if (/^\s*(```|~~~)/.test(line)) {
            fenced = !fenced;
            return line;
        }
        if (fenced)
            return line;
        if (styled || /<style\b/i.test(line)) {
            styled = !/<\/style>/i.test(line);
            return "";
        }
        return /^\s*<\/?(div|p|center|span|picture|source|br|link)\b[^>]*>\s*$/i.test(line) ? "" : rewriteImages(stripBackgrounds(line));
    }).join("\n");
}
