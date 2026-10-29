pragma ComponentBehavior: Bound

import QtQuick
import QtTest
import Quickshell
import qs.Common
import qs.Services
import qs.Modules
import qs.DCommon.Common as DC
import qs.DCommon.FileBrowser

ShellRoot {
    id: root

    readonly property var log: Log.scoped("fixture")

    property var window: null
    property bool ready: false
    property int mountRequests: 0
    property var mountReply: null

    Component {
        id: mainWindow

        MainWindow {
            windowId: 1
        }
    }

    Component.onCompleted: {
        DC.Style.theme = Theme;
        DC.Style.settings = SettingsData;
        DC.I18n.backend = I18n;
        DC.Paths.backend = Paths;
        DC.Log.backend = Log;
        DC.Host.session = SessionService;
        DC.Host.cache = CacheData;
        DC.Host.files = DirectoryService;
        window = mainWindow.createObject(root);
        ready = true;
    }

    readonly property QtObject fakeTransport: QtObject {
        property bool connected: true
        function subscribeTopic(topic, callback) {
        }
        function unsubscribeTopic(topic, callback) {
        }
        function call(method, params, callback) {
            sendRequest(method, params, response => callback?.(DuxFilesService.unwrap(response)));
        }
        function sendRequest(method, params, callback) {
            let result = {};
            switch (method) {
            case "devices.mount":
                root.mountRequests++;
                root.mountReply = callback;
                return;
            case "devices.list":
                result = {
                    available: true,
                    devices: [
                        {
                            id: "root",
                            name: "System SSD",
                            system: true,
                            hidden: false,
                            mounted: true,
                            mountPoint: "/",
                            mountPoints: ["/"],
                            device: "/dev/nvme0n1p2",
                            fstype: "ext4",
                            iconName: "drive-harddisk"
                        },
                        {
                            id: "home",
                            name: "Home",
                            system: false,
                            protected: true,
                            hidden: true,
                            mounted: true,
                            mountPoint: "/home",
                            mountPoints: ["/home"],
                            device: "/dev/nvme0n1p3",
                            iconName: "drive-harddisk"
                        },
                        {
                            id: "data",
                            name: "Projects",
                            system: false,
                            hidden: true,
                            mounted: true,
                            mountPoint: "/mnt/projects",
                            mountPoints: ["/mnt/projects", "/srv/projects"],
                            device: "/dev/nvme1n1p1",
                            fstype: "btrfs",
                            iconName: "drive-harddisk"
                        },
                        {
                            id: "usb",
                            name: "Backup",
                            system: false,
                            hidden: false,
                            mounted: false,
                            mountPoint: "",
                            mountPoints: [],
                            device: "/dev/sda1",
                            fstype: "ext4",
                            iconName: "drive-removable-media"
                        }
                    ]
                };
                break;
            case "devices.usage":
                result = {
                    usage: [
                        {
                            mountPoint: "/",
                            total: 1000000000000,
                            used: 250000000000,
                            free: 750000000000
                        },
                        {
                            mountPoint: "/mnt/projects",
                            total: 2000000000000,
                            used: 1200000000000,
                            free: 800000000000
                        },
                        {
                            mountPoint: "/srv/projects",
                            total: 2000000000000,
                            used: 1200000000000,
                            free: 800000000000
                        }
                    ]
                };
                break;
            case "network.mounts":
                result = {
                    available: true,
                    mounts: [
                        {
                            id: "nas",
                            name: "NAS",
                            path: "/run/user/1000/gvfs/nas",
                            kind: "network",
                            iconName: "folder-remote"
                        }
                    ]
                };
                break;
            }
            if (callback)
                callback({
                    result
                });
        }
    }

    SignalSpy {
        id: dragSpy
        signalName: "itemDragStarted"
    }

    TestCase {
        name: "Interactions"
        when: root.ready
        function check(actual, expected) {
            compare(actual, expected);
        }
        function entries(directory) {
            directory.entries.clear();
            directory.error = "";
            directory.loading = false;
            for (let i = 0; i < 60; i++)
                directory.entries.append({
                    name: "Document " + i + ".txt",
                    path: "/tmp/document" + i,
                    isDir: false,
                    isSymlink: false,
                    symlinkBroken: false,
                    size: 1024,
                    mtimeMs: 1000,
                    ctimeMs: 1000,
                    mode: "-rw-r--r--",
                    owner: "brandon",
                    group: "brandon",
                    iconName: "text-plain",
                    thumbnail: "",
                    mime: "text/plain",
                    extension: "txt",
                    displayName: "",
                    untrusted: false,
                    unreadable: false,
                    hidden: false
                });
        }
        function test_interactions() {
            const win = root.window;
            const surface = win.contentItem.children.find(child => child.isFloatingWindowSurface === true);
            SettingsData.animationDuration = 0;
            win.minimumSize = Qt.size(1100, 760);
            win.implicitWidth = 1100;
            win.implicitHeight = 760;
            CacheData.sidebarWidth = 240;
            DevicesService.transport = root.fakeTransport;
            NetworkService.transport = root.fakeTransport;
            wait(200);
            const pane = win.activePane;
            pane.navigate("/tmp");
            pane.setViewMode("grid");
            wait(100);
            entries(pane.body.directory);
            wait(150);
            let view = pane.body.view;
            dragSpy.target = view;
            view.positionViewAtBeginning();
            const tile = view.itemAtIndex(0);
            const blankY = FileBrowserMetrics.gridTileVerticalPadding / 2;
            check(tile.containsContent(tile.width / 2, blankY), false);
            mousePress(tile, tile.width / 2, blankY);
            mouseMove(view, view.cellWidth * 2 - 2, view.cellHeight - 2);
            wait(100);
            check(pane.selection.paths.includes("/tmp/document0"), true);
            wait(100);
            mouseRelease(view, view.cellWidth * 2 - 2, view.cellHeight - 2);
            check(dragSpy.count, 0);
            check(pane.selection.paths.length, 2);
            const visual = tile.hitTargets[0];
            mouseClick(visual, visual.width / 2, visual.height / 2);
            check(pane.selection.paths, ["/tmp/document0"]);
            mousePress(visual, visual.width / 2, visual.height / 2);
            mouseMove(visual, visual.width / 2 + 40, visual.height / 2);
            mouseRelease(visual, visual.width / 2 + 40, visual.height / 2);
            check(dragSpy.count, 1);
            wait(50);
            DragService.end();
            pane.selection.select("/tmp/document10");
            mousePress(tile, tile.width / 2, blankY, Qt.LeftButton, Qt.ControlModifier);
            mouseMove(view, view.cellWidth * 2 - 2, view.cellHeight - 2);
            mouseRelease(view, view.cellWidth * 2 - 2, view.cellHeight - 2, Qt.LeftButton, Qt.ControlModifier);
            check(pane.selection.paths.includes("/tmp/document10"), true);
            check(pane.selection.paths.includes("/tmp/document1"), true);
            check(view.indicesIn(Qt.rect(view.cellWidth - FileBrowserMetrics.gridGap + 1, 20, FileBrowserMetrics.gridGap - 2, 20)).length, 0);
            SettingsData.language = "ar";
            wait(100);
            const firstTile = view.itemAtIndex(0);
            check(view.indicesIn(Qt.rect(firstTile.x + 1, firstTile.y + 1, 10, 10)), [0]);
            SettingsData.language = "en";
            wait(100);
            const before = view.contentY;
            mouseWheel(surface, surface.width - 4, 200, 0, -120);
            wait(100);
            check(view.contentY > before, true);
            view.positionViewAtBeginning();
            mousePress(view, view.cellWidth - 2, view.cellHeight - 2);
            mouseMove(view, view.cellWidth - 2, view.height - 2);
            wait(250);
            check(view.contentY > view.originY, true);
            mouseRelease(view, view.cellWidth - 2, view.height - 2);
            pane.setViewMode("list");
            wait(150);
            view = pane.body.view;
            dragSpy.target = view;
            dragSpy.clear();
            view.positionViewAtBeginning();
            const row = view.itemAtIndex(0);
            const name = row.hitTargets[1];
            const blankX = name.x + name.contentWidth + Theme.spacingL;
            check(row.containsContent(blankX, row.height / 2), false);
            mousePress(row, blankX, row.height / 2);
            mouseMove(view, view.width - 80, view.rowPitch * 4);
            wait(100);
            check(pane.selection.paths.length > 1, true);
            mouseRelease(view, view.width - 80, view.rowPitch * 4);
            check(dragSpy.count, 0);
            mouseClick(row, blankX, row.height / 2);
            check(pane.selection.paths, ["/tmp/document0"]);
            pane.body.directory.entries.setProperty(0, "isDir", true);
            view.chevrons = true;
            wait(50);
            const chevron = row.hitTargets[2];
            verify(chevron.width > 0);
            pane.selection.clear();
            mouseClick(chevron, chevron.width / 2, chevron.height / 2);
            check(pane.selection.paths, ["/tmp/document0"]);
            view.chevrons = false;
            pane.body.directory.entries.setProperty(0, "isDir", false);
            view.positionViewAtBeginning();
            mouseWheel(surface, surface.width - 4, 200, 0, -120);
            wait(100);
            check(view.contentY > view.originY, true);
            function sidebarIn(item) {
                if (item instanceof Sidebar && item.visible)
                    return item;
                for (const child of item.children) {
                    const found = sidebarIn(child);
                    if (found)
                        return found;
                }
                return null;
            }
            const sidebar = sidebarIn(surface);
            verify(sidebar !== null);
            const rows = sidebar.rows();
            const system = rows.find(item => item.label === "/");
            const projects = rows.find(item => item.label === "/mnt/projects");
            verify(system !== undefined);
            verify(projects !== undefined);
            check(system.subtitle, "/dev/nvme0n1p2");
            check(projects.subtitle, "/dev/nvme1n1p1");
            check(system.actionIcon, "");
            const home = rows.find(item => item.label === "/home");
            verify(home !== undefined);
            check(home.actionIcon, "");
            check(home.actionTooltip, "");
            check(projects.actionTooltip, "Unmount");
            check(system.ringValue, 0.25);
            check(projects.ringValue, 0.6);
            verify(system.tooltipText.includes("/dev/nvme0n1p2"));
            verify(projects.tooltipText.includes("/mnt/projects"));
            projects.clicked();
            check(pane.path, "/mnt/projects");
            const backup = rows.find(item => item.label === "Backup");
            check(backup.actionTooltip, "Mount");
            backup.clicked();
            backup.clicked();
            check(root.mountRequests, 1);
            root.mountReply({
                error: "cancelled",
                code: "CANCELLED"
            });
            pane.restoreState({
                path: "computer:",
                back: ["/tmp", "computer:"]
            });
            check(pane.path, "/");
            pane.back();
            check(pane.path, "/");
            function jobsButtonIn(item) {
                if (item instanceof JobsButton)
                    return item;
                for (const child of item.children) {
                    const found = jobsButtonIn(child);
                    if (found)
                        return found;
                }
                return null;
            }
            const topbarJobs = jobsButtonIn(win.topbar);
            verify(topbarJobs !== null);
            DC.Host.ownWindowDecorations = false;
            wait(50);
            const topbarY = win.topbar.mapToItem(surface, 0, 0).y;
            FileOpsService.jobs = [
                {
                    id: "j1",
                    state: "running"
                }
            ];
            wait(50);
            check(win.topbar.mapToItem(surface, 0, 0).y, topbarY);
            check(topbarJobs.visible, true);
            DC.Host.ownWindowDecorations = true;
            wait(50);
            check(topbarJobs.visible, false);
            SettingsData.showTitleBar = false;
            wait(50);
            check(topbarJobs.visible, true);
            SettingsData.showTitleBar = true;
            FileOpsService.jobs = [];
            root.log.info("FIXTURE PASS");
        }
    }
}
