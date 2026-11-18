pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.Services

Singleton {
    id: root

    readonly property var log: Log.scoped("ChooserService")
    readonly property ListModel requests: ListModel {}
    readonly property int count: requests.count
    property var byId: ({})

    function indexOf(id) {
        for (let i = 0; i < requests.count; i++) {
            if (requests.get(i).requestId === id)
                return i;
        }
        return -1;
    }

    function requestFor(id) {
        return byId[id] ?? null;
    }

    function open(request) {
        if (!request || typeof request.id !== "string" || request.id === "")
            return;
        if (indexOf(request.id) >= 0)
            return;
        const all = byId;
        all[request.id] = request;
        byId = all;
        requests.append({
            "requestId": request.id
        });
        log.debug(`chooser ${request.method} opened, ${requests.count} open`);
    }

    function close(id) {
        const index = indexOf(id);
        if (index < 0)
            return;
        requests.remove(index);
        const all = byId;
        delete all[id];
        byId = all;
        log.debug(`chooser closed, ${requests.count} open`);
    }

    function answer(id, answer) {
        DuxFilesService.call("portal.respond", Object.assign({
            "id": id
        }, answer), result => {
            if (result.error)
                root.log.warn("portal.respond failed", result.error);
        });
        close(id);
    }
}
