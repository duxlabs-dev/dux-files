pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

OverlayDialog {
    id: root

    property var mimes: []
    property var paths: []
    property var sections: ({})
    property bool mixed: false
    property bool rememberChoice: false
    property int requestSerial: 0

    readonly property string mime: mimes.length === 1 ? mimes[0] : ""
    readonly property var current: appList.currentIndex >= 0 && appList.currentIndex < matches.length ? matches[appList.currentIndex] : null
    readonly property string selectedId: current ? current.id : ""
    readonly property bool canSetDefault: mime !== "" && SandboxService.available("setDefault")

    readonly property var matches: {
        const needle = searchField.text.trim().toLowerCase();
        const rows = [];
        for (const section of ["recent", "recommended", "other"]) {
            for (const app of sections[section] ?? []) {
                if (needle !== "" && !root.appMatches(app, needle))
                    continue;
                rows.push(Object.assign({
                    "section": section
                }, app));
            }
        }
        return rows;
    }

    function appMatches(app, needle) {
        return app.name.toLowerCase().includes(needle) || (app.genericName ?? "").toLowerCase().includes(needle) || app.id.toLowerCase().includes(needle);
    }

    function sectionLabel(section) {
        switch (section) {
        case "recent":
            return I18n.tr("Recently used", "open with dialog section listing apps picked before for this file type");
        case "recommended":
            return I18n.tr("Recommended", "open with dialog section listing apps registered for the file type");
        default:
            return I18n.tr("Other applications", "open with dialog section listing every remaining app");
        }
    }

    function ask(mimeTypes, targets) {
        mimes = (Array.isArray(mimeTypes) ? mimeTypes : [mimeTypes]).filter(mimeType => typeof mimeType === "string" && mimeType !== "");
        paths = targets;
        sections = {};
        mixed = false;
        rememberChoice = false;
        searchField.text = "";
        const token = ++requestSerial;
        AppsService.sections(mimes, mimes.length > 0 ? [] : targets, response => {
            if (token !== requestSerial || !visible)
                return;
            if (response.error) {
                ToastService.info(FileFormat.operationError(response.code, response.error));
                return;
            }
            mimes = response.mimeTypes || [];
            mixed = response.mixed === true;
            sections = {
                "recent": response.recent || [],
                "recommended": response.recommended || [],
                "other": response.other || []
            };
            appList.currentIndex = 0;
        });
        open();
        searchField.forceActiveFocus();
    }

    function submit() {
        if (selectedId === "")
            return;
        if (rememberChoice && canSetDefault)
            AppsService.setDefault(mime, selectedId, null);
        AppsService.openWith(paths, selectedId, response => {
            if (response.error)
                ToastService.info(FileFormat.operationError(response.code, response.error));
        });
        close();
    }

    title: I18n.tr("Open with", "open with dialog header")
    acceptEnabled: selectedId !== ""
    onAccepted: submit()
    onClosed: sections = {}

    DSearchField {
        id: searchField

        width: parent.width
        placeholderText: I18n.tr("Search applications", "open with dialog search placeholder")
        onAccepted: root.submit()
        Keys.onDownPressed: appList.forceActiveFocus()
    }

    DListView {
        id: appList

        width: parent.width
        height: Theme.listItemHeight * 7
        clip: true
        spacing: Theme.groupedListGap
        model: root.matches
        focus: true
        currentIndex: 0
        section.property: "section"
        section.delegate: Item {
            id: header

            required property string section

            width: appList.width
            height: label.implicitHeight + Theme.spacingM + Theme.spacingXS

            StyledText {
                id: label

                anchors.left: parent.left
                anchors.leftMargin: Theme.spacingL
                anchors.bottom: parent.bottom
                anchors.bottomMargin: Theme.spacingXS
                text: root.sectionLabel(header.section)
                color: Theme.primary
                font.pixelSize: Theme.fontSizeSmall
                font.weight: Theme.fontWeightMedium
            }
        }

        delegate: AppRow {
            firstInGroup: root.matches[index - 1]?.section !== modelData.section
            lastInGroup: root.matches[index + 1]?.section !== modelData.section
            onClicked: appList.currentIndex = index
            onDoubleClicked: root.submit()
        }
    }

    DToggle {
        width: parent.width
        visible: (root.mime !== "" || root.mixed) && SandboxService.available("setDefault")
        enabled: root.canSetDefault
        text: I18n.tr("Always use this application", "open with dialog option that sets the default")
        checked: root.rememberChoice
        onToggled: checked => root.rememberChoice = checked
    }

    actions: [
        DButton {
            text: I18n.tr("Open", "context menu entry opening a file")
            enabled: root.selectedId !== ""
            onClicked: root.submit()
        }
    ]
}
