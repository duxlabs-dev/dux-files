{
  config,
  lib,
  dfilesPkgs,
  ...
}:
let
  inherit (lib) types;
in
{
  imports = [
    (lib.mkRemovedOptionModule [
      "programs"
      "dux-files"
      "quickshell"
      "package"
    ] "The Quickshell runtime is embedded in dfiles as dux-qt-base; set programs.dux-files.duxQtBase.package instead.")
  ];

  options.programs.dux-files = {
    enable = lib.mkEnableOption "Dux Files";

    package = lib.mkOption {
      type = types.package;
      default = dfilesPkgs.dux-files.override { duxQtBase = config.programs.dux-files.duxQtBase.package; };
      defaultText = lib.literalExpression "dfilesPkgs.dux-files.override { duxQtBase = config.programs.dux-files.duxQtBase.package; }";
      description = "The Dux Files package to use (defaults to be built from source against duxQtBase.package)";
    };

    duxQtBase = {
      package = lib.mkPackageOption dfilesPkgs "dux-qt-base" {
        extraDescription = "The Quickshell runtime library embedded in the dfiles package (pkgs.dux-qt-base when the overlay provides it, else the dux-qt-base flake input).";
      };
    };

    portal = {
      enable = lib.mkEnableOption "serving open and save dialogs through xdg-desktop-portal (registers the dfiles backend)";

      preferred = lib.mkOption {
        type = types.bool;
        default = false;
        description = "Prefer dfiles for org.freedesktop.impl.portal.FileChooser in the portal configuration named by portal.desktop.";
      };

      desktop = lib.mkOption {
        type = types.str;
        default = "common";
        description = "Which xdg.portal.config entry receives the FileChooser preference (a lowercased XDG_CURRENT_DESKTOP name, or common).";
      };
    };

    systemd = {
      enable = lib.mkEnableOption "Dux Files systemd startup";

      target = lib.mkOption {
        type = types.str;
        default = "graphical-session.target";
        description = "Systemd target that will automatically start dfiles.";
      };

      restartIfChanged = lib.mkOption {
        type = types.bool;
        default = true;
        description = "Auto-restart dfiles.service when dux-files changes";
      };
    };
  };
}
