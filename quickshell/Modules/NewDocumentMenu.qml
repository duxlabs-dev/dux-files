pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.FileBrowser

FileMenu {
    id: menu

    required property string folder
    required property string opScope

    property var templates: []

    function present(pointX, pointY) {
        templates = [];
        loadTemplates();
        openAt(pointX, pointY);
    }

    function loadTemplates() {
        const dir = DirectoryService.templatesDir;
        if (dir === "")
            return;
        DirectoryService.list(dir, {
            "limit": 100
        }, result => {
            if (result.error)
                return;
            templates = (result.entries || []).filter(entry => !entry.isDir);
        });
    }

    function createDocument(template) {
        const name = template ? template.name : I18n.tr("Untitled document", "default name for a new empty file");
        FileOpsService.newFile(folder, name, template?.path ?? "", opScope, true);
    }

    FileMenuItem {
        text: I18n.tr("Empty file", "new document menu entry creating a blank file")
        iconName: "draft"
        onClicked: {
            menu.close();
            menu.createDocument(null);
        }
    }

    Repeater {
        model: menu.templates

        FileMenuItem {
            required property var modelData

            text: modelData.displayName !== "" ? modelData.displayName : modelData.name
            iconName: "description"
            onClicked: {
                menu.close();
                menu.createDocument(modelData);
            }
        }
    }
}
