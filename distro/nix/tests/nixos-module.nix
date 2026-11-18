{
  self,
  pkgs,
  ...
}:
pkgs.testers.runNixOSTest {
  name = "dux-files-nixos-module";

  nodes.machine = {
    imports = [
      self.nixosModules.dux-files
    ];

    users.users.duxlabs = {
      isNormalUser = true;
      extraGroups = [ "wheel" ];
    };

    programs.dux-files = {
      enable = true;
      systemd.enable = true;
    };

    system.stateVersion = "25.11";
  };

  testScript = ''
    machine.wait_for_unit("multi-user.target")

    machine.succeed("command -v dfiles")
    machine.succeed("su -- duxlabs -c 'dfiles --help >/dev/null'")
    machine.succeed("test -f /run/current-system/sw/share/systemd/user/dfiles.service")
    machine.succeed("test -f /run/current-system/sw/share/dbus-1/services/dev.duxlabs.files.FileManager1.service")
    machine.succeed("test -f /run/current-system/sw/share/polkit-1/actions/dev.duxlabs.files.policy")
  '';
}
