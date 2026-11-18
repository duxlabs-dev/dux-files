import QtQuick
import qs.Common
import qs.DCommon.Widgets

DTextField {
    width: parent.width
    outlined: true
    echoMode: TextInput.Password
    showPasswordToggle: true
    labelText: I18n.tr("Password", "label for a password input")
}
