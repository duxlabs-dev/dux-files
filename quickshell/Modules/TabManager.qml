import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

QtObject {
    id: manager

    required property var panes
    required property var activePane
    required property int activePaneIndex
    required property bool primary

    property var tabs: []
    property int currentTab: 0
    property var closedTabs: []
    property bool applyingTab: false

    readonly property var primaryPane: panes[0]
    readonly property var secondPane: panes[1] ?? null

    signal activePaneRequested(int index)
    signal dropRequested(var paths, string dest, string action)

    function tabState(index) {
        return tabs[index] ?? null;
    }

    function paneStates() {
        const stored = tabState(currentTab)?.panes ?? [];
        return [primaryPane.saveState(), secondPane ? secondPane.saveState() : (stored[1] ?? null)];
    }

    function captureTabs() {
        if (applyingTab || tabs.length === 0)
            return;
        const next = tabs.slice();
        next[currentTab] = {
            "panes": paneStates(),
            "active": activePaneIndex
        };
        tabs = next;
        persistTabs();
    }

    function applyTab(index) {
        const state = tabState(index);
        if (!state)
            return;
        applyingTab = true;
        primaryPane.restoreState(state.panes?.[0] ?? {
            "path": Paths.strip(Paths.home)
        });
        leaveMissingFolder(primaryPane);
        if (secondPane) {
            secondPane.restoreState(state.panes?.[1] ?? {
                "path": primaryPane.path
            });
            leaveMissingFolder(secondPane);
        }
        activePaneRequested(secondPane ? (state.active ?? 0) : 0);
        applyingTab = false;
    }

    function selectTab(index) {
        if (index === currentTab || index < 0 || index >= tabs.length)
            return;
        captureTabs();
        currentTab = index;
        applyTab(index);
        persistTabs();
    }

    function newTab(path, background) {
        captureTabs();
        const next = tabs.slice();
        next.push({
            "panes": [
                {
                    "path": path === "" ? SettingsData.folderFor(SettingsData.newTabFolder, activePane.path) : path
                },
                null],
            "active": 0
        });
        tabs = next;
        if (background) {
            persistTabs();
            return;
        }
        currentTab = next.length - 1;
        applyTab(currentTab);
        persistTabs();
        if (path !== "")
            FolderHistory.visit(primaryPane.path);
    }

    function closeTab(index) {
        closeTabsAt([index], index);
    }

    function cycleTab(step) {
        if (tabs.length < 2)
            return;
        selectTab((currentTab + step + tabs.length) % tabs.length);
    }

    function reopenTab() {
        if (closedTabs.length === 0)
            return;
        const stack = closedTabs.slice();
        const closed = stack.pop();
        closedTabs = stack;
        captureTabs();
        const next = tabs.slice();
        const index = Math.min(closed.index, next.length);
        next.splice(index, 0, closed.state);
        tabs = next;
        currentTab = index;
        applyTab(index);
        persistTabs();
    }

    function moveTab(from, to) {
        if (from === to || from < 0 || to < 0 || from >= tabs.length || to >= tabs.length)
            return;
        captureTabs();
        const current = tabs[currentTab];
        const next = tabs.slice();
        const moved = next.splice(from, 1)[0];
        next.splice(to, 0, moved);
        tabs = next;
        currentTab = next.indexOf(current);
        persistTabs();
    }

    function closeTabsAt(indices, fallback) {
        const closing = indices.filter(index => index >= 0 && index < tabs.length);
        if (closing.length === 0 || closing.length >= tabs.length)
            return;
        captureTabs();
        const current = tabs[currentTab];
        closedTabs = closedTabs.concat(closing.slice().sort((a, b) => b - a).map(index => ({
                    "index": index,
                    "state": tabs[index]
                }))).slice(-FilesMetrics.closedTabHistory);
        const kept = tabs.filter((_, index) => !closing.includes(index));
        tabs = kept;
        const index = kept.indexOf(current);
        if (index >= 0) {
            currentTab = index;
            persistTabs();
            return;
        }
        currentTab = Math.max(0, Math.min(fallback, kept.length - 1));
        applyTab(currentTab);
        persistTabs();
    }

    function closeOtherTabs(index) {
        closeTabsAt(tabs.map((_, other) => other).filter(other => other !== index), 0);
    }

    function closeTabsToRight(index) {
        closeTabsAt(tabs.map((_, other) => other).filter(other => other > index), index);
    }

    function duplicateTab(index) {
        if (!tabState(index))
            return;
        captureTabs();
        const next = tabs.slice();
        next.splice(index + 1, 0, Object.assign({}, tabs[index]));
        tabs = next;
        currentTab = index + 1;
        applyTab(currentTab);
        persistTabs();
    }

    function dropOnTab(index, drop) {
        const state = tabState(index);
        const dest = index === currentTab ? activePane.path : (state?.panes?.[state.active ?? 0]?.path || state?.panes?.[0]?.path || "");
        const paths = DragService.pathsFrom(drop);
        if (dest === "" || dest === RecentService.path || paths.length === 0 || paths.includes(dest))
            return;
        dropRequested(paths, dest, DragService.overrideFor(drop.proposedAction));
    }

    function folderTitle(path) {
        if (path === "")
            return I18n.tr("Files", "main window title");
        if (path === TrashService.filesPath)
            return I18n.tr("Trash", "trash view header and sidebar entry");
        if (path === RecentService.path)
            return I18n.tr("Recent", "recent files view header and sidebar entry");
        if (path === Paths.strip(Paths.home))
            return I18n.tr("Home", "sidebar entry for the user's home folder");
        if (path === "/")
            return "/";
        return FilePaths.baseName(path);
    }

    function tabLabel(state) {
        return folderTitle(state?.panes?.[0]?.path || Paths.strip(Paths.home));
    }

    function persistTabs() {
        if (!primary)
            return;
        SessionData.tabs = tabs;
        SessionData.currentTab = currentTab;
    }

    function tabsAt(path) {
        return [
            {
                "panes": [
                    {
                        "path": path
                    },
                    null],
                "active": 0
            }
        ];
    }

    function restoreSession() {
        const stored = SessionData.tabs;
        const folder = SettingsData.folderFor(SettingsData.startupFolder, "");
        if (folder !== "")
            tabs = tabsAt(folder);
        else
            tabs = Array.isArray(stored) && stored.length > 0 ? stored : tabsAt(Paths.strip(Paths.home));
        currentTab = Math.max(0, Math.min(SessionData.currentTab, tabs.length - 1));
        applyTab(currentTab);
    }

    function leaveMissingFolder(pane) {
        const path = pane.path;
        if (!path.startsWith("/"))
            return;
        DirectoryService.stat(path, result => {
            if (result.code === "ENOENT" && pane.path === path)
                pane.restoreState({
                    "path": Paths.strip(Paths.home)
                });
        });
    }

    function startFresh() {
        tabs = tabsAt(SettingsData.folderFor(SettingsData.newWindowFolder, Paths.strip(Paths.home)));
        currentTab = 0;
        applyTab(0);
    }
}
