pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.DCommon.Widgets

OverlayDialog {
    id: root

    property string path: ""
    property string name: ""

    signal runRequested
    signal editRequested

    function ask(entryPath, entryName) {
        path = entryPath;
        name = entryName;
        open();
    }

    title: I18n.tr("Run this file?", "executable file dialog header")
    supportingText: I18n.tr("%1 can be run as a program. Open it in an editor instead to read it first.", "executable file dialog explanation").arg(name)
    onAccepted: {
        runRequested();
        close();
    }

    actions: [
        DButton {
            text: I18n.tr("Open in editor", "executable file dialog action opening the file as text")
            variant: "tonal"
            onClicked: {
                root.editRequested();
                root.close();
            }
        },
        DButton {
            text: I18n.tr("Run", "executable file dialog action running the file")
            onClicked: {
                root.runRequested();
                root.close();
            }
        }
    ]
}
