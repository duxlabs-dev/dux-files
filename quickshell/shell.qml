//@ pragma Env QT_WAYLAND_DISABLE_WINDOWDECORATION=1
//@ pragma Env QSG_RENDER_LOOP=threaded
//@ pragma Env QT_QUICK_CONTROLS_STYLE=Material
//@ pragma UseQApplication
//@ pragma AppId dev.duxlabs.files

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import qs.Common
import qs.DCommon.Common as DC
import qs.Modules
import qs.Services

ShellRoot {
    id: root

    readonly property var log: Log.scoped("Shell")
    readonly property bool enableHotReload: Quickshell.env("DFILES_ENABLE_HOTRELOAD") === "1" || Quickshell.env("DFILES_ENABLE_HOTRELOAD") === "true"

    readonly property bool dbusActivated: Quickshell.env("DFILES_DBUS_ACTIVATED") === "1"
    readonly property bool keepRunning: Quickshell.env("DFILES_KEEP_RUNNING") === "1"
    readonly property bool quitOnLastClose: SettingsData.closeBehavior === "quit" && !keepRunning
    readonly property bool idle: WindowService.count === 0 && ChooserService.count === 0
    property bool quitDeferred: false

    Component.onCompleted: {
        Quickshell.watchFiles = enableHotReload;
        DC.Style.theme = Theme;
        DC.Style.settings = SettingsData;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;
        DC.Log.backend = Log;
        DC.Host.session = SessionService;
        DC.Host.cache = CacheData;
        DC.Host.files = DirectoryService;
        DC.Host.appearance = FolderAppearances;
        DC.Host.ownWindowDecorations = Qt.binding(() => SettingsData.windowTitleBars);
        if (Quickshell.env("DFILES_START_HIDDEN") !== "1")
            WindowService.create([]);
    }

    function windowObject(windowId) {
        for (let i = 0; i < windowInstances.count; i++) {
            const instance = windowInstances.objectAt(i) as MainWindow;
            if (instance && instance.windowId === windowId)
                return instance;
        }
        return null;
    }

    function ownToplevels() {
        const toplevels = ToplevelManager.toplevels;
        if (!toplevels || !toplevels.values)
            return [];
        return toplevels.values.filter(t => t.appId === "dev.duxlabs.files");
    }

    function unclaimedToplevel() {
        const claimed = [];
        for (let i = 0; i < windowInstances.count; i++) {
            const instance = windowInstances.objectAt(i) as MainWindow;
            if (instance?.toplevel)
                claimed.push(instance.toplevel);
        }
        return ownToplevels().find(t => !claimed.includes(t)) ?? null;
    }

    function focusWindow(windowId) {
        const instance = windowObject(windowId);
        const t = instance?.toplevel ?? unclaimedToplevel();
        if (!t)
            return;
        if (t.minimized)
            t.minimized = false;
        t.activate();
    }

    function showAndFocus() {
        const target = WindowService.target();
        if (target === 0) {
            WindowService.create([]);
            return;
        }
        focusWindow(target);
    }

    function anyWindowVisible() {
        const target = windowObject(WindowService.target());
        if (!target)
            return false;
        const t = target.toplevel ?? unclaimedToplevel();
        return !t || !t.minimized;
    }

    function handleWindowAction(action) {
        switch (action) {
        case "show":
            showAndFocus();
            break;
        case "hide":
            WindowService.closeAll();
            break;
        case "toggle":
            if (anyWindowVisible()) {
                WindowService.closeAll();
                return;
            }
            showAndFocus();
            break;
        }
    }

    function closeWindow(windowId) {
        if (WindowService.count <= 1 && quitOnLastClose) {
            if (ChooserService.count > 0) {
                quitDeferred = true;
                WindowService.close(windowId);
                return;
            }
            FolderHistory.unload();
            DuxFilesService.quit();
            return;
        }
        WindowService.close(windowId);
    }

    function quitWhenIdle() {
        if (!idle)
            return;
        FolderHistory.unload();
        DuxFilesService.quit();
    }

    Connections {
        target: DuxFilesService
        function onWindowActionRequested(action) {
            root.handleWindowAction(action);
        }
        function onOpenRequested(locations, newWindow) {
            if (locations.length === 0 && !newWindow)
                return;
            const windowId = WindowService.open(locations, newWindow);
            root.focusWindow(windowId);
        }
        function onPropertiesRequested(paths) {
            if (paths.length === 0)
                return;
            const windowId = WindowService.target() === 0 ? WindowService.create([]) : WindowService.target();
            root.focusWindow(windowId);
            root.windowObject(windowId)?.showPropertiesFor(paths);
        }
        function onChooserRequested(request) {
            ChooserService.open(request);
        }
        function onChooserCloseRequested(id) {
            ChooserService.close(id);
        }
    }

    Connections {
        target: WindowService
        function onOpenInWindow(windowId, locations) {
            root.windowObject(windowId)?.openLocations(locations, false);
        }
        function onWindowAdded() {
            focusRetry.restart();
        }
    }

    Timer {
        interval: 30000
        running: root.idle && !root.keepRunning && (root.dbusActivated || root.quitDeferred)
        onTriggered: root.quitWhenIdle()
    }

    Timer {
        id: focusRetry
        interval: 150
        repeat: false
        onTriggered: root.focusWindow(WindowService.lastActiveId)
    }

    Instantiator {
        id: chooserInstances

        model: ChooserService.requests

        ChooserWindow {
            required property string id

            request: ChooserService.requestFor(id)
        }
    }

    Instantiator {
        id: windowInstances

        model: WindowService.windows

        MainWindow {
            onNewWindowRequested: locations => WindowService.create(locations)
            onCloseRequested: root.closeWindow(windowId)
        }
    }
}
