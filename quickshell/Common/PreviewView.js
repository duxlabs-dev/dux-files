.pragma library

const minZoom = 0.25;
const maxZoom = 8;
const zoomStep = 1.25;

function kindFor(mime) {
    if (!mime)
        return "";
    switch (mime) {
    case "application/pdf":
        return "pdf";
    }
    return mime.startsWith("image/") ? "image" : "";
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
