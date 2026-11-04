pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtMultimedia
import qs.Common
import qs.DCommon.Widgets
import "../Common/PreviewView.js" as PreviewView

Item {
    id: media

    property url source
    property QtObject sound: QtObject {
        property real volume: 1
        property bool muted: false
    }
    readonly property bool failed: player.error !== MediaPlayer.NoError
    readonly property bool playing: player.playbackState === MediaPlayer.PlayingState

    onSoundChanged: volumeSlider.value = Math.round(sound.volume * 100)

    function togglePlayback() {
        if (playing) {
            player.pause();
            return;
        }
        player.play();
    }

    MediaPlayer {
        id: player

        source: media.source
        autoPlay: true
        videoOutput: output
        audioOutput: AudioOutput {
            volume: media.sound.volume
            muted: media.sound.muted
        }
        onPositionChanged: {
            if (!seek.isDragging)
                seek.value = player.position;
        }
    }

    VideoOutput {
        id: output

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: controls.top
        anchors.bottomMargin: Theme.spacingS
        visible: player.hasVideo

        MouseArea {
            anchors.fill: parent
            onClicked: media.togglePlayback()
        }
    }

    RowLayout {
        id: controls

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        spacing: Theme.spacingS
        visible: !media.failed

        DActionButton {
            iconName: media.playing ? "pause" : "play_arrow"
            tooltipText: media.playing ? I18n.tr("Pause", "tooltip on the button pausing a video or audio file in the large preview") : I18n.tr("Play", "tooltip on the button playing a video or audio file in the large preview")
            onClicked: media.togglePlayback()
        }

        StyledText {
            text: PreviewView.clock(player.position)
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
        }

        DSlider {
            id: seek

            Layout.fillWidth: true
            minimum: 0
            maximum: Math.max(1, player.duration)
            enabled: player.seekable
            showValue: false
            wheelEnabled: false
            onSliderValueChanged: newValue => player.setPosition(newValue)
        }

        StyledText {
            text: PreviewView.clock(player.duration)
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
        }

        DActionButton {
            iconName: media.sound.muted || media.sound.volume === 0 ? "volume_off" : "volume_up"
            tooltipText: media.sound.muted ? I18n.tr("Unmute", "tooltip on the button turning the sound back on in the large preview") : I18n.tr("Mute", "tooltip on the button silencing a video or audio file in the large preview")
            onClicked: media.sound.muted = !media.sound.muted
        }

        DSlider {
            id: volumeSlider

            Layout.preferredWidth: Theme.spacingXL * 4
            minimum: 0
            maximum: 100
            showValue: false
            wheelEnabled: false
            Component.onCompleted: value = Math.round(media.sound.volume * 100)
            onSliderValueChanged: newValue => {
                media.sound.volume = newValue / 100;
                media.sound.muted = false;
            }
        }
    }
}
