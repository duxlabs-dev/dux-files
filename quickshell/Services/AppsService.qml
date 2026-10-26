pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Common
import qs.Services

Singleton {
    id: root

    property var transport: DuxFilesService

    function open(paths, callback) {
        const params = {
            "paths": paths
        };
        if (SettingsData.opener !== "")
            params.opener = SettingsData.opener;
        _launch("files.open", params, callback);
    }

    function openWith(paths, desktopId, callback) {
        _launch("files.openWith", {
            "paths": paths,
            "desktopId": desktopId
        }, callback);
    }

    function run(path, callback) {
        _send("files.run", _launchParams({
            "path": path
        }), callback);
    }

    function openTerminal(path, callback) {
        _send("files.openTerminal", _launchParams({
            "path": path
        }), callback);
    }

    function appsForMime(mime, path, callback) {
        const params = {};
        if (mime)
            params.mime = mime;
        if (path)
            params.path = path;
        _send("files.apps.forMime", params, callback);
    }

    function sections(mimes, paths, callback) {
        const params = {};
        if (mimes && mimes.length > 0)
            params.mimes = mimes;
        else
            params.paths = paths;
        _send("files.apps.sections", params, callback);
    }

    function allApps(callback) {
        _send("files.apps.all", {}, callback);
    }

    function setDefault(mime, desktopId, callback) {
        _send("files.apps.setDefault", {
            "mime": mime,
            "desktopId": desktopId
        }, callback);
    }

    function _launchParams(params) {
        if (SettingsData.terminal !== "")
            params.terminal = SettingsData.terminal;
        return params;
    }

    function _launch(method, params, callback) {
        _send(method, _launchParams(params), result => {
            if (!result.error)
                RecentService.refresh();
            callback?.(result);
        });
    }

    function _send(method, params, callback) {
        transport.call(method, params, callback);
    }
}
