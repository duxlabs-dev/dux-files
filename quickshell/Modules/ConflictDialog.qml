import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.Services
import qs.DCommon.Widgets

PausedJobDialog {
    id: root

    readonly property var conflict: prompt

    pausedState: "paused-conflict"
    promptKey: "conflict"
    acceptEnabled: offers("keepBoth")
    title: I18n.tr("This item already exists", "file conflict dialog header")
    supportingText: conflict ? I18n.tr("%1 already exists in this folder.", "file conflict dialog explanation").arg(conflict.target.name) : ""
    onAccepted: {
        if (offers("keepBoth"))
            answer("keepBoth");
    }

    ColumnLayout {
        width: parent.width
        spacing: Theme.spacingM

        ConflictSide {
            Layout.fillWidth: true
            label: I18n.tr("Already here", "file conflict dialog label for the existing item")
            entry: root.conflict ? root.conflict.target : null
        }

        ConflictSide {
            Layout.fillWidth: true
            label: I18n.tr("Being added", "file conflict dialog label for the incoming item")
            entry: root.conflict ? root.conflict.source : null
        }

        DToggle {
            Layout.fillWidth: true
            text: I18n.tr("Do this for the rest", "file conflict dialog option to reuse the answer")
            checked: root.applyToAll
            onToggled: checked => root.applyToAll = checked
        }
    }

    actions: [
        DButton {
            visible: root.offers("merge")
            text: I18n.tr("Merge", "file conflict dialog button to merge folders")
            variant: "tonal"
            onClicked: root.answer("merge")
        },
        DButton {
            visible: root.offers("skip")
            text: I18n.tr("Skip", "file conflict dialog button to leave the item alone")
            variant: "tonal"
            onClicked: root.answer("skip")
        },
        DButton {
            visible: root.offers("replace")
            text: I18n.tr("Replace", "file conflict dialog button to overwrite the existing item")
            variant: "tonal"
            danger: true
            onClicked: root.answer("replace")
        },
        DButton {
            visible: root.offers("keepBoth")
            text: I18n.tr("Keep both", "file conflict dialog button to keep both items")
            onClicked: root.answer("keepBoth")
        }
    ]
}
