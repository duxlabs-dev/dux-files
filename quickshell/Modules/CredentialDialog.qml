pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

OverlayDialog {
    id: root

    property var ask: null
    readonly property bool isQuestion: ask !== null && ask.type === "question"
    readonly property var saveOptions: [I18n.tr("Never", "credential dialog option that forgets the password"), I18n.tr("This session", "credential dialog option that keeps the password until logout"), I18n.tr("Always", "credential dialog option that stores the password in the keyring")]

    function present(request) {
        if (ask && ask.id !== request.id)
            NetworkService.abort(ask.id);
        ask = request;
        userField.text = request.defaultUser || "";
        domainField.text = request.defaultDomain || "";
        passwordField.text = "";
        anonymousToggle.checked = false;
        saveDropdown.currentValue = saveOptions[0];
        open();
    }

    function submit() {
        if (!ask)
            return;
        NetworkService.answer(ask.id, {
            "password": passwordField.text,
            "user": userField.text,
            "domain": domainField.text,
            "anonymous": anonymousToggle.checked,
            "save": Math.max(0, saveOptions.indexOf(saveDropdown.currentValue))
        });
        ask = null;
        close();
    }

    function choose(index) {
        NetworkService.answer(ask.id, {
            "choice": index
        });
        ask = null;
        close();
    }

    function abort() {
        if (ask)
            NetworkService.abort(ask.id);
        ask = null;
        close();
    }

    title: isQuestion ? I18n.tr("Confirm", "dialog header for a question the server asked") : I18n.tr("Sign in", "credential dialog header")
    supportingText: ask ? ask.message : ""
    onAccepted: {
        if (!isQuestion)
            submit();
    }
    onRejected: abort()
    cancelAction: () => root.abort()

    Column {
        width: parent.width
        spacing: Theme.spacingM
        visible: !root.isQuestion

        DTextField {
            id: userField

            width: parent.width
            visible: root.ask !== null && root.ask.needUsername === true
            focus: visible
            outlined: true
            enabled: !anonymousToggle.checked
            labelText: I18n.tr("Username", "label for a user name input")
            onAccepted: root.submit()
        }

        DTextField {
            id: domainField

            width: parent.width
            visible: root.ask !== null && root.ask.needDomain === true
            outlined: true
            enabled: !anonymousToggle.checked
            labelText: I18n.tr("Domain", "label for a network domain input")
            onAccepted: root.submit()
        }

        PasswordField {
            id: passwordField

            visible: root.ask !== null && root.ask.needPassword === true
            focus: visible && !userField.visible
            enabled: !anonymousToggle.checked
            onAccepted: root.submit()
        }

        DToggle {
            id: anonymousToggle

            width: parent.width
            visible: root.ask !== null && root.ask.canAnonymous === true
            text: I18n.tr("Connect anonymously", "credential dialog option that skips the login")
        }

        DDropdown {
            id: saveDropdown

            width: parent.width
            visible: root.ask !== null && root.ask.canSave === true && !anonymousToggle.checked
            text: I18n.tr("Remember password", "credential dialog option controlling password storage")
            options: root.saveOptions
            currentValue: root.saveOptions[0]
        }
    }

    Row {
        width: parent.width
        spacing: Theme.spacingS
        visible: root.isQuestion

        Repeater {
            model: root.ask !== null ? root.ask.choices ?? [] : []

            DButton {
                required property int index
                required property var modelData

                text: modelData
                variant: index === 0 ? "filled" : "tonal"
                onClicked: root.choose(index)
            }
        }
    }

    actions: [
        DButton {
            visible: !root.isQuestion
            text: I18n.tr("Sign in", "credential dialog header")
            onClicked: root.submit()
        }
    ]
}
