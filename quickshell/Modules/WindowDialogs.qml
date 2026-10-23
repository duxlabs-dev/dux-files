import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser
import qs.Modules.Settings

Item {
    id: dialogs

    required property var mainWindow

    property int propertiesRequest: 0

    readonly property bool anyOpen: conflictDialog.visible || errorDialog.visible || archivePasswordDialog.visible || confirmDialog.visible || openConfirmDialog.visible || openWithDialog.visible || executableDialog.visible || trustDialog.visible || unlockDialog.visible || connectDialog.visible || credentialDialog.visible || propertiesDialog.visible || appearanceDialog.visible || shortcutsOverlay.visible || compressDialog.visible || renameBookmarkDialog.visible || trashConfirmDialog.visible || emptyTrashDialog.visible || clearRecentDialog.visible || (extractPickerLoader.item?.visible ?? false) || (transferPickerLoader.item?.visible ?? false)

    function openSettings() {
        settingsLoader.active = true;
        settingsLoader.item.show();
    }

    function toggleShortcuts() {
        if (shortcutsOverlay.visible) {
            shortcutsOverlay.close();
            return;
        }
        shortcutsOverlay.open();
    }

    function renameBookmark(path) {
        renameBookmarkDialog.path = path;
        renameBookmarkDialog.ask(BookmarksService.labelFor(path));
    }

    function showProperties(entries) {
        propertiesDialog.show(entries);
    }

    function showAppearance(entries) {
        const folder = entries.find(entry => entry.isDir);
        if (folder)
            appearanceDialog.show(folder);
    }

    function showPropertiesFor(paths) {
        const request = ++propertiesRequest;
        const entries = [];
        let pending = paths.length;
        paths.forEach((path, index) => DuxFilesService.sendRequest("files.stat", {
                "path": path
            }, response => {
                if (request !== dialogs.propertiesRequest)
                    return;
                pending--;
                if (response.error)
                    ToastService.info(FileFormat.operationError(response.code, response.error));
                entries[index] = (response.result || {}).entry ?? null;
                if (pending > 0)
                    return;
                const found = entries.filter(entry => entry);
                if (found.length > 0)
                    propertiesDialog.show(found);
            }));
    }

    function askOpenWith(mimes, paths) {
        openWithDialog.ask(mimes, paths);
    }

    function askExecutable(path, name) {
        executableDialog.ask(path, name);
    }

    function confirmTrust(subject, path) {
        trustDialog.subject = subject;
        trustDialog.pending = path;
        trustDialog.open();
    }

    function confirmOpen(plan, subject) {
        openConfirmDialog.pending = plan;
        openConfirmDialog.subject = subject;
        openConfirmDialog.open();
    }

    function confirmTrash(paths, subject) {
        trashConfirmDialog.pending = paths;
        trashConfirmDialog.subject = subject;
        trashConfirmDialog.open();
    }

    function confirmDelete(paths, subject) {
        confirmDialog.pending = paths;
        confirmDialog.subject = subject;
        confirmDialog.open();
    }

    function confirmEmptyTrash() {
        if (TrashService.count === 0)
            return;
        emptyTrashDialog.subject = FileFormat.count(TrashService.count);
        emptyTrashDialog.open();
    }

    function confirmClearRecent() {
        clearRecentDialog.open();
    }

    function askCompress(paths) {
        compressDialog.ask(paths);
    }

    function askExtractTo(paths) {
        if (paths.length === 0)
            return;
        extractPickerLoader.active = true;
        extractPickerLoader.item.archives = paths;
        extractPickerLoader.item.open();
    }

    function askTransferTo(paths, move) {
        if (paths.length === 0)
            return;
        transferPickerLoader.active = true;
        transferPickerLoader.item.paths = paths;
        transferPickerLoader.item.move = move;
        transferPickerLoader.item.open();
    }

    function transferTo(dest) {
        const picker = transferPickerLoader.item;
        FileOpsService.lastDestination = dest;
        if (picker.move) {
            FileOpsService.move(picker.paths, dest, mainWindow.opScope);
            return;
        }
        FileOpsService.copy(picker.paths, dest, mainWindow.opScope);
    }

    function askUnlock(device) {
        unlockDialog.ask(device);
    }

    function askServer() {
        connectDialog.ask();
    }

    function showServerFailure(uri, result) {
        connectDialog.showFailure(uri, result);
    }

    function connectRemote(uri, password) {
        NetworkService.connectTo(uri, result => {
            const path = connectDialog.localPath(result);
            if (path === "") {
                connectDialog.showFailure(uri, result);
                return;
            }
            dialogs.mainWindow.openPath(path, true);
        }, password);
    }

    Connections {
        target: NetworkService

        function onAskRequested(ask) {
            credentialDialog.present(ask);
        }
    }

    PropertiesDialog {
        id: propertiesDialog

        onRenameRequested: (target, name) => FileOpsService.rename(target, name, dialogs.mainWindow.opScope)
    }

    FolderAppearanceDialog {
        id: appearanceDialog

        onBrowseRequested: {
            badgePickerLoader.active = true;
            badgePickerLoader.item.open();
        }
    }

    LazyLoader {
        id: badgePickerLoader

        active: false

        FileBrowserModal {
            parentModal: dialogs.mainWindow
            browserTitle: I18n.tr("Choose a badge image", "file picker title for a folder badge image")
            bucket: "badge"
            filters: ["*.svg", "*.png", "*.jpg", "*.jpeg", "*.webp"]
            onAccepted: paths => appearanceDialog.apply({
                    "emblem": paths[0],
                    "icon": ""
                })
        }
    }

    ShortcutsOverlay {
        id: shortcutsOverlay
    }

    OpenWithDialog {
        id: openWithDialog
    }

    ExecutableDialog {
        id: executableDialog

        onRunRequested: AppsService.run(path, response => dialogs.mainWindow.openFlow.reportError(response))
        onEditRequested: openWithDialog.ask("text/plain", [path])
    }

    ConfirmDialog {
        id: trustDialog

        property string pending: ""

        title: I18n.tr("Run this desktop file?", "untrusted desktop file dialog header")
        message: I18n.tr("%1 has not been marked as trusted. Running it allows it to start any program.", "untrusted desktop file dialog explanation").arg(subject)
        confirmText: I18n.tr("Trust and run", "untrusted desktop file dialog action")
        danger: false
        onConfirmed: {
            dialogs.mainWindow.openFlow.trustAndRun(pending);
            pending = "";
        }
    }

    ConflictDialog {
        id: conflictDialog

        opScope: dialogs.mainWindow.opScope
        catchUnscoped: dialogs.mainWindow.lastActive
    }

    OperationErrorDialog {
        id: errorDialog

        opScope: dialogs.mainWindow.opScope
        catchUnscoped: dialogs.mainWindow.lastActive
    }

    ArchivePasswordDialog {
        id: archivePasswordDialog

        opScope: dialogs.mainWindow.opScope
        catchUnscoped: dialogs.mainWindow.lastActive
    }

    NameDialog {
        id: renameBookmarkDialog

        property string path: ""

        title: I18n.tr("Rename", "rename dialog header")
        acceptText: I18n.tr("Rename", "rename dialog header")
        onSubmitted: name => BookmarksService.rename(path, name)
    }

    CompressDialog {
        id: compressDialog

        onSubmitted: (paths, name, format, password) => FileOpsService.compress(paths, dialogs.mainWindow.currentPath, name, format, password, dialogs.mainWindow.opScope)
    }

    ConfirmDialog {
        id: openConfirmDialog

        property var pending: null

        title: I18n.tr("Open %1 items?", "confirmation before opening many selected items at once").arg(subject)
        message: (pending?.archives?.length ?? 0) > 0 ? I18n.tr("Folders open in new tabs, archives are extracted and other files open in their applications.", "explanation in the confirmation before opening many items, some of them archives") : I18n.tr("Folders open in new tabs and files open in their applications.", "explanation in the confirmation before opening many items")
        confirmText: I18n.tr("Open", "context menu entry opening a file")
        danger: false
        onConfirmed: dialogs.mainWindow.openFlow.runOpenPlan(pending)
    }

    ConfirmDialog {
        id: trashConfirmDialog

        property var pending: []

        title: I18n.tr("Move to Trash", "context menu entry trashing the selection")
        message: I18n.tr("%1 will be moved to the trash.", "trash confirmation explanation").arg(subject)
        confirmText: I18n.tr("Move to Trash", "context menu entry trashing the selection")
        danger: false
        onConfirmed: {
            FileOpsService.trash(pending, dialogs.mainWindow.opScope);
            pending = [];
        }
    }

    LazyLoader {
        id: extractPickerLoader

        active: false

        FileBrowserModal {
            property var archives: []

            parentModal: dialogs.mainWindow
            browserTitle: I18n.tr("Extract to", "folder picker title for unpacking archives")
            bucket: "extract"
            mode: "openFolder"
            revealPath: dialogs.mainWindow.currentPath
            onAccepted: paths => dialogs.mainWindow.openFlow.extractTo(archives, paths[0], "reveal")
        }
    }

    LazyLoader {
        id: transferPickerLoader

        active: false

        FileBrowserModal {
            property var paths: []
            property bool move: false

            parentModal: dialogs.mainWindow
            browserTitle: move ? I18n.tr("Move to", "folder picker title for moving the selection") : I18n.tr("Copy to", "folder picker title for copying the selection")
            bucket: "transfer"
            mode: "openFolder"
            revealPath: FileOpsService.lastDestination !== "" ? FileOpsService.lastDestination : dialogs.mainWindow.currentPath
            onAccepted: picked => dialogs.transferTo(picked[0])
        }
    }

    Loader {
        id: settingsLoader

        active: false
        sourceComponent: SettingsModal {
            parentWindow: dialogs.mainWindow
        }
    }

    ConfirmDialog {
        id: emptyTrashDialog

        title: I18n.tr("Empty the trash?", "confirmation header before deleting everything in the trash")
        message: I18n.tr("%1 will be deleted permanently. This cannot be undone.", "permanent delete confirmation explanation").arg(subject)
        confirmText: I18n.tr("Empty Trash", "action deleting everything in the trash")
        onConfirmed: FileOpsService.emptyTrash()
    }

    ConfirmDialog {
        id: clearRecentDialog

        title: I18n.tr("Clear the recent files list?", "confirmation header before emptying the recently used list")
        message: I18n.tr("The files themselves stay where they are.", "explanation under the clear recent confirmation")
        confirmText: I18n.tr("Clear Recent", "action emptying the recently used list without touching the files")
        danger: false
        onConfirmed: RecentService.clear()
    }

    ConfirmDialog {
        id: confirmDialog

        property var pending: []

        title: I18n.tr("Delete permanently?", "permanent delete confirmation header")
        message: I18n.tr("%1 will be deleted permanently. This cannot be undone.", "permanent delete confirmation explanation").arg(subject)
        onConfirmed: {
            FileOpsService.deletePermanently(pending);
            pending = [];
        }
    }

    UnlockDialog {
        id: unlockDialog

        onUnlocked: mountPoint => {
            if (mountPoint !== "")
                dialogs.mainWindow.openPath(mountPoint, false);
        }
    }

    ConnectServerDialog {
        id: connectDialog

        onConnected: path => dialogs.mainWindow.openPath(path, false)
    }

    CredentialDialog {
        id: credentialDialog
    }
}
