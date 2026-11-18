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
    animationDuration: [250, 250, 250, 250],
    focusRingWidth: [1.5, 1.5, 1.5, 1.5],
    thumbnailSizeLimitMb: [64, 64, 64, 64]
};

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
