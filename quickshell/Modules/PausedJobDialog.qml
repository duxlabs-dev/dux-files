import QtQuick
import qs.Services

OverlayDialog {
    id: root

    property string opScope: ""
    property bool catchUnscoped: false
    property string pausedState: ""
    property string promptKey: ""
    property bool applyToAll: false

    readonly property var paused: FileOpsService.jobs.filter(job => job.state === pausedState && (job.scope === opScope || (catchUnscoped && job.scope === "")))
    readonly property var current: paused.length > 0 ? paused[0] : null
    readonly property var prompt: current ? current[promptKey] : null

    function answer(action) {
        if (!current || !prompt)
            return;
        FileOpsService.respond(current.id, prompt.id, action, applyToAll);
        applyToAll = false;
    }

    function offers(action) {
        return prompt ? (prompt.actions || []).includes(action) : false;
    }

    closeEnabled: false
    cancelAction: () => root.answer("cancel")
    onPromptChanged: prompt ? open() : close()
}
