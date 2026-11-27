.pragma library

var LEVELS = ["spacious", "comfortable", "compact", "dense"];

// Indexed in LEVELS order. Comfortable must match the JsonAdapter defaults in SettingsData.
var DEFAULTS = {
    fileNameSize: [15, 14, 13, 12],
    radiusStrength: [50, 50, 50, 50],
    fontWeight: [400, 400, 400, 400],
    fontScale: [1.0, 1.0, 1.0, 1.0],
    floatingWindowTransparency: [1.0, 1.0, 1.0, 1.0],
    floatingWindowForegroundTransparency: [1.0, 1.0, 1.0, 1.0],
    motionDuration: [2, 2, 2, 2],
    focusRingWidth: [1.5, 1.5, 1.5, 1.5],
    thumbnailSizeLimitMb: [64, 64, 64, 64]
};

var MOTION_STEP_MS = [250, 375, 500, 750, 1000];

function motionStep(durationMs) {
    if (durationMs <= 0)
        return -1;
    return MOTION_STEP_MS.reduce((nearest, ms, step) => Math.abs(ms - durationMs) < Math.abs(MOTION_STEP_MS[nearest] - durationMs) ? step : nearest, 0);
}

function has(key) {
    return key in DEFAULTS;
}

function value(key, density) {
    var index = LEVELS.indexOf(density);
    return DEFAULTS[key]?.[index >= 0 ? index : LEVELS.indexOf("comfortable")];
}

// Only settings still on the outgoing density's default move; anything else is user-set.
function transition(settings, from, to) {
    var out = {};
    if (from === to)
        return out;
    for (var key in DEFAULTS) {
        if (settings[key] === value(key, from))
            out[key] = value(key, to);
    }
    return out;
}
