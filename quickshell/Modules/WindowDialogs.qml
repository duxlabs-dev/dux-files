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

    readonly property bool anyOpen: conflictDialog.visible || errorDialog.visible || archivePasswordDialog.visible || mainWindow.paneActions.trashDialogOpen || confirmLoader.open || openConfirmLoader.open || openWithLoader.open || executableLoader.open || trustLoader.open || unlockLoader.open || connectLoader.open || credentialLoader.open || propertiesLoader.open || appearanceLoader.open || shortcutsLoader.open || compressLoader.open || renameBookmarkLoader.open || emptyTrashLoader.open || clearRecentLoader.open || quitLoader.open || (extractPickerLoader.item?.visible ?? false) || (transferPickerLoader.item?.visible ?? false) || (badgePickerLoader.item?.visible ?? false)

    function dialog(loader) {
        loader.asynchronous = false;
        return loader.item;
    }

    function openSettings() {
        settingsLoader.active = true;
        settingsLoader.item.show();
    }

    function toggleShortcuts() {
        const overlay = dialog(shortcutsLoader);
        if (overlay.visible) {
            overlay.close();
            return;
        }
        overlay.open();
    }

    function renameBookmark(path) {
        const nameDialog = dialog(renameBookmarkLoader);
        nameDialog.path = path;
        nameDialog.ask(BookmarksService.labelFor(path));
    }

    function showProperties(entries) {
        dialog(propertiesLoader).show(entries);
    }

    function showAppearance(entries) {
        const folder = entries.find(entry => entry.isDir);
        if (folder)
            dialog(appearanceLoader).show(folder);
    }

    function showPropertiesFor(paths) {
        const request = ++propertiesRequest;
        DirectoryService.statAll(paths, (entries, failures) => {
            if (request !== dialogs.propertiesRequest)
                return;
            for (const failure of failures)
                ToastService.info(FileFormat.operationError(failure.code, failure.error));
            if (entries.length > 0)
                dialog(propertiesLoader).show(entries);
        });
    }

    function askOpenWith(mimes, paths) {
        dialog(openWithLoader).ask(mimes, paths);
    }

    function askExecutable(path, name) {
        dialog(executableLoader).ask(path, name);
    }

    function confirmTrust(subject, path) {
        const trustDialog = dialog(trustLoader);
        trustDialog.subject = subject;
        trustDialog.pending = path;
        trustDialog.open();
    }

    function confirmOpen(plan, subject) {
        const confirm = dialog(openConfirmLoader);
        confirm.pending = plan;
        confirm.subject = subject;
        confirm.open();
    }

    function confirmDelete(paths, subject) {
        const confirm = dialog(confirmLoader);
        confirm.pending = paths;
        confirm.subject = subject;
        confirm.open();
    }

    function confirmEmptyTrash() {
        if (TrashService.count === 0)
            return;
        const confirm = dialog(emptyTrashLoader);
        confirm.subject = FileFormat.count(TrashService.count);
        confirm.open();
    }

    function confirmClearRecent() {
        dialog(clearRecentLoader).open();
    }

    function confirmQuit() {
        const confirm = dialog(quitLoader);
        confirm.running = FileOpsService.activeCount;
        confirm.open();
    }

    function askCompress(paths) {
        dialog(compressLoader).ask(paths);
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
        dialog(unlockLoader).ask(device);
    }

    function askServer() {
        dialog(connectLoader).ask();
    }

    function presentServer(uri, result) {
        dialog(connectLoader).present(uri, result);
    }

    function connectRemote(uri, password) {
        NetworkService.connectTo(uri, result => {
            const connectDialog = dialog(connectLoader);
            const path = connectDialog.localPath(result);
            if (path === "") {
                connectDialog.present(uri, result);
                return;
            }
            dialogs.mainWindow.openPath(path, true);
        }, password);
    }

    Connections {
        target: NetworkService
        enabled: dialogs.mainWindow.lastActive && ChooserService.count === 0

        function onAskRequested(ask) {
            dialogs.dialog(credentialLoader).present(ask);
        }
    }

    DialogLoader {
        id: propertiesLoader

        PropertiesDialog {
            onRenameRequested: (target, name) => FileOpsService.rename(target, name, dialogs.mainWindow.opScope)
        }
    }

    DialogLoader {
        id: appearanceLoader

        FolderAppearanceDialog {
            onBrowseRequested: {
                badgePickerLoader.active = true;
                badgePickerLoader.item.open();
            }
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
            onAccepted: paths => dialogs.dialog(appearanceLoader).apply({
                    "emblem": paths[0],
                    "icon": ""
                })
        }
    }

    DialogLoader {
        id: shortcutsLoader

        ShortcutsOverlay {}
    }

    DialogLoader {
        id: openWithLoader

        OpenWithDialog {}
    }

    DialogLoader {
        id: executableLoader

        ExecutableDialog {
            onRunRequested: AppsService.run(path, response => dialogs.mainWindow.openFlow.reportError(response))
            onEditRequested: dialogs.dialog(openWithLoader).ask("text/plain", [path])
        }
    }

    DialogLoader {
        id: trustLoader

        ConfirmDialog {
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

    DialogLoader {
        id: renameBookmarkLoader

        NameDialog {
            property string path: ""

            title: I18n.tr("Rename", "rename dialog header")
            acceptText: I18n.tr("Rename", "rename dialog header")
            onSubmitted: name => BookmarksService.rename(path, name)
        }
    }

    DialogLoader {
        id: compressLoader

        CompressDialog {
            onSubmitted: (paths, name, format, password) => FileOpsService.compress(paths, dialogs.mainWindow.currentPath, name, format, password, dialogs.mainWindow.opScope)
        }
    }

    DialogLoader {
        id: openConfirmLoader

        ConfirmDialog {
            property var pending: null

            title: I18n.tr("Open %1 items?", "confirmation before opening many selected items at once").arg(subject)
            message: (pending?.archives?.length ?? 0) > 0 ? I18n.tr("Folders open in new tabs, archives are extracted and other files open in their applications.", "explanation in the confirmation before opening many items, some of them archives") : I18n.tr("Folders open in new tabs and files open in their applications.", "explanation in the confirmation before opening many items")
            confirmText: I18n.tr("Open", "context menu entry opening a file")
            danger: false
            onConfirmed: dialogs.mainWindow.openFlow.runOpenPlan(pending)
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

    DialogLoader {
        id: emptyTrashLoader

        ConfirmDialog {
            title: I18n.tr("Empty the trash?", "confirmation header before deleting everything in the trash")
            message: I18n.tr("%1 will be deleted permanently. This cannot be undone.", "permanent delete confirmation explanation").arg(subject)
            confirmText: I18n.tr("Empty Trash", "action deleting everything in the trash")
            onConfirmed: FileOpsService.emptyTrash()
        }
    }

    DialogLoader {
        id: clearRecentLoader

        ConfirmDialog {
            title: I18n.tr("Clear the recent files list?", "confirmation header before emptying the recently used list")
            message: I18n.tr("The files themselves stay where they are.", "explanation under the clear recent confirmation")
            confirmText: I18n.tr("Clear Recent", "action emptying the recently used list without touching the files")
            danger: false
            onConfirmed: RecentService.clear()
        }
    }

    DialogLoader {
        id: quitLoader

        ConfirmDialog {
            property int running: 0

            title: I18n.tr("Quit while file operations are running?", "confirmation header when closing the last window would stop running file operations")
            message: running === 1 ? I18n.tr("The running operation will be stopped part way through.", "quit confirmation explanation for a single running file operation") : I18n.tr("%1 running operations will be stopped part way through.", "quit confirmation explanation for several running file operations").arg(running)
            confirmText: I18n.tr("Quit", "close behavior button group option to quit the app")
            onConfirmed: dialogs.mainWindow.closeRequested(true)
        }
    }

    DialogLoader {
        id: confirmLoader

        ConfirmDialog {
            property var pending: []

            title: I18n.tr("Delete permanently?", "permanent delete confirmation header")
            message: I18n.tr("%1 will be deleted permanently. This cannot be undone.", "permanent delete confirmation explanation").arg(subject)
            onConfirmed: {
                FileOpsService.deletePermanently(pending);
                pending = [];
            }
        }
    }

    DialogLoader {
        id: unlockLoader

        UnlockDialog {
            onUnlocked: mountPoint => {
                if (mountPoint !== "")
                    dialogs.mainWindow.openPath(mountPoint, false);
            }
        }
    }

    DialogLoader {
        id: connectLoader

        ConnectServerDialog {
            onConnected: path => dialogs.mainWindow.openPath(path, false)
        }
    }

    DialogLoader {
        id: credentialLoader

        CredentialDialog {}
    }
}
