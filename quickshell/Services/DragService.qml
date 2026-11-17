pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Singleton {
    id: root

    property var transport: DuxFilesService
    property var paths: []
    property bool active: false
    property int generation: 0
    property var plainDrops: ({})

    readonly property string arkServiceMime: "application/x-kde-ark-dndextract-service"
    readonly property string arkPathMime: "application/x-kde-ark-dndextract-path"

    function hasArkDrop(drop) {
        const formats = drop.formats || [];
        let service = false;
        let path = false;
        for (let i = 0; i < formats.length; i++) {
            service = service || formats[i] === arkServiceMime;
            path = path || formats[i] === arkPathMime;
        }
        return service && path;
    }

    function extractArkDrop(drop, dest) {
        if (!hasArkDrop(drop) || dest === "" || dest === TrashService.filesPath || dest === RecentService.path)
            return false;
        transport.call("files.dndExtract", {
            "service": drop.getDataAsString(arkServiceMime),
            "path": drop.getDataAsString(arkPathMime),
            "dest": dest
        }, result => {
            if (result.error)
                ToastService.info(FileFormat.operationError(result.code, result.error));
        });
        return true;
    }

    readonly property string hoverDest: active ? FileDragHover.dest : ""
    readonly property string action: {
        if (hoverDest === "" || paths.includes(hoverDest))
            return "";
        if (hoverDest === TrashService.filesPath)
            return paths.every(path => TrashService.holds(path)) ? "" : "trash";
        const resolved = resolve(overrideFor(FileDragHover.proposedAction), plainDrops[hoverDest] ?? "");
        if (resolved === "move" && allInside(paths, hoverDest))
            return "";
        return resolved;
    }

    onHoverDestChanged: {
        if (hoverDest === "" || hoverDest === TrashService.filesPath || plainDrops[hoverDest] !== undefined)
            return;
        const dest = hoverDest;
        const drag = generation;
        plainDrop(paths, dest, resolved => {
            if (drag !== root.generation)
                return;
            const known = Object.assign({}, root.plainDrops);
            known[dest] = resolved;
            root.plainDrops = known;
        });
    }

    function begin(selection) {
        paths = selection.slice();
        plainDrops = ({});
        generation++;
        active = true;
    }

    function end() {
        paths = [];
        plainDrops = ({});
        generation++;
        active = false;
    }

    function mimeData(portalKey) {
        const data = {
            "text/uri-list": paths.map(path => FilePaths.toFileUrl(path)).join("\r\n") + "\r\n",
            "x-special/gnome-copied-files": "copy\n" + paths.map(path => FilePaths.toFileUrl(path)).join("\n") + "\n"
        };
        if (portalKey)
            data["application/vnd.portal.filetransfer"] = portalKey;
        return data;
    }

    function exportPortal(sources, callback) {
        transport.call("files.op.portalTransfer", {
            "paths": sources
        }, result => callback(result.key ?? ""));
    }

    function pathsFrom(drop) {
        if (active && paths.length > 0)
            return paths.slice();
        return localPaths(drop);
    }

    function localPaths(drop) {
        if (!drop.hasUrls)
            return [];
        return drop.urls.map(url => url.toString()).filter(text => text.startsWith("file://")).map(text => FilePaths.fromUrl(text));
    }

    function allInside(sources, dir) {
        for (const path of sources) {
            if (FilePaths.parentOf(path) !== dir)
                return false;
        }
        return true;
    }

    function overrideFor(proposedAction) {
        if (!active)
            return "";
        switch (proposedAction) {
        case Qt.CopyAction:
            return "copy";
        case Qt.LinkAction:
            return "ask";
        default:
            return "";
        }
    }

    function resolve(forced, plain) {
        if (forced !== "")
            return forced;
        if (SettingsData.dragAction !== "auto")
            return SettingsData.dragAction;
        return plain;
    }

    function plainDrop(sources, dest, callback) {
        const known = active ? plainDrops[dest] : undefined;
        if (known !== undefined) {
            callback(known);
            return;
        }
        transport.call("files.op.dropAction", {
            "paths": sources,
            "dest": dest
        }, result => callback(result.action === "move" ? "move" : "copy"));
    }
}
