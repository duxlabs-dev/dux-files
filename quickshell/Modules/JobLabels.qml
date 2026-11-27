pragma Singleton

import QtQuick
import Quickshell
import qs.Common

Singleton {
    readonly property var kinds: ({
            "copy": {
                "active": I18n.tr("Copying", "file operation in progress"),
                "done": I18n.tr("Copied", "toast after copying files")
            },
            "move": {
                "active": I18n.tr("Moving", "file operation in progress"),
                "done": I18n.tr("Moved", "toast after moving files")
            },
            "trash": {
                "active": I18n.tr("Moving to trash", "file operation in progress"),
                "done": I18n.tr("Moved to trash", "toast after trashing files")
            },
            "restore": {
                "active": I18n.tr("Restoring", "file operation in progress"),
                "done": I18n.tr("Restored", "toast after restoring items from the trash")
            },
            "delete": {
                "active": I18n.tr("Deleting", "file operation in progress"),
                "done": ""
            },
            "emptyTrash": {
                "active": I18n.tr("Emptying the trash", "file operation in progress"),
                "done": ""
            },
            "rename": {
                "active": I18n.tr("Renaming", "file operation in progress"),
                "done": I18n.tr("Renamed", "toast after renaming a file")
            },
            "compress": {
                "active": I18n.tr("Compressing", "file operation in progress"),
                "done": I18n.tr("Archive created", "toast after compressing files")
            },
            "extract": {
                "active": I18n.tr("Extracting", "file operation in progress"),
                "done": ""
            },
            "unlockArchive": {
                "active": I18n.tr("Unlocking", "file operation in progress"),
                "done": ""
            },
            "duplicate": {
                "active": I18n.tr("Duplicating", "file operation in progress"),
                "done": I18n.tr("Duplicated", "toast after copying files beside themselves")
            },
            "pasteImage": {
                "active": I18n.tr("Pasting image", "file operation in progress"),
                "done": I18n.tr("Image pasted", "toast after writing a clipboard image into the folder")
            },
            "link": {
                "active": I18n.tr("Linking", "file operation in progress"),
                "done": ""
            },
            "newFolder": {
                "active": I18n.tr("Creating", "file operation in progress"),
                "done": ""
            },
            "newFile": {
                "active": I18n.tr("Creating", "file operation in progress"),
                "done": ""
            },
            "newFolderWith": {
                "active": I18n.tr("Creating", "file operation in progress"),
                "done": ""
            },
            "setPermissions": {
                "active": I18n.tr("Changing permissions", "file operation in progress"),
                "done": ""
            },
            "setOwner": {
                "active": I18n.tr("Changing owner", "file operation in progress"),
                "done": ""
            }
        })

    function active(kind) {
        return kinds[kind]?.active ?? I18n.tr("Working", "file operation in progress");
    }

    function done(kind) {
        return kinds[kind]?.done ?? "";
    }
}
