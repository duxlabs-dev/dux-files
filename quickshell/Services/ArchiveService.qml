pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell

Singleton {
    id: root

    property var opened: ({})

    readonly property string watchPrefix: "archive:"
    readonly property var containerSuffixes: [".zip", ".7z", ".rar", ".cpio", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tbz", ".tb2", ".tar.xz", ".txz", ".tar.zst", ".tzst", ".tar.lzma", ".tlz", ".tar.lz"]

    function rootFromWatch(watchId) {
        if (!watchId.startsWith(watchPrefix))
            return "";
        const rest = watchId.substring(watchPrefix.length);
        const cut = rest.indexOf(":");
        const length = Number(rest.substring(0, cut));
        if (cut <= 0 || !(length > 0))
            return "";
        return rest.substring(cut + 1, cut + 1 + length);
    }

    function holds(archive, path) {
        return path === archive || path.startsWith(archive + "/");
    }

    function rootByName(path) {
        for (const archive in opened) {
            if (holds(archive, path))
                return archive;
        }
        const parts = path.split("/");
        for (let i = 1; i < parts.length; i++) {
            const name = parts[i].toLowerCase();
            if (containerSuffixes.some(suffix => name.length > suffix.length && name.endsWith(suffix)))
                return parts.slice(0, i + 1).join("/");
        }
        return "";
    }

    function member(archive, path) {
        return path.substring(archive.length + 1);
    }

    function adopt(archive) {
        if (opened[archive])
            return;
        const next = Object.assign({}, opened);
        next[archive] = true;
        opened = next;
    }
}
