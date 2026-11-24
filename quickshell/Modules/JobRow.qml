import QtQuick
import QtQuick.Layouts
import qs.Common
import qs.Services
import qs.DCommon.Widgets
import qs.DCommon.FileBrowser

Item {
    id: row

    required property var job

    readonly property bool finished: job.state === "done" || job.state === "failed"
    readonly property bool succeeded: job.state === "done" && !job.cancelled
    readonly property string title: (succeeded && JobLabels.done(job.kind)) || JobLabels.active(job.kind)
    readonly property string outcomeIcon: {
        if (job.state === "failed")
            return "error";
        return job.cancelled ? "cancel" : "check_circle";
    }
    readonly property real fraction: {
        const total = job.progress.bytesTotal > 0 ? job.progress.bytesTotal : job.progress.filesTotal;
        const done = job.progress.bytesTotal > 0 ? job.progress.bytesDone : job.progress.filesDone;
        if (total <= 0)
            return 0;
        return Math.min(1, done / total);
    }

    function detail() {
        if (job.state === "failed")
            return FileFormat.operationError(job.code, job.error);
        if (job.cancelled)
            return I18n.tr("Cancelled", "file operation was cancelled");
        if (job.state === "queued")
            return I18n.tr("Waiting", "file operation is queued");
        if (job.progress.estimating)
            return I18n.tr("Estimating…", "file operation is still measuring the work");
        if (finished)
            return I18n.tr("Finished", "file operation completed");
        return FilePaths.baseName(job.progress.currentPath);
    }

    implicitHeight: content.implicitHeight

    ColumnLayout {
        id: content

        anchors.left: parent.left
        anchors.right: parent.right
        spacing: Theme.spacingXS

        RowLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingS

            DIcon {
                visible: row.finished
                name: row.outcomeIcon
                size: Theme.iconSizeSmall
                color: row.job.state === "failed" ? Theme.error : Theme.primary
            }

            StyledText {
                text: row.title
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeMedium
            }

            Item {
                Layout.fillWidth: true
            }

            StyledText {
                text: row.job.progress.filesTotal > 0 ? `${row.job.progress.filesDone}/${row.job.progress.filesTotal}` : ""
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
            }

            DActionButton {
                iconName: "close"
                buttonSize: Theme.buttonHeightXS
                iconSize: Theme.iconSizeSmall
                tooltipText: row.finished ? I18n.tr("Clear", "file operations list: remove a finished operation") : I18n.tr("Cancel", "dialog button to cancel")
                onClicked: row.finished ? FileOpsService.dismiss(row.job.id) : FileOpsService.cancel(row.job.id)
            }
        }

        Rectangle {
            id: expiry

            property real remaining: FileOpsService.historyRemaining(row.job.id)

            Layout.fillWidth: true
            Layout.preferredHeight: Theme.spacingXS
            visible: row.finished
            radius: Theme.cornerRadiusFull
            color: Theme.chipSurface

            Rectangle {
                width: parent.width * expiry.remaining
                height: parent.height
                radius: parent.radius
                color: Theme.primary
            }

            NumberAnimation {
                target: expiry
                property: "remaining"
                from: FileOpsService.historyRemaining(row.job.id)
                to: 0
                duration: from * FileOpsService.historyDuration
                running: row.finished && from > 0 && !FileOpsService.historyHeld
            }
        }

        Rectangle {
            Layout.fillWidth: true
            visible: !row.finished
            Layout.preferredHeight: Theme.spacingXS
            radius: Theme.cornerRadiusFull
            color: Theme.chipSurface

            Rectangle {
                width: parent.width * row.fraction
                height: parent.height
                radius: parent.radius
                color: Theme.primary

                Behavior on width {
                    enabled: Theme.animationsEnabled
                    NumberAnimation {
                        duration: Theme.expressiveDurations.expressiveSlowEffects
                        easing.type: Easing.BezierSpline
                        easing.bezierCurve: Theme.expressiveCurves.expressiveSlowEffects
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingS

            StyledText {
                Layout.fillWidth: true
                text: row.detail()
                color: row.job.state === "failed" ? Theme.error : Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                elide: Text.ElideMiddle
            }

            StyledText {
                visible: row.job.state === "running" && text !== ""
                text: FilesFormat.transferHint(row.job.progress)
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                font.features: {
                    "tnum": 1
                }
            }
        }
    }
}
