import QtQuick

Loader {
    id: loader

    default property alias dialog: loader.sourceComponent
    readonly property bool open: item?.visible ?? false

    asynchronous: true
}
