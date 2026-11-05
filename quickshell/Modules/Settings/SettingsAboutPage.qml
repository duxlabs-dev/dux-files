pragma ComponentBehavior: Bound

import QtQuick
import qs.Common
import qs.Services
import qs.DCommon.Widgets

Item {
    id: root

    LayoutMirroring.enabled: I18n.isRtl
    LayoutMirroring.childrenInherit: true

    readonly property string githubUrl: "https://github.com/duxlabs-dev/dux-files"
    readonly property string docsUrl: "https://duxlabs.dev/docs"
    readonly property string kofiUrl: "https://ko-fi.com/duxlabs"
    readonly property string licenseUrl: githubUrl + "/blob/master/LICENSE"
    readonly property string version: DuxFilesService.daemonVersion.match(/^v?(\d+\.\d+)/)?.[1] ?? ""

    readonly property var featureRows: [
        {
            "id": "hostFilesystem",
            "label": I18n.tr("Host filesystem", "capability row on the about page")
        },
        {
            "id": "trash",
            "label": I18n.tr("Trash", "capability row on the about page")
        },
        {
            "id": "thumbnails",
            "label": I18n.tr("Thumbnails", "capability row on the about page")
        },
        {
            "id": "devices",
            "label": I18n.tr("Devices", "sidebar section listing drives and volumes")
        },
        {
            "id": "network",
            "label": I18n.tr("Network", "sidebar section listing network locations")
        },
        {
            "id": "terminal",
            "label": I18n.tr("Open in terminal", "context menu entry opening a terminal")
        },
        {
            "id": "run",
            "label": I18n.tr("Run programs", "capability row on the about page")
        },
        {
            "id": "openWith",
            "label": I18n.tr("Open with", "open with dialog header")
        },
        {
            "id": "setDefault",
            "label": I18n.tr("Default applications", "capability row on the about page")
        },
        {
            "id": "elevation",
            "label": I18n.tr("Retry as administrator", "file operation error dialog action")
        },
        {
            "id": "indexSearch",
            "label": I18n.tr("Search with the danksearch index", "capability row on the about page")
        }
    ]

    function host(url) {
        return url.replace(/^https?:\/\//, "").replace(/\/$/, "");
    }

    function statusFor(id) {
        const feature = SandboxService.features[id];
        if (!feature)
            return {
                "text": I18n.tr("Unknown", "capability status when the daemon did not report it"),
                "color": Theme.surfaceVariantText
            };
        if (!feature.available)
            return {
                "text": FilesFormat.featureUnavailable(feature.reason),
                "color": Theme.error
            };
        if (feature.degraded)
            return {
                "text": FilesFormat.featureUnavailable(feature.reason),
                "color": Theme.tertiary
            };
        return {
            "text": I18n.tr("Available", "capability status on the about page"),
            "color": Theme.success
        };
    }

    SettingsPage {
        DHeroCard {
            brand: I18n.tr("Files", "main window title")
            caption: root.version ? `dfiles ${root.version}` : "dfiles"

            DButton {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr("Support Dux Labs", "about page hero button, opens the Ko-fi donation page")
                iconName: "favorite"
                onClicked: DuxFilesService.openUri(root.kofiUrl)
            }
        }

        SettingsGroup {
            SettingsLinkRow {
                iconName: "menu_book"
                title: I18n.tr("Docs", "about page resource button")
                keywords: I18n.tr("documentation help manual", "settings search keywords, space separated, add synonyms in your language")
                subtitle: root.host(root.docsUrl)
                url: root.docsUrl
            }

            SettingsLinkRow {
                iconName: "code"
                title: I18n.tr("GitHub", "about page resource button")
                keywords: I18n.tr("source code repository issues", "settings search keywords, space separated, add synonyms in your language")
                subtitle: root.host(root.githubUrl)
                url: root.githubUrl
            }
        }

        SettingsCard {
            title: I18n.tr("Backend", "about page daemon section header")

            SettingsRow {
                title: I18n.tr("Version", "about page daemon version column label")
                keywords: I18n.tr("daemon release build", "settings search keywords, space separated, add synonyms in your language")
                trailingBadge: DuxFilesService.daemonVersion || "—"
            }

            SettingsRow {
                title: I18n.tr("API", "about page daemon api version column label")
                keywords: I18n.tr("daemon protocol", "settings search keywords, space separated, add synonyms in your language")
                trailingBadge: DuxFilesService.apiVersion > 0 ? `v${DuxFilesService.apiVersion}` : "—"
            }

            SettingsRow {
                title: I18n.tr("Status", "about page daemon status column label")
                keywords: I18n.tr("daemon connected offline socket", "settings search keywords, space separated, add synonyms in your language")
                trailingBadge: DuxFilesService.connected ? I18n.tr("Connected", "about page daemon status value") : I18n.tr("Offline", "about page daemon status value")

                DBadge {
                    color: DuxFilesService.connected ? Theme.success : Theme.error
                }
            }

            SettingsRow {
                title: I18n.tr("Socket", "about page daemon socket row label")
                keywords: I18n.tr("daemon path unix", "settings search keywords, space separated, add synonyms in your language")
                subtitle: DuxFilesService.socketPath || "—"
            }

            SettingsRow {
                visible: SandboxService.sandboxed
                title: I18n.tr("Sandbox", "about page row naming the flatpak id")
                keywords: I18n.tr("flatpak app id", "settings search keywords, space separated, add synonyms in your language")
                subtitle: SandboxService.appId
            }
        }

        SettingsCard {
            title: I18n.tr("Capabilities", "about page daemon capabilities label")

            Repeater {
                model: root.featureRows

                SettingsRow {
                    required property var modelData

                    readonly property var status: root.statusFor(modelData.id)

                    title: modelData.label
                    keywords: I18n.tr("capability available sandbox", "settings search keywords, space separated, add synonyms in your language")
                    trailingBadge: status.text
                    trailingBadgeColor: status.color
                }
            }
        }

        StyledText {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("MIT License", "about page license footer link")
            font.pixelSize: Theme.fontSizeMedium
            color: Theme.primary

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: DuxFilesService.openUri(root.licenseUrl)
            }
        }
    }
}
