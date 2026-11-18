import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

QtObject {
    id: flow

    required property var mainWindow

    property string pendingTrust: ""
    property var pendingExtracts: ({})

    readonly property var activePane: mainWindow.activePane
    readonly property var primaryPane: mainWindow.panes[0]
    readonly property var secondPane: mainWindow.panes[1] ?? null
    readonly property string opScope: mainWindow.opScope
    readonly property bool lastActive: mainWindow.lastActive
    readonly property var dialogs: mainWindow.dialogs

    function reportError(response) {
        if (response.error)
            ToastService.info(FileFormat.operationError(response.code, response.error));
    }

    function activate(entry) {
        if (!entry)
            return;
        if (activePane.trashActive) {
            offerRestore([entry]);
            return;
        }
        if (entry.isDir) {
            activePane.navigate(entry.path);
            return;
        }
        const canRun = SandboxService.available("run");
        if (entry.untrusted && canRun) {
            dialogs.confirmTrust(entry.name, entry.path);
            return;
        }
        if (canRun && FilesFormat.isRunnable(entry.mime, entry.isExecutable)) {
            dialogs.askExecutable(entry.path, entry.name);
            return;
        }
        if (SettingsData.extractOnOpen && FilesFormat.extractable(entry.mime)) {
            extractBeside(entry.path, "enter");
            return;
        }
        AppsService.open([entry.path], response => flow.reportError(response));
    }

    function trustAndRun(path) {
        pendingTrust = path;
        FileOpsService.setPermissions([path], "755", false);
    }

    function openEntries(entries) {
        if (entries.length === 0)
            return;
        if (activePane.trashActive) {
            offerRestore(entries);
            return;
        }
        if (entries.length === 1) {
            activate(entries[0]);
            return;
        }
        const plan = openPlan(entries, SettingsData.extractOnOpen);
        if (openCount(plan) <= FilesMetrics.openConfirmLimit) {
            runOpenPlan(plan);
            return;
        }
        dialogs.confirmOpen(plan, String(entries.length));
    }

    function openPlan(entries, extractArchives) {
        const folders = entries.filter(entry => entry.isDir).map(entry => entry.path);
        const files = entries.filter(entry => !entry.isDir);
        const unpacks = entry => extractArchives && FilesFormat.extractable(entry.mime);
        return {
            "folder": folders[0] ?? "",
            "tabs": folders.slice(1),
            "files": files.filter(entry => !unpacks(entry)).map(entry => entry.path),
            "archives": files.filter(entry => unpacks(entry)).map(entry => entry.path)
        };
    }

    function openCount(plan) {
        return plan.tabs.length + plan.files.length + plan.archives.length;
    }

    function restoreEntries(entries) {
        const byDir = ({});
        for (const entry of entries) {
            if (!entry.trashDir)
                continue;
            byDir[entry.trashDir] = (byDir[entry.trashDir] ?? []).concat([entry.trashName]);
        }
        for (const dir in byDir)
            FileOpsService.restore(byDir[dir], dir, opScope);
    }

    function runOpenPlan(plan) {
        for (const path of plan.tabs)
            mainWindow.tabManager.newTab(path, true);
        if (plan.files.length > 0)
            AppsService.open(plan.files, response => flow.reportError(response));
        for (const path of plan.archives)
            extractBeside(path, "reveal");
        if (plan.folder !== "")
            activePane.navigate(plan.folder);
    }

    function extractBeside(path, follow) {
        extractTo([path], FilePaths.parentOf(path), follow);
    }

    function extractTo(paths, dest, follow) {
        const pending = pendingExtracts;
        for (const path of paths)
            pending[path] = follow;
        pendingExtracts = pending;
        FileOpsService.extract(paths, dest, opScope);
    }

    function takeExtractFollow(sources) {
        const pending = pendingExtracts;
        let follow = "reveal";
        for (const source of sources) {
            if (pending[source])
                follow = pending[source];
            delete pending[source];
        }
        pendingExtracts = pending;
        return follow;
    }

    function handleExtracted(job) {
        const follow = takeExtractFollow(job.sources);
        const created = job.created ?? [];
        if (job.cancelled || created.length === 0)
            return;
        const paths = created.map(entry => entry.path);
        const dest = FilePaths.parentOf(paths[0]);
        const stillHere = dest === activePane.path && !activePane.resultsActive;
        if (stillHere && follow === "enter" && created.length === 1 && created[0].dir) {
            activePane.navigate(paths[0]);
            undoToast(I18n.tr("Extracted", "toast after unpacking an archive"));
            return;
        }
        if (stillHere) {
            activePane.reveal(paths);
            undoToast(I18n.tr("Extracted", "toast after unpacking an archive"));
            return;
        }
        ToastService.show(I18n.tr("Extracted to %1", "toast after unpacking into another folder, %1 is the folder name").arg(FilePaths.baseName(dest)), {
            "actionLabel": I18n.tr("Show", "toast action revealing the extracted files"),
            "action": () => flow.activePane.reveal(paths)
        });
    }

    function undoToast(message) {
        ToastService.show(message, {
            "actionLabel": I18n.tr("Undo", "toast action reversing the last operation"),
            "action": () => FileOpsService.undo(flow.opScope)
        });
    }

    function handleReplayed(job) {
        if (job.cancelled)
            return;
        if (job.replay === "redo") {
            if (job.undoable)
                undoToast(I18n.tr("Redone", "toast after repeating an undone operation"));
            return;
        }
        const message = I18n.tr("Undone", "toast after reversing an operation");
        if (!job.redoable) {
            ToastService.info(message);
            return;
        }
        ToastService.show(message, {
            "actionLabel": I18n.tr("Redo", "toast action repeating the operation that was just undone"),
            "action": () => FileOpsService.redo(flow.opScope)
        });
    }

    function revealNew(job) {
        const paths = (job.created ?? []).map(entry => entry.path);
        if (paths.length === 0)
            return;
        const dir = FilePaths.parentOf(paths[0]);
        const pane = [activePane, primaryPane, secondPane].find(candidate => candidate?.path === dir && !candidate.resultsActive);
        pane?.reveal(paths);
    }

    function offerRestore(entries) {
        const items = entries.map(entry => ({
                    "trashDir": entry.trashDir,
                    "trashName": entry.trashName
                }));
        ToastService.show(I18n.tr("Items in the trash cannot be opened", "toast when opening an item inside the trash"), {
            "actionLabel": I18n.tr("Restore", "trash action returning items to where they were deleted from"),
            "action": () => flow.restoreEntries(items)
        });
    }

    readonly property Connections jobs: Connections {
        target: FileOpsService

        function onJobFinished(job) {
            if (job.kind === "setPermissions" && flow.pendingTrust !== "" && (job.sources ?? []).includes(flow.pendingTrust)) {
                const target = flow.pendingTrust;
                flow.pendingTrust = "";
                if (job.state !== "failed")
                    AppsService.run(target, response => flow.reportError(response));
            }
            if (job.state === "failed") {
                if (job.kind === "extract")
                    flow.takeExtractFollow(job.sources);
                if (job.scope === flow.opScope || (job.scope === "" && flow.lastActive))
                    ToastService.info(FileFormat.operationError(job.code, job.error));
                return;
            }
            DevicesService.refreshUsage(true);
            if (job.scope !== flow.opScope)
                return;
            if (job.replay) {
                flow.handleReplayed(job);
                return;
            }
            switch (job.kind) {
            case "extract":
                flow.handleExtracted(job);
                return;
            case "newFolder":
            case "newFile":
            case "newFolderWith":
                mainWindow.paneActions.revealCreated(job);
                return;
            case "duplicate":
            case "pasteImage":
                flow.revealNew(job);
                break;
            }
            if (job.cancelled || !job.undoable)
                return;
            const message = JobLabels.done(job.kind);
            if (message === "")
                return;
            flow.undoToast(message);
        }
    }
}
