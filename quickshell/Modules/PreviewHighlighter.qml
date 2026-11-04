import QtQuick
import org.kde.syntaxhighlighting
import qs.Common

SyntaxHighlighter {
    property string fileName: ""

    definition: Repository.definitionForFileName(fileName)
    theme: Repository.defaultTheme(Theme.isLightMode ? Repository.LightTheme : Repository.DarkTheme)
}
