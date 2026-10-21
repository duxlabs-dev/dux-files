pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Common

Singleton {
    id: root

    enum AnimationSpeed {
        None,
        Short,
        Medium,
        Long,
        Custom
    }

    enum TextRenderType {
        Qt,
        Native,
        Curve
    }

    enum TextRenderQuality {
        Default,
        Low,
        Normal,
        High,
        VeryHigh
    }

    property bool popoutElevationEnabled: true
    property bool scrollbarsEnabled: true
    property int textRenderType: SettingsData.TextRenderType.Qt
    property int textRenderQuality: SettingsData.TextRenderQuality.Default
    property bool blurBorderEnabled: true
    property real blurBorderOpacity: 0.35
    property real blurLayerOutlineOpacity: 0
    property string blurBorderColor: "outline"
    property string blurBorderCustomColor: "#ffffff"
    property bool powerActionConfirm: true
    property real powerActionHoldDuration: 0.5
    property var powerMenuActions: ["reboot", "logout", "poweroff", "lock", "suspend", "restart"]
    property string powerMenuDefaultAction: "logout"
    property bool powerMenuGridLayout: false

    readonly property string settingsPath: Paths.configDir + "/ui-settings.json"

    property alias themeMode: adapter.themeMode
    property alias colorSource: adapter.colorSource
    property alias presetTheme: adapter.presetTheme
    property alias customThemeFile: adapter.customThemeFile
    property alias radiusStrength: adapter.radiusStrength
    property alias animationDuration: adapter.animationDuration
    property alias reduceMotion: adapter.reduceMotion
    property alias springBounce: adapter.springBounce
    property alias enableRippleEffects: adapter.enableRippleEffects
    property alias fontSource: adapter.fontSource
    property alias fontFamily: adapter.fontFamily
    property alias monoFontFamily: adapter.monoFontFamily
    property alias displayFontFamily: adapter.displayFontFamily
    property alias fontWeight: adapter.fontWeight
    property alias fontScale: adapter.fontScale
    property alias focusRingEnabled: adapter.focusRingEnabled
    property alias focusRingWidth: adapter.focusRingWidth
    property alias focusRingColor: adapter.focusRingColor
    property alias windowTitleBars: adapter.windowTitleBars
    property alias language: adapter.language
    property alias terminal: adapter.terminal
    property alias closeBehavior: adapter.closeBehavior
    property alias openLocationIn: adapter.openLocationIn
    property alias extractOnOpen: adapter.extractOnOpen
    property alias registerFileManager: adapter.registerFileManager
    property alias recentFiles: adapter.recentFiles
    property alias sidebarCollapsed: adapter.sidebarCollapsed
    property alias defaultView: adapter.defaultView
    property alias defaultSort: adapter.defaultSort
    property alias defaultSortDescending: adapter.defaultSortDescending
    property alias showHidden: adapter.showHidden
    property alias showExtensions: adapter.showExtensions
    property alias foldersFirst: adapter.foldersFirst
    property alias rememberFolderViews: adapter.rememberFolderViews
    property alias folderCounts: adapter.folderCounts
    property alias confirmTrash: adapter.confirmTrash
    property alias clickToOpen: adapter.clickToOpen
    property alias dragAction: adapter.dragAction
    property alias thumbnails: adapter.thumbnails
    property alias thumbnailSizeLimitMb: adapter.thumbnailSizeLimitMb
    property alias networkThumbnails: adapter.networkThumbnails
    property alias showSystemPartitions: adapter.showSystemPartitions
    property alias mountOnClick: adapter.mountOnClick
    property alias searchHidden: adapter.searchHidden
    property alias searchExcludes: adapter.searchExcludes
    property alias vimKeys: adapter.vimKeys
    property alias listTyping: adapter.listTyping
    property alias gtkBookmarksOffered: adapter.gtkBookmarksOffered

    function flush() {
        if (!writeDebounce.running)
            return;
        writeDebounce.stop();
        settingsFile.blockWrites = true;
        settingsFile._write();
    }

    Component.onDestruction: flush()

    Timer {
        id: writeDebounce

        interval: 400
        onTriggered: {
            if (!Paths.ready) {
                restart();
                return;
            }
            settingsFile._write();
        }
    }

    FileView {
        id: settingsFile
        path: root.settingsPath
        blockLoading: true
        atomicWrites: true
        watchChanges: true
        printErrors: false

        property bool _loaded: false
        property bool _selfWrite: false

        onLoaded: _loaded = true
        onLoadFailed: _loaded = true
        onFileChanged: {
            if (_selfWrite) {
                _selfWrite = false;
                return;
            }
            reload();
        }
        onAdapterUpdated: {
            if (!_loaded)
                return;
            writeDebounce.restart();
        }

        function _write() {
            _selfWrite = true;
            writeAdapter();
        }

        JsonAdapter {
            id: adapter
            property string themeMode: "auto"
            property string colorSource: "auto"
            property string presetTheme: "purple"
            property string customThemeFile: ""
            property int radiusStrength: 50
            property int animationDuration: 250
            property bool reduceMotion: false
            property int springBounce: 1
            property bool enableRippleEffects: true
            property string fontSource: "auto"
            property string fontFamily: ""
            property string monoFontFamily: ""
            property string displayFontFamily: ""
            property int fontWeight: 400
            property real fontScale: 1.0
            property bool focusRingEnabled: true
            property real focusRingWidth: 1.5
            property string focusRingColor: "primary"
            property bool windowTitleBars: true
            property string language: ""
            property string terminal: ""
            property string closeBehavior: "quit"
            property string openLocationIn: "current"
            property bool extractOnOpen: true
            property bool registerFileManager: true
            property bool recentFiles: true
            property bool sidebarCollapsed: false
            property string defaultView: "list"
            property string defaultSort: "name"
            property bool defaultSortDescending: false
            property bool showHidden: false
            property bool showExtensions: true
            property bool foldersFirst: true
            property bool rememberFolderViews: true
            property bool folderCounts: true
            property bool confirmTrash: false
            property string clickToOpen: "double"
            property string dragAction: "auto"
            property bool thumbnails: true
            property int thumbnailSizeLimitMb: 64
            property bool networkThumbnails: false
            property bool showSystemPartitions: false
            property bool mountOnClick: true
            property bool searchHidden: false
            property var searchExcludes: []
            property bool vimKeys: false
            property string listTyping: "jump"
            property bool gtkBookmarksOffered: false
        }
    }
}
