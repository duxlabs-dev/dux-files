pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

Item {
    id: actions

    required property string opScope
    required property var panes
    property bool handlesJobs: false

    readonly property alias trashDialogOpen: trashDialog.visible

    function subjectFor(paths) {
        return paths.length === 1 ? FilePaths.baseName(paths[0]) : FileFormat.count(paths.length);
    }

    function pasteInto(dest, asLink) {
        ClipboardService.read(result => {
            if (result.paths.length === 0) {
                if (!asLink)
                    FileOpsService.pasteImage(dest, I18n.tr("Pasted image", "file name for an image pasted from the clipboard"), actions.opScope);
                return;
            }
            if (asLink) {
                FileOpsService.link(result.paths, dest, actions.opScope);
                return;
            }
            if (result.cut) {
                FileOpsService.move(result.paths, dest, actions.opScope);
                ClipboardService.clear();
                return;
            }
            FileOpsService.copy(result.paths, dest, actions.opScope);
        });
    }

    function trashPaths(paths, subject) {
        if (paths.length === 0)
            return;
        if (!SettingsData.confirmTrash) {
            FileOpsService.trash(paths, opScope);
            return;
        }
        trashDialog.pending = paths;
        trashDialog.subject = subject ?? subjectFor(paths);
        trashDialog.open();
    }

    function copyText(text) {
        Paths.copyPathToClipboard(text);
        ClipboardService.clear();
    }

    function newFolder(dir) {
        if (dir === "")
            return;
        FileOpsService.newFolder(dir, I18n.tr("Untitled folder", "default name for a new folder"), opScope, true);
    }

    function revealCreated(job) {
        const created = job.created?.[0]?.path ?? "";
        if (created === "")
            return;
        const dir = FilePaths.parentOf(created);
        const pane = panes.find(candidate => candidate?.path === dir);
        pane?.revealAndRename(created);
    }

    Connections {
        target: FileOpsService
        enabled: actions.handlesJobs

        function onJobFinished(job) {
            if (job.scope !== actions.opScope)
                return;
            if (job.state === "failed") {
                ToastService.info(FileFormat.operationError(job.code, job.error));
                return;
            }
            if (job.kind === "newFolder" || job.kind === "newFile")
                actions.revealCreated(job);
        }
    }

    ConfirmDialog {
        id: trashDialog

        property var pending: []

        title: FileActions.label("trash")
        message: I18n.tr("%1 will be moved to the trash.", "trash confirmation explanation").arg(subject)
        confirmText: FileActions.label("trash")
        danger: false
        onConfirmed: {
            FileOpsService.trash(pending, actions.opScope);
            pending = [];
        }
    }
}
