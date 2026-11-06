pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser
import "../Common/PreviewView.js" as PreviewView

DFlickable {
    id: viewer

    property var backend: DirectoryService
    property string path: ""
    property string fileName: ""
    property bool markdown: false
    property Component highlighter: null
    property QtObject highlight: null
    property int request: 0
    property bool binary: false
    property bool failed: false
    property bool truncated: false
    property real fileSize: 0

    readonly property int maxBytes: 256 * 1024
    readonly property string note: truncated ? I18n.tr("First %1 of %2", "preview column note when only the start of the file is shown").arg(FileFormat.size(maxBytes)).arg(FileFormat.size(fileSize)) : ""

    function load() {
        const current = ++request;
        clear();
        if (path === "")
            return;
        backend.preview(path, maxBytes, result => {
            if (current !== viewer.request)
                return;
            if (result.error) {
                viewer.failed = true;
                return;
            }
            viewer.binary = result.binary === true;
            viewer.truncated = result.truncated === true;
            viewer.fileSize = result.size ?? 0;
            edit.text = viewer.markdown ? PreviewView.localMarkdown(result.text ?? "") : result.text ?? "";
            viewer.highlightText();
        });
    }

    function clear() {
        highlight?.destroy();
        highlight = null;
        binary = false;
        failed = false;
        truncated = false;
        edit.text = "";
        contentY = 0;
    }

    function highlightText() {
        if (markdown || highlighter?.status !== Component.Ready)
            return;
        highlight = highlighter.createObject(viewer, {
            "textEdit": edit,
            "fileName": fileName
        });
    }

    function copySelection() {
        if (edit.selectedText === "")
            return false;
        edit.copy();
        return true;
    }

    function scrollPage(direction) {
        contentY = Math.max(0, Math.min(contentHeight - height, contentY + direction * height));
    }

    onPathChanged: load()
    onMarkdownChanged: load()
    Component.onCompleted: load()

    contentWidth: width
    contentHeight: edit.implicitHeight
    clip: true

    TextEdit {
        id: edit

        width: viewer.width
        readOnly: true
        selectByMouse: true
        activeFocusOnPress: false
        textFormat: viewer.markdown ? TextEdit.MarkdownText : TextEdit.PlainText
        baseUrl: FilePaths.toFileUrl(FilePaths.parentOf(viewer.path)) + "/"
        wrapMode: TextEdit.WrapAtWordBoundaryOrAnywhere
        color: Theme.surfaceText
        selectionColor: Theme.primary
        selectedTextColor: Theme.onPrimary
        font.family: viewer.markdown ? Theme.fontFamily : Theme.monoFontFamily
        font.pixelSize: Theme.fontSizeMedium
        onLinkActivated: link => {
            if (/^(https?|mailto):/.test(link))
                Qt.openUrlExternally(link);
        }
    }
}
