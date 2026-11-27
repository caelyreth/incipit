{
  description = "Incipit - a writing workspace built with Go and MyGo";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      linuxSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      devSystems = linuxSystems ++ [
        "aarch64-darwin"
      ];
      forLinuxSystems = nixpkgs.lib.genAttrs linuxSystems;
      forDevSystems = nixpkgs.lib.genAttrs devSystems;
      mkPackage = pkgs: import ./nix/package.nix { inherit pkgs; };
    in
    {
      packages = forLinuxSystems (
        system:
        let
          incipit = mkPackage nixpkgs.legacyPackages.${system};
        in
        {
          inherit incipit;
          default = incipit;
        }
      );

      apps = forLinuxSystems (
        system:
        let
          app = {
            type = "app";
            program = nixpkgs.lib.getExe self.packages.${system}.default;
            meta.description = self.packages.${system}.default.meta.description;
          };
        in
        {
          incipit = app;
          default = app;
        }
      );

      checks = forLinuxSystems (system: {
        package = self.packages.${system}.default;
      });

      devShells = forDevSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go_1_27
              gotools
              golangci-lint
              gopls
              nixpkgs-fmt
              jq
            ];

            GOTOOLCHAIN = "go1.27.3+auto";
            CGO_ENABLED = "0";
          };
        }
      );

      formatter = forDevSystems (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);
      overlays.default = final: _prev: { incipit = mkPackage final; };
    };
}
