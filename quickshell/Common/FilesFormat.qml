pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import qs.DCommon.FileBrowser

Singleton {
    id: root

    function elevationUnavailable(reason) {
        switch (reason) {
        case "SANDBOXED":
            return I18n.tr("Administrator access is not available inside the sandbox", "file operation error dialog explanation");
        case "NOPKEXEC":
            return I18n.tr("Administrator access is not available because pkexec is not installed", "file operation error dialog explanation");
        case "NOPOLICY":
            return I18n.tr("Administrator access is not available because the polkit policy is not installed", "file operation error dialog explanation");
        default:
            return I18n.tr("Administrator access is not available", "file operation error dialog explanation");
        }
    }

    function featureUnavailable(reason) {
        switch (reason) {
        case "SANDBOXED":
            return I18n.tr("Not available inside the sandbox", "explanation for a feature the flatpak sandbox blocks");
        case "NOHOSTFS":
            return I18n.tr("Not available without access to the host filesystem", "explanation for a feature that needs the flatpak host filesystem permission");
        case "NOUDISKS":
            return I18n.tr("Drives are unavailable because the sandbox cannot reach udisks2", "explanation when the flatpak lacks the udisks2 permission");
        case "NOGVFS":
            return I18n.tr("Network locations are unavailable because the sandbox cannot reach gvfs", "explanation when the flatpak lacks the gvfs permissions");
        case "NOHOSTSPAWN":
            return I18n.tr("Not available because the sandbox cannot start programs on the host", "explanation for a feature that needs flatpak-spawn");
        case "PORTALCHOOSER":
            return I18n.tr("The system application chooser opens instead", "explanation when open with is handed to the desktop portal");
        case "NOWINDOW":
            return I18n.tr("Only the window process serves the portal", "explanation when a daemon without a window answers a portal question");
        case "NOBUS":
            return I18n.tr("This instance cannot reach the session bus.", "file manager registration setting description when the daemon has no session bus connection");
        default:
            return I18n.tr("Not available", "explanation for a feature the environment does not permit");
        }
    }

    function matching(count, text) {
        return I18n.tr("%1 matching “%2”", "filter bar above a folder listing, count then the typed text").arg(FileFormat.count(count)).arg(text);
    }

    function timeLeft(seconds) {
        const whole = Math.max(1, Math.ceil(seconds));
        if (whole < 60)
            return I18n.tr("%1 s left", "time left for a file operation, in seconds").arg(whole);
        const minutes = Math.ceil(whole / 60);
        if (minutes < 60)
            return I18n.tr("%1 min left", "time left for a file operation, in minutes").arg(minutes);
        return I18n.tr("%1 h %2 min left", "time left for a file operation, in hours and minutes").arg(Math.floor(minutes / 60)).arg(minutes % 60);
    }

    function transferHint(progress) {
        if (!progress || progress.estimating || progress.bytesTotal <= 0 || progress.bytesPerSec <= 0)
            return "";
        const rate = I18n.tr("%1/s", "transfer speed, %1 is a size such as 12 MB").arg(FileFormat.size(progress.bytesPerSec));
        const remaining = Math.max(0, progress.bytesTotal - progress.bytesDone) / progress.bytesPerSec;
        return I18n.tr("%1 · %2", "transfer speed then time left").arg(rate).arg(timeLeft(remaining));
    }

    function extractable(mime) {
        switch (mime) {
        case "application/zip":
        case "application/x-tar":
        case "application/gzip":
        case "application/x-gzip":
        case "application/x-compressed-tar":
        case "application/x-bzip":
        case "application/x-bzip2":
        case "application/bzip2":
        case "application/x-bzip-compressed-tar":
        case "application/x-bzip2-compressed-tar":
        case "application/x-xz":
        case "application/x-xz-compressed-tar":
        case "application/zstd":
        case "application/x-zstd-compressed-tar":
        case "application/x-lzma":
        case "application/x-lzma-compressed-tar":
        case "application/x-lzip":
        case "application/x-lzip-compressed-tar":
        case "application/x-7z-compressed":
        case "application/vnd.rar":
        case "application/x-rar":
        case "application/x-cpio":
            return true;
        }
        return false;
    }

    function isRunnable(mime, executable) {
        if (!executable)
            return false;
        switch (true) {
        case mime === "":
        case mime.startsWith("text/"):
        case mime.endsWith("script"):
        case mime === "application/x-executable":
        case mime === "application/x-sharedlib":
        case mime === "application/x-pie-executable":
            return true;
        }
        return false;
    }

    function permissionBits(mode) {
        const rwx = mode.length >= 9 ? mode.substring(mode.length - 9) : "---------";
        const triplets = [];
        for (let i = 0; i < 9; i += 3) {
            triplets.push({
                "read": rwx[i] === "r",
                "write": rwx[i + 1] === "w",
                "execute": rwx[i + 2] === "x" || rwx[i + 2] === "s" || rwx[i + 2] === "t"
            });
        }
        return triplets;
    }

    function octalFor(triplets) {
        return triplets.map(t => (t.read ? 4 : 0) + (t.write ? 2 : 0) + (t.execute ? 1 : 0)).join("");
    }
}
