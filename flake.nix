{
  description = "Dux Files";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    dank-qml-common = {
      url = "github:duxlabs-dev/dux-qml-common";
      flake = false;
    };
    dux-qt-base = {
      url = "github:duxlabs-dev/dux-qt-base";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      dank-qml-common,
      dux-qt-base,
      ...
    }:
    let
      goModVersion =
        let
          content = builtins.readFile ./core/go.mod;
          lines = builtins.filter builtins.isString (builtins.split "\n" content);
          goLines = builtins.filter (l: builtins.match "go [0-9]+\\..*" l != null) lines;
          matched =
            if goLines != [ ] then builtins.match "go ([0-9]+)\\.([0-9]+).*" (builtins.head goLines) else null;
        in
        if matched != null then
          {
            major = builtins.elemAt matched 0;
            minor = builtins.elemAt matched 1;
          }
        else
          {
            major = "1";
            minor = "26";
          };
      goForPkgs = pkgs: pkgs.${"go_${goModVersion.major}_${goModVersion.minor}"};

      forEachSystem =
        fn:
        nixpkgs.lib.genAttrs [ "aarch64-linux" "x86_64-linux" ] (
          system: fn system nixpkgs.legacyPackages.${system}
        );

      duxQtBaseFor = pkgs: (dux-qt-base.overlays.default pkgs pkgs).dux-qt-base;

      mkQmlImportPath =
        pkgs: qmlPkgs:
        pkgs.lib.concatStringsSep ":" (map (o: "${o}/${pkgs.qt6.qtbase.qtQmlPrefix}") qmlPkgs);

      mkQtPluginPath =
        pkgs: qtPkgs:
        pkgs.lib.concatStringsSep ":" (map (o: "${o}/${pkgs.qt6.qtbase.qtPluginPrefix}") qtPkgs);

      mkModuleWithDfilesPkgs =
        modulePath:
        args@{ pkgs, ... }:
        {
          imports = [
            (import modulePath (args // { dfilesPkgs = buildDfilesPkgs pkgs; }))
          ];
        };

      mkDfiles =
        pkgs:
        pkgs.lib.makeOverridable (
          {
            withPdfPreview ? true,
            duxQtBase ? duxQtBaseFor pkgs,
          }:
          let
            version = "2.1.0";
            thumbnailTools = with pkgs; [
              bubblewrap
              ffmpegthumbnailer
              poppler-utils
            ];
            qtPackages =
              (with pkgs.kdePackages; [
                qtdeclarative
                qtsvg
              ])
              ++ pkgs.lib.optional withPdfPreview pkgs.qt6.qtwebengine;
          in
          (pkgs.buildGoModule.override { go = goForPkgs pkgs; }) {
            inherit version;
            pname = "dux-files";
            src = ./.;
            modRoot = "core";
            vendorHash = "sha256-ol3OOGb+35SWVeVJ/ny0U8Vp/Uis9KEVhqIFbQt+dZI=";

            subPackages = [ "cmd/dfiles" ];

            tags = [
              "withshell"
              "withqtbase"
            ];

            # Mirror `make -C core sync-shell`. The flake src excludes submodule
            # content, so the DCommon symlink is replaced with the pinned input.
            postPatch = ''
              rm -rf core/internal/shellembed/dist
              cp -r quickshell core/internal/shellembed/dist
              rm -f core/internal/shellembed/dist/DCommon
              cp -r ${dank-qml-common}/DCommon core/internal/shellembed/dist/DCommon
              chmod -R u+w core/internal/shellembed/dist/DCommon
              rm -rf core/internal/shellembed/dist/scripts \
                core/internal/shellembed/dist/tests \
                core/internal/shellembed/dist/.claude
              rm -f core/internal/shellembed/dist/.qmlls.ini \
                core/internal/shellembed/dist/translations/extract_translations.py
            '';

            # embed the runtime library the same way make qtbase does; buildGoModule copies preBuild
            # into the vendor derivation, which would make the vendorHash build depend on dux-qt-base
            preConfigure = ''
              install -Dm644 ${duxQtBase}/lib/libdux-qt-base.so.0 core/internal/qtbaseembed/lib/libdux-qt-base.so.0
              sha256sum core/internal/qtbaseembed/lib/libdux-qt-base.so.0 | cut -c1-16 > core/internal/qtbaseembed/lib/.dankrev
            '';

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${version}"
              "-X main.Commit=${self.shortRev or self.dirtyShortRev or "dirty"}"
            ];

            nativeBuildInputs = with pkgs; [
              installShellFiles
              makeWrapper
            ];

            postInstall = ''
              install -Dm644 ${./assets/dev.duxlabs.files.desktop} \
                $out/share/applications/dev.duxlabs.files.desktop
              install -Dm644 ${./assets/dev.duxlabs.files.svg} \
                $out/share/icons/hicolor/scalable/apps/dev.duxlabs.files.svg
              install -Dm644 ${./assets/dev.duxlabs.files.metainfo.xml} \
                $out/share/metainfo/dev.duxlabs.files.metainfo.xml

              install -Dm644 ${./assets/dev.duxlabs.files.policy} \
                $out/share/polkit-1/actions/dev.duxlabs.files.policy
              substituteInPlace $out/share/polkit-1/actions/dev.duxlabs.files.policy \
                --replace-fail /usr/bin/dfiles $out/bin/.dfiles-wrapped

              install -Dm644 ${./assets/dbus/dev.duxlabs.files.FileManager1.service} \
                $out/share/dbus-1/services/dev.duxlabs.files.FileManager1.service
              substituteInPlace $out/share/dbus-1/services/dev.duxlabs.files.FileManager1.service \
                --replace-fail /usr/bin/dfiles $out/bin/dfiles

              install -Dm644 ${./assets/portal/dfiles.portal} \
                $out/share/xdg-desktop-portal/portals/dfiles.portal
              install -Dm644 ${./assets/dbus/org.freedesktop.impl.portal.desktop.dfiles.service} \
                $out/share/dbus-1/services/org.freedesktop.impl.portal.desktop.dfiles.service
              substituteInPlace $out/share/dbus-1/services/org.freedesktop.impl.portal.desktop.dfiles.service \
                --replace-fail /usr/bin/dfiles $out/bin/dfiles

              install -Dm644 ${./assets/systemd/dfiles.service} \
                $out/lib/systemd/user/dfiles.service
              substituteInPlace $out/lib/systemd/user/dfiles.service \
                --replace-fail /usr/bin/dfiles $out/bin/dfiles

              installShellCompletion --cmd dfiles \
                --bash <($out/bin/dfiles completion bash) \
                --fish <($out/bin/dfiles completion fish) \
                --zsh <($out/bin/dfiles completion zsh)

              wrapProgram $out/bin/dfiles \
                --prefix PATH : ${pkgs.lib.makeBinPath thumbnailTools} \
                --prefix NIXPKGS_QT6_QML_IMPORT_PATH : "${mkQmlImportPath pkgs qtPackages}" \
                --prefix QT_PLUGIN_PATH : "${mkQtPluginPath pkgs qtPackages}"
            '';

            meta = {
              description = "File manager for the dux desktop";
              homepage = "https://github.com/duxlabs-dev/dux-files";
              license = pkgs.lib.licenses.mit;
              mainProgram = "dfiles";
              platforms = pkgs.lib.platforms.linux;
            };
          }
        ) { };

      buildDfilesPkgs = pkgs: {
        dux-files = mkDfiles pkgs;
        dux-qt-base = duxQtBaseFor pkgs;
      };
    in
    {
      packages = forEachSystem (
        system: pkgs: {
          dux-files = mkDfiles pkgs;
          default = self.packages.${system}.dux-files;
        }
      );

      lib = { inherit mkDfiles buildDfilesPkgs; };

      homeModules.dux-files = mkModuleWithDfilesPkgs ./distro/nix/home.nix;

      homeModules.default = self.homeModules.dux-files;

      nixosModules.dux-files = mkModuleWithDfilesPkgs ./distro/nix/nixos.nix;

      nixosModules.default = self.nixosModules.dux-files;

      devShells = forEachSystem (
        system: pkgs: {
          default = pkgs.mkShell {
            GOTOOLCHAIN = "auto";

            buildInputs = with pkgs; [
              (goForPkgs pkgs)
              gopls
              delve
              go-tools
              gnumake

              quickshell

              prek
              uv
              python3

              nixd
              nil
            ];

            shellHook = ''
              touch quickshell/.qmlls.ini 2>/dev/null
              if [ ! -f .git/hooks/pre-commit ]; then prek install; fi
            '';
          };
        }
      );

      nixosTests = nixpkgs.lib.genAttrs [ "aarch64-linux" "x86_64-linux" ] (
        system:
        import ./distro/nix/tests {
          inherit self;
          pkgs = nixpkgs.legacyPackages.${system};
        }
      );
    };
}
