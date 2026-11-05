pragma ComponentBehavior: Bound

import QtQuick
import qs.Services
import qs.DCommon.FileBrowser

QtObject {
    id: root

    required property string method
    property var affects: job => false
    property bool finishedOnly: false
    property var _waiting: []

    signal check
    signal reconnected
    signal affected
    signal fetched(var result)

    function refresh() {
        _delay.restart();
    }

    function fetch(params, callback) {
        _waiting.push(callback);
        if (_waiting.length > 1)
            return;
        DuxFilesService.call(method, params, result => {
            root.fetched(result);
            const waiting = root._waiting;
            root._waiting = [];
            for (const pending of waiting)
                pending(result);
        });
    }

    readonly property Timer _delay: Timer {
        interval: FileBrowserMetrics.settleInterval
        onTriggered: root.check()
    }

    readonly property Connections _link: Connections {
        target: DuxFilesService

        function onConnectedChanged() {
            root.reconnected();
        }
    }

    readonly property Connections _jobs: Connections {
        target: FileOpsService

        function onJobFinished(job) {
            if (root.finishedOnly && job.state !== "done")
                return;
            if (root.affects(job))
                root.affected();
        }
    }
}
