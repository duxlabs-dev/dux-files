pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import qs.Common
import qs.DCommon.Common as DC

Singleton {
    id: root

    property bool popoutElevationEnabled: true
    property bool scrollbarsEnabled: true
    property int textRenderType: DC.Style.TextRenderType.Qt
    property int textRenderQuality: DC.Style.TextRenderQuality.Default
    property bool blurBorderEnabled: true
    property real blurBorderOpacity: 0.35
    property real blurLayerOutlineOpacity: 0
    property string blurBorderColor: "outline"
    property string blurBorderCustomColor: "#ffffff"

    property alias themeMode: adapter.themeMode
    property alias colorSource: adapter.colorSource
    property alias presetTheme: adapter.presetTheme
    property alias customThemeFile: adapter.customThemeFile
    property alias radiusStrength: adapter.radiusStrength
    property alias density: adapter.density
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
    property alias floatingWindowTransparency: adapter.floatingWindowTransparency
    property alias floatingWindowForegroundLayers: adapter.floatingWindowForegroundLayers
    property alias floatingWindowForegroundTransparency: adapter.floatingWindowForegroundTransparency
    property alias blurEnabled: adapter.blurEnabled
    property alias language: adapter.language
    readonly property bool keepRunning: Quickshell.env("DFILES_KEEP_RUNNING") === "1"
    property alias terminal: adapter.terminal
    property alias opener: adapter.opener
    property alias closeBehavior: adapter.closeBehavior
    property alias startupFolder: adapter.startupFolder
    property alias newWindowFolder: adapter.newWindowFolder
    property alias newTabFolder: adapter.newTabFolder
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
    property alias dateFormat: adapter.dateFormat
    property alias foldersFirst: adapter.foldersFirst
    property alias hiddenLast: adapter.hiddenLast
    property alias rememberFolderViews: adapter.rememberFolderViews
    property alias folderCounts: adapter.folderCounts
    property alias confirmTrash: adapter.confirmTrash
    property alias clickToOpen: adapter.clickToOpen
    property alias dragAction: adapter.dragAction
    property alias thumbnails: adapter.thumbnails
    property alias thumbnailSizeLimitMb: adapter.thumbnailSizeLimitMb
    property alias networkThumbnails: adapter.networkThumbnails
    property alias showDevicesSection: adapter.showDevicesSidebar
    property alias showSystemPartitions: adapter.showSystemPartitions
    property alias showNetworkSection: adapter.showNetworkSidebar
    property alias showGamesSidebar: adapter.showGamesSidebar
    property alias showSteamGames: adapter.showSteamGames
    property alias showHeroicGames: adapter.showHeroicGames
    property alias showLutrisGames: adapter.showLutrisGames
    property alias showGameSettingsFolders: adapter.showGameSettingsFolders
    property alias showGamePrefixes: adapter.showGamePrefixes
    property alias showGameFolders: adapter.showGameFolders
    property alias mountOnClick: adapter.mountOnClick
    property alias searchHidden: adapter.searchHidden
    property alias searchExcludes: adapter.searchExcludes
    property alias vimKeys: adapter.vimKeys
    property alias listTyping: adapter.listTyping
    property alias gtkBookmarksOffered: adapter.gtkBookmarksOffered

    function folderFor(setting, fallback) {
        if (setting === "home")
            return Paths.strip(Paths.home);
        return setting.startsWith("/") ? setting : fallback;
    }

    function flush() {
        store.flush();
    }

    Component.onCompleted: store.read()
    Component.onDestruction: flush()

    JsonStore {
        id: store

        name: "settings"
        path: Paths.configDir + "/ui-settings.json"
        watchChanges: true

        adapter: JsonAdapter {
            id: adapter
            property string themeMode: "auto"
            property string colorSource: "auto"
            property string presetTheme: "purple"
            property string customThemeFile: ""
            property int radiusStrength: 50
            property string density: "comfortable"
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
            property real floatingWindowTransparency: 1.0
            property bool floatingWindowForegroundLayers: true
            property real floatingWindowForegroundTransparency: 1.0
            property bool blurEnabled: false
            property string language: ""
            property string terminal: ""
            property string opener: ""
            property string closeBehavior: "hide"
            property string startupFolder: "last"
            property string newWindowFolder: "current"
            property string newTabFolder: "current"
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
            property string dateFormat: "locale"
            property bool foldersFirst: true
            property bool hiddenLast: true
            property bool rememberFolderViews: true
            property bool folderCounts: true
            property bool confirmTrash: false
            property string clickToOpen: "double"
            property string dragAction: "auto"
            property bool thumbnails: true
            property int thumbnailSizeLimitMb: 64
            property bool networkThumbnails: false
            property bool showDevicesSidebar: true
            property bool showSystemPartitions: false
            property bool showNetworkSidebar: true
            property bool showGamesSidebar: true
            property bool showSteamGames: true
            property bool showHeroicGames: true
            property bool showLutrisGames: true
            property bool showGameSettingsFolders: true
            property bool showGamePrefixes: true
            property bool showGameFolders: true
            property bool mountOnClick: true
            property bool searchHidden: false
            property var searchExcludes: []
            property bool vimKeys: false
            property string listTyping: "jump"
            property bool gtkBookmarksOffered: false
        }
    }
}
